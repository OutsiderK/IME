package rime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProductRimeDataIsValidAndFocused(t *testing.T) {
	dataDir := filepath.Join("..", "..", "product-data")
	files := []string{
		"default.yaml",
		"rime_frost.schema.yaml",
		"rime_frost_double_pinyin_flypy.schema.yaml",
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var document yaml.Node
		if err := yaml.Unmarshal(data, &document); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}

	defaultData, err := os.ReadFile(filepath.Join(dataDir, "default.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defaultText := string(defaultData)
	if strings.Count(defaultText, "- schema:") != 2 {
		t.Fatalf("product must expose exactly full pinyin and one double-pinyin schema")
	}

	all := ""
	for _, name := range files {
		data, _ := os.ReadFile(filepath.Join(dataDir, name))
		all += strings.ToLower(string(data))
	}
	for _, removed := range []string{"emoji", "chinese_english", "martian", "chaifen", "webdav", "wubi", "t9"} {
		if strings.Contains(all, removed) {
			t.Fatalf("removed feature %q leaked into product Rime data", removed)
		}
	}
	if !strings.Contains(all, "zh-moqi") || !strings.Contains(all, "enable_user_dict: true") {
		t.Fatal("language model and Rime user learning must remain enabled")
	}
}
