package rime

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gaboolic/moqi-ime/imecore"
)

func TestLightweightMenuHasFiveTopLevelItems(t *testing.T) {
	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()
	want := []string{"中文输入", "输入方案 · 白霜拼音", "本地 AI · 智能", "个人词库", "设置与诊断"}
	got := make([]string, 0, len(items))
	for _, item := range items {
		if text, _ := item["text"].(string); text != "" {
			got = append(got, text)
		}
	}
	for index, text := range want {
		if index >= len(got) || got[index] != text {
			t.Fatalf("item %d: want %q, got %#v", index, text, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("expected five top-level actions, got %#v", got)
	}
}

func TestLightweightMenuDoesNotExposeRemovedFeatures(t *testing.T) {
	ime := newIsolatedTestIME(t)
	entries := menuTexts(ime.buildMenu())
	removed := []string{"webdav", "云剪贴板", "皮肤", "字体", "翻译", "在线方案", "下载方案"}
	for _, entry := range entries {
		for _, feature := range removed {
			if strings.Contains(strings.ToLower(entry), feature) {
				t.Fatalf("removed feature %q leaked into menu entry %q", feature, entry)
			}
		}
	}
}

func menuTexts(items []map[string]interface{}) []string {
	var result []string
	for _, item := range items {
		if text, ok := item["text"].(string); ok {
			result = append(result, text)
		}
		if submenu, ok := item["submenu"].([]map[string]interface{}); ok {
			result = append(result, menuTexts(submenu)...)
		}
	}
	return result
}

func TestLegacyAppearanceConfigKeepsBehaviorButNotVisuals(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	legacy := map[string]interface{}{
		"font_face":               "SimSun",
		"font_point":              30,
		"candidate_count":         3,
		"candidate_theme":         "purple",
		"auto_pair_quotes":        true,
		"semicolon_select_second": true,
		"synced_options": map[string]bool{
			"ascii_punct": true,
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(moqiAppDataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userAppearanceConfigPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	ime := newTestIME()
	ime.loadAppearancePrefs()
	if !ime.autoPairQuotes || !ime.semicolonSelectSecond || !ime.syncedOptions["ascii_punct"] {
		t.Fatalf("legacy input behavior was not migrated")
	}
	if ime.style.FontFace != "Noto Sans SC" || ime.style.FontPoint != 13 || ime.candidateCount() != 7 || ime.style.CandidateTheme != "mist-shore" {
		t.Fatalf("legacy visuals escaped fixed product style: %#v", ime.style)
	}
}

func TestRetiredAppearanceCommandIsInert(t *testing.T) {
	ime := newIsolatedTestIME(t)
	before := ime.style
	resp := ime.HandleRequest(&imecore.Request{
		Method: "onCommand",
		SeqNum: 1,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_FONT_30, IsInt: true},
	})
	if resp.ReturnValue != 0 {
		t.Fatalf("retired appearance command should be rejected: %#v", resp)
	}
	if ime.style != before {
		t.Fatalf("retired appearance command changed product style")
	}
}
