package rime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gaboolic/moqi-ime/imecore"
)

const productSettingsFileName = "settings.json"

type aiRunMode string

const (
	aiRunModeSmart    aiRunMode = "smart"
	aiRunModeResident aiRunMode = "resident"
	aiRunModeManual   aiRunMode = "manual"
)

type productSettings struct {
	AIEnabled         bool      `json:"ai_enabled"`
	AIRunMode         aiRunMode `json:"ai_run_mode"`
	AIIdleExitMinutes int       `json:"ai_idle_exit_minutes"`
}

func defaultProductSettings() productSettings {
	return productSettings{
		AIEnabled:         true,
		AIRunMode:         aiRunModeSmart,
		AIIdleExitMinutes: 30,
	}
}

func productSettingsPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, productSettingsFileName)
}

func loadProductSettings() productSettings {
	settings := defaultProductSettings()
	path := productSettingsPath()
	if path == "" {
		return settings
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultProductSettings()
	}
	return normalizeProductSettings(settings)
}

func normalizeProductSettings(settings productSettings) productSettings {
	switch aiRunMode(strings.ToLower(strings.TrimSpace(string(settings.AIRunMode)))) {
	case aiRunModeSmart:
		settings.AIRunMode = aiRunModeSmart
	case aiRunModeResident:
		settings.AIRunMode = aiRunModeResident
	case aiRunModeManual:
		settings.AIRunMode = aiRunModeManual
	default:
		settings.AIRunMode = aiRunModeSmart
	}
	if settings.AIIdleExitMinutes != 10 && settings.AIIdleExitMinutes != 30 && settings.AIIdleExitMinutes != 0 {
		settings.AIIdleExitMinutes = 30
	}
	return settings
}

func saveProductSettings(settings productSettings) bool {
	path := productSettingsPath()
	if path == "" {
		return false
	}
	settings = normalizeProductSettings(settings)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false
	}
	return os.WriteFile(path, data, 0o644) == nil
}

func (ime *IME) applyLocalAISettings(resp *imecore.Response) bool {
	if !saveProductSettings(ime.productSettings) {
		if resp != nil {
			resp.TrayNotification = trayNotification("本地 AI 设置保存失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	if err := ime.reloadAIConfig(); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("本地 AI 配置无效: "+err.Error(), imecore.TrayNotificationIconError)
		}
		return false
	}
	if resp != nil {
		resp.RemovePreservedKey = append(resp.RemovePreservedKey,
			ghostPreservedKeyGUID, ghostLongPreservedKeyGUID, ghostNextPreservedKeyGUID)
		if ime.ghostCompletionEnabled() {
			resp.AddPreservedKey = append(resp.AddPreservedKey, ghostPreservedKeyInfos()...)
		}
	}
	if !ime.productSettings.AIEnabled {
		sharedLocalAIRuntime.stop()
	}
	return true
}

func (ime *IME) openAIConfig(resp *imecore.Response) bool {
	if err := ensureUserAIConfigCopied(); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("无法创建本地 AI 配置: "+err.Error(), imecore.TrayNotificationIconError)
		}
		return false
	}
	path := userAIConfigPath()
	if path == "" {
		return false
	}
	if err := openWithDefaultApp(path); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("无法打开本地 AI 配置: "+err.Error(), imecore.TrayNotificationIconError)
		}
		return false
	}
	return true
}
