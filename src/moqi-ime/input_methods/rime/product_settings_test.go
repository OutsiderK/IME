package rime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductSettingsRoundTripAndNormalize(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	want := productSettings{
		AIEnabled:         false,
		AIRunMode:         aiRunModeResident,
		AIIdleExitMinutes: 10,
	}
	if !saveProductSettings(want) {
		t.Fatal("saveProductSettings failed")
	}
	got := loadProductSettings()
	if got != want {
		t.Fatalf("settings round trip got %#v want %#v", got, want)
	}

	invalid := []byte(`{"ai_enabled":true,"ai_run_mode":"unknown","ai_idle_exit_minutes":7}`)
	if err := os.WriteFile(productSettingsPath(), invalid, 0o644); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}
	got = loadProductSettings()
	if !got.AIEnabled || got.AIRunMode != aiRunModeSmart || got.AIIdleExitMinutes != 30 {
		t.Fatalf("invalid settings were not normalized: %#v", got)
	}
}

func TestLocalAIPathsHonorExplicitConfiguration(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model.gguf")
	server := filepath.Join(root, "llama-server.exe")
	if err := os.WriteFile(model, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server, []byte("server"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOQI_LOCAL_AI_MODEL", model)
	t.Setenv("MOQI_LLAMA_SERVER", server)
	if got := localAIModelPath(); got != model {
		t.Fatalf("localAIModelPath got %q want %q", got, model)
	}
	if got := localAIServerPath(); got != server {
		t.Fatalf("localAIServerPath got %q want %q", got, server)
	}
}

func TestLocalAIModelPathFallsBackToUserProfile(t *testing.T) {
	profile := t.TempDir()
	model := filepath.Join(profile, "AppData", "Local", "MoqiAI", "models", localAIModelFileName)
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MOQI_LOCAL_AI_MODEL", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "missing-local-app-data"))
	t.Setenv("USERPROFILE", profile)
	if got := localAIModelPath(); got != model {
		t.Fatalf("localAIModelPath got %q want user-profile fallback %q", got, model)
	}
}

func TestLocalAIServerPathFallsBackToUserProfile(t *testing.T) {
	profile := t.TempDir()
	server := filepath.Join(profile, "AppData", "Local", "Microsoft", "WinGet", "Packages", "ggml.llamacpp_test", "llama-server.exe")
	if err := os.MkdirAll(filepath.Dir(server), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server, []byte("server"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MOQI_LLAMA_SERVER", "")
	t.Setenv("PATH", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "missing-local-app-data"))
	t.Setenv("USERPROFILE", profile)
	if got := localAIServerPath(); got != server {
		t.Fatalf("localAIServerPath got %q want user-profile fallback %q", got, server)
	}
}
