package rime

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	localAIModelFileName  = "Qwen3.5-4B-Q4_K_M.gguf"
	localAIStartupTimeout = 35 * time.Second
)

// localAIRuntime owns only llama-server processes started by Moqi. An
// already-running user process is used but never stopped by us.
type localAIRuntime struct {
	mu        sync.Mutex
	processID int
	idleTimer *time.Timer
}

var sharedLocalAIRuntime localAIRuntime

func (runtime *localAIRuntime) ensure(client *aiClient, mode aiRunMode, idleMinutes int) error {
	if client == nil || !isLoopbackAIEndpoint(client.baseURL) {
		return nil
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	if localAIHealthy(client.baseURL) {
		runtime.armIdleTimerLocked(mode, idleMinutes)
		return nil
	}

	pid, err := startLocalAIServer(client.baseURL)
	if err != nil {
		return err
	}
	runtime.processID = pid
	runtime.armIdleTimerLocked(mode, idleMinutes)
	return nil
}

func (runtime *localAIRuntime) touch(mode aiRunMode, idleMinutes int) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.armIdleTimerLocked(mode, idleMinutes)
}

func (runtime *localAIRuntime) armIdleTimerLocked(mode aiRunMode, idleMinutes int) {
	if runtime.idleTimer != nil {
		runtime.idleTimer.Stop()
		runtime.idleTimer = nil
	}
	if runtime.processID <= 0 || mode == aiRunModeResident || idleMinutes == 0 {
		return
	}
	delay := time.Duration(idleMinutes) * time.Minute
	runtime.idleTimer = time.AfterFunc(delay, runtime.stop)
}

func (runtime *localAIRuntime) stop() {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.idleTimer != nil {
		runtime.idleTimer.Stop()
		runtime.idleTimer = nil
	}
	if runtime.processID > 0 {
		if process, err := os.FindProcess(runtime.processID); err == nil {
			_ = process.Kill()
		}
		runtime.processID = 0
	}
}

func managedCompletionGenerator(client *aiClient, mode aiRunMode, idleMinutes int) func(aiCompletionRequest, aiCompletionConfig) ([]string, error) {
	if client == nil {
		return nil
	}
	return func(input aiCompletionRequest, cfg aiCompletionConfig) ([]string, error) {
		if err := sharedLocalAIRuntime.ensure(client, mode, idleMinutes); err != nil {
			return nil, err
		}
		candidates, err := client.GenerateInlineCompletions(input, cfg)
		sharedLocalAIRuntime.touch(mode, idleMinutes)
		return candidates, err
	}
}

func localAIHealthy(baseURL string) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	client := http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func startLocalAIServer(baseURL string) (int, error) {
	modelPath := localAIModelPath()
	if info, err := os.Stat(modelPath); err != nil || info.IsDir() {
		return 0, fmt.Errorf("Model file was not found: %s", modelPath)
	}
	serverPath := localAIServerPath()
	if serverPath == "" {
		return 0, fmt.Errorf("llama-server.exe was not found")
	}

	logDir := filepath.Join(filepath.Dir(filepath.Dir(modelPath)), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, fmt.Errorf("create local AI log directory: %w", err)
	}
	stdout, err := os.OpenFile(filepath.Join(logDir, "llama-server.stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open local AI output log: %w", err)
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(logDir, "llama-server.stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open local AI error log: %w", err)
	}
	defer stderr.Close()

	cmd := exec.Command(serverPath,
		"--model", modelPath,
		"--alias", "qwen3.5-4b-ime",
		"--host", "127.0.0.1",
		"--port", "8080",
		"--ctx-size", "4096",
		"--predict", "32",
		"--n-gpu-layers", "all",
		"--flash-attn", "on",
		"--parallel", "1",
		"--reasoning", "off",
		"--api-key", "ime-local",
		"--cors-origins", "localhost",
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	configureLocalAICommand(cmd)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start llama-server.exe: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.NewTimer(localAIStartupTimeout)
	ticker := time.NewTicker(400 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		select {
		case err := <-exited:
			if err == nil {
				err = io.EOF
			}
			return 0, fmt.Errorf("llama-server.exe exited during startup: %w", err)
		case <-ticker.C:
			if localAIHealthy(baseURL) {
				return cmd.Process.Pid, nil
			}
		case <-deadline.C:
			_ = cmd.Process.Kill()
			return 0, fmt.Errorf("local AI startup timed out after %s", localAIStartupTimeout)
		}
	}
}

func localAIModelPath() string {
	if configured := strings.TrimSpace(os.Getenv("MOQI_LOCAL_AI_MODEL")); configured != "" {
		return configured
	}

	// A packaged text host can start the launcher with a reduced environment.
	// Prefer Windows' user cache directory, then retain environment and legacy
	// package locations as fallbacks. Return the canonical path when none exist
	// so the startup error remains actionable.
	candidates := make([]string, 0, 4)
	if cacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheDir) != "" {
		candidates = append(candidates, filepath.Join(cacheDir, "MoqiAI", "models", localAIModelFileName))
	}
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "MoqiAI", "models", localAIModelFileName))
	}
	if profile := strings.TrimSpace(os.Getenv("USERPROFILE")); profile != "" {
		candidates = append(candidates, filepath.Join(profile, "AppData", "Local", "MoqiAI", "models", localAIModelFileName))
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "local-ai", "models", localAIModelFileName))
	}
	for _, candidate := range uniquePaths(candidates) {
		if isRegularFile(candidate) {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join("MoqiAI", "models", localAIModelFileName)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			continue
		}
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func localAIServerPath() string {
	if configured := strings.TrimSpace(os.Getenv("MOQI_LLAMA_SERVER")); isRegularFile(configured) {
		return configured
	}
	if found, err := exec.LookPath("llama-server.exe"); err == nil && isRegularFile(found) {
		return found
	}

	localRoots := make([]string, 0, 3)
	if cacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheDir) != "" {
		localRoots = append(localRoots, cacheDir)
	}
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		localRoots = append(localRoots, localAppData)
	}
	if profile := strings.TrimSpace(os.Getenv("USERPROFILE")); profile != "" {
		localRoots = append(localRoots, filepath.Join(profile, "AppData", "Local"))
	}
	for _, localRoot := range uniquePaths(localRoots) {
		root := filepath.Join(localRoot, "Microsoft", "WinGet", "Packages")
		matches, _ := filepath.Glob(filepath.Join(root, "ggml.llamacpp_*", "llama-server.exe"))
		for _, match := range matches {
			if isRegularFile(match) {
				return match
			}
		}
	}
	return ""
}

func isRegularFile(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
