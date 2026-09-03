package rime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gaboolic/moqi-ime/imecore"
)

const appearanceConfigFileName = "appearance_config.json"
const rimeDefaultCustomConfigFileName = "default.custom.yaml"
const fixedCandidateCount = 7

// appearanceConfig keeps historical JSON keys readable during migration. The
// visual fields are intentionally ignored: the personal edition has one fixed
// product style instead of a theme/font/color settings system.
type appearanceConfig struct {
	CandidateTheme                 *string `json:"candidate_theme,omitempty"`
	FontFace                       *string `json:"font_face,omitempty"`
	FontPoint                      *int    `json:"font_point,omitempty"`
	CandidateCommentFontFace       *string `json:"candidate_comment_font_face,omitempty"`
	CandidateCommentFontPoint      *int    `json:"candidate_comment_font_point,omitempty"`
	InlinePreedit                  *bool   `json:"inline_preedit,omitempty"`
	CandidatePerRow                *int    `json:"candidate_per_row,omitempty"`
	CandidateCount                 *int    `json:"candidate_count,omitempty"`
	CandidateSpacing               *int    `json:"candidate_spacing,omitempty"`
	CandidateBackgroundColor       *string `json:"candidate_background_color,omitempty"`
	CandidateHighlightColor        *string `json:"candidate_highlight_color,omitempty"`
	CandidateTextColor             *string `json:"candidate_text_color,omitempty"`
	CandidateHighlightTextColor    *string `json:"candidate_highlight_text_color,omitempty"`
	CandidateCommentColor          *string `json:"candidate_comment_color,omitempty"`
	CandidateCommentHighlightColor *string `json:"candidate_comment_highlight_color,omitempty"`

	InputStateShared         *bool             `json:"input_state_shared,omitempty"`
	SharedOptions            map[string]bool   `json:"shared_options,omitempty"`
	SyncedOptions            map[string]bool   `json:"synced_options,omitempty"`
	CurrentSchemaID          *string           `json:"current_schema_id,omitempty"`
	CurrentSchemaBySchemeSet map[string]string `json:"current_schema_by_scheme_set,omitempty"`
	SharedAsciiMode          *bool             `json:"shared_ascii_mode,omitempty"`
	SharedFullShape          *bool             `json:"shared_full_shape,omitempty"`
	SharedTraditionalization *bool             `json:"shared_traditionalization,omitempty"`
	AutoPairQuotes           *bool             `json:"auto_pair_quotes,omitempty"`
	SemicolonSelectSecond    *bool             `json:"semicolon_select_second,omitempty"`
}

var appearanceState struct {
	mu      sync.RWMutex
	version uint64
	cfg     appearanceConfig
	loaded  bool
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneAppearanceConfig(cfg appearanceConfig) appearanceConfig {
	return appearanceConfig{
		InputStateShared:         cloneBoolPtr(cfg.InputStateShared),
		SharedOptions:            cloneBoolMap(cfg.SharedOptions),
		SyncedOptions:            cloneBoolMap(cfg.SyncedOptions),
		CurrentSchemaID:          cloneStringPtr(cfg.CurrentSchemaID),
		CurrentSchemaBySchemeSet: cloneStringMap(cfg.CurrentSchemaBySchemeSet),
		SharedAsciiMode:          cloneBoolPtr(cfg.SharedAsciiMode),
		SharedFullShape:          cloneBoolPtr(cfg.SharedFullShape),
		SharedTraditionalization: cloneBoolPtr(cfg.SharedTraditionalization),
		AutoPairQuotes:           cloneBoolPtr(cfg.AutoPairQuotes),
		SemicolonSelectSecond:    cloneBoolPtr(cfg.SemicolonSelectSecond),
	}
}

func sharedAppearanceConfig() (appearanceConfig, uint64, bool) {
	appearanceState.mu.RLock()
	defer appearanceState.mu.RUnlock()
	if !appearanceState.loaded {
		return appearanceConfig{}, 0, false
	}
	return cloneAppearanceConfig(appearanceState.cfg), appearanceState.version, true
}

func setSharedAppearanceConfig(cfg appearanceConfig) uint64 {
	appearanceState.mu.Lock()
	defer appearanceState.mu.Unlock()
	appearanceState.version++
	appearanceState.cfg = cloneAppearanceConfig(cfg)
	appearanceState.loaded = true
	return appearanceState.version
}

func resetSharedAppearanceConfigForTest() {
	appearanceState.mu.Lock()
	defer appearanceState.mu.Unlock()
	appearanceState.version = 0
	appearanceState.cfg = appearanceConfig{}
	appearanceState.loaded = false
}

func userAppearanceConfigPath() string {
	if root := moqiAppDataDir(); root != "" {
		return filepath.Join(root, appearanceConfigFileName)
	}
	return ""
}

func legacyUserAppearanceConfigPath() string {
	if root := moqiAppDataDir(); root != "" {
		return filepath.Join(root, defaultSchemeSetName, appearanceConfigFileName)
	}
	return ""
}

func (ime *IME) applyAppearanceConfig(cfg appearanceConfig) {
	ime.style = defaultStyle()
	if cfg.InputStateShared != nil {
		ime.inputStateShared = *cfg.InputStateShared
	}
	if len(cfg.SharedOptions) > 0 {
		ime.sharedOptions = cloneBoolMap(cfg.SharedOptions)
	}
	if len(cfg.SyncedOptions) > 0 {
		ime.syncedOptions = cloneBoolMap(cfg.SyncedOptions)
	}
	if ime.sharedOptions == nil {
		ime.sharedOptions = make(map[string]bool)
	}
	if ime.syncedOptions == nil {
		ime.syncedOptions = make(map[string]bool)
	}
	if cfg.CurrentSchemaID != nil {
		ime.syncedSchemaID = strings.TrimSpace(*cfg.CurrentSchemaID)
		ime.setSyncedSchemaIDForCurrentSchemeSet(ime.syncedSchemaID)
	}
	if len(cfg.CurrentSchemaBySchemeSet) > 0 {
		ime.syncedSchemaBySchemeSet = cloneStringMap(cfg.CurrentSchemaBySchemeSet)
		if schemaID := strings.TrimSpace(ime.syncedSchemaBySchemeSet[currentSchemeSetName()]); schemaID != "" {
			ime.syncedSchemaID = schemaID
		}
	}
	if cfg.SharedAsciiMode != nil {
		ime.sharedOptions["ascii_mode"] = *cfg.SharedAsciiMode
	}
	if cfg.SharedFullShape != nil {
		ime.sharedOptions["full_shape"] = *cfg.SharedFullShape
	}
	if cfg.SharedTraditionalization != nil {
		ime.sharedOptions["traditionalization"] = *cfg.SharedTraditionalization
	}
	if cfg.AutoPairQuotes != nil {
		ime.autoPairQuotes = *cfg.AutoPairQuotes
	}
	if cfg.SemicolonSelectSecond != nil {
		ime.semicolonSelectSecond = *cfg.SemicolonSelectSecond
	}
}

func (ime *IME) loadAppearancePrefs() {
	if cfg, version, ok := sharedAppearanceConfig(); ok {
		ime.applyAppearanceConfig(cfg)
		ime.appearanceVersion = version
		return
	}
	primaryPath := userAppearanceConfigPath()
	if primaryPath == "" {
		return
	}
	var cfg appearanceConfig
	loadedPath := ""
	for _, path := range []string{primaryPath, legacyUserAppearanceConfigPath()} {
		data, err := os.ReadFile(path)
		if err == nil {
			if json.Unmarshal(data, &cfg) == nil {
				loadedPath = path
			}
			break
		}
		if !os.IsNotExist(err) {
			return
		}
	}
	if loadedPath == "" {
		ime.saveAppearancePrefsWithReason("create_preferences")
		return
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
	if loadedPath != primaryPath {
		ime.saveAppearancePrefsWithReason("migrate_preferences")
	}
}

func (ime *IME) saveAppearancePrefs() {
	ime.saveAppearancePrefsWithReason("unspecified")
}

func (ime *IME) saveAppearancePrefsWithReason(reason string) {
	path := userAppearanceConfigPath()
	if path == "" || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	inputStateShared := ime.inputStateShared
	autoPairQuotes := ime.autoPairQuotes
	semicolonSelectSecond := ime.semicolonSelectSecond
	cfg := appearanceConfig{
		InputStateShared:      &inputStateShared,
		SyncedOptions:         cloneBoolMap(ime.syncedOptions),
		AutoPairQuotes:        &autoPairQuotes,
		SemicolonSelectSecond: &semicolonSelectSecond,
	}
	if ime.inputStateShared {
		cfg.SharedOptions = cloneBoolMap(ime.sharedOptions)
	}
	if current := strings.TrimSpace(ime.syncedSchemaIDForCurrentSchemeSet()); current != "" {
		cfg.CurrentSchemaID = &current
	}
	if len(ime.syncedSchemaBySchemeSet) > 0 {
		cfg.CurrentSchemaBySchemeSet = cloneStringMap(ime.syncedSchemaBySchemeSet)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil || os.WriteFile(path, data, 0o644) != nil {
		return
	}
	debugLogf("savePreferences triggered_by=%s path=%q", strings.TrimSpace(reason), path)
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
}

func (ime *IME) syncAppearancePrefs() bool {
	cfg, version, ok := sharedAppearanceConfig()
	if !ok || version == ime.appearanceVersion {
		return false
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = version
	return true
}

func (ime *IME) sendAsyncAppearanceUpdate(notification *imecore.TrayNotification) {
	if ime.asyncResponseSender == nil {
		return
	}
	resp := imecore.NewResponse(0, true)
	resp.CustomizeUI = ime.customizeUIMap()
	ime.fillResponseFromCurrentState(resp)
	resp.TrayNotification = notification
	ime.asyncResponseSender(resp)
}

func normalizeColor(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	if len(value) != 7 {
		return ""
	}
	for _, ch := range value[1:] {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return value
}

func (ime *IME) inlinePreeditEnabled() bool        { return true }
func (ime *IME) isHorizontalCandidateLayout() bool { return true }
func (ime *IME) horizontalCandidatePerRow() int    { return fixedCandidateCount }
func (ime *IME) effectiveCandidatePerRow() int     { return fixedCandidateCount }
func (ime *IME) candidateCount() int               { return fixedCandidateCount }
func isCandidateCountCommand(int) bool             { return false }
func (ime *IME) applyAppearanceCommand(int) bool   { return false }

func (ime *IME) customizeUIMap() map[string]interface{} {
	style := defaultStyle()
	return map[string]interface{}{
		"candFontName":              style.FontFace,
		"candFontSize":              style.FontPoint,
		"candCommentFontName":       style.CandidateCommentFontFace,
		"candCommentFontSize":       style.CandidateCommentFontPoint,
		"candPerRow":                style.CandidatePerRow,
		"candSpacing":               style.CandidateSpacing,
		"candUseCursor":             style.CandidateUseCursor,
		"candBackgroundColor":       normalizeColor(style.CandidateBackgroundColor),
		"candHighlightColor":        normalizeColor(style.CandidateHighlightColor),
		"candTextColor":             normalizeColor(style.CandidateTextColor),
		"candHighlightTextColor":    normalizeColor(style.CandidateHighlightTextColor),
		"candCommentColor":          normalizeColor(style.CandidateCommentColor),
		"candCommentHighlightColor": normalizeColor(style.CandidateCommentHighlightColor),
		"inlinePreedit":             true,
		"autoPairQuotes":            ime.autoPairQuotes,
		"autoPairRules":             ime.currentAutoPairRules(),
		"semicolonSelectSecond":     ime.semicolonSelectSecond,
	}
}

// Retired appearance command IDs remain inert during a rolling upgrade.
func (ime *IME) writeCandidateCountConfig() bool {
	userDir := ime.userDir()
	if userDir == "" || os.MkdirAll(userDir, 0o755) != nil {
		return false
	}
	count := fixedCandidateCount
	content := fmt.Sprintf("config_version: '%d'\npatch:\n  menu/page_size: %d\n", count, count)
	return os.WriteFile(filepath.Join(userDir, rimeDefaultCustomConfigFileName), []byte(content), 0o644) == nil
}
