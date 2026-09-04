package rime

import "testing"

type ThemeDefinition struct {
	ID string
}

func listThemes() []ThemeDefinition {
	return []ThemeDefinition{{ID: "purple"}}
}

func themeCommandID(index int) int {
	return ID_APPEARANCE_THEME_BASE + index
}

func (ime *IME) buildLegacyMenu() []map[string]interface{} {
	return ime.buildMenu()
}

// Kept while legacy appearance persistence tests are migrated to fixed product
// styling. The compact product menu no longer exposes these commands.
func themeCommandForTheme(t *testing.T, themeID string) int {
	t.Helper()
	for index, theme := range listThemes() {
		if theme.ID == themeID {
			return themeCommandID(index)
		}
	}
	t.Fatalf("theme %q not found", themeID)
	return 0
}
