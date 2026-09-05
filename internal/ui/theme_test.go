package ui_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"gridfm/internal/ui"
)

func TestThemeFromMapFillsDefaults(t *testing.T) {
	t.Parallel()

	theme, err := ui.ThemeFromMap(nil)
	if err != nil {
		t.Fatal(err)
	}
	def := ui.DefaultTheme()
	if theme.Accent != def.Accent || theme.Categories[ui.CategoryDir] != def.Categories[ui.CategoryDir] {
		t.Fatal("an empty map must produce the default theme")
	}
}

func TestThemeFromMapOverridesRolesAndCategories(t *testing.T) {
	t.Parallel()

	theme, err := ui.ThemeFromMap(map[string]string{
		"accent": "#ff00ff",
		"dir":    "21",
	})
	if err != nil {
		t.Fatal(err)
	}
	if theme.Accent != lipgloss.Color("#ff00ff") {
		t.Errorf("accent = %v, want the override", theme.Accent)
	}
	if theme.Categories[ui.CategoryDir] != lipgloss.Color("21") {
		t.Errorf("dir = %v, want the override", theme.Categories[ui.CategoryDir])
	}
	// Untouched roles keep defaults.
	if theme.Warning != ui.DefaultTheme().Warning {
		t.Errorf("warning = %v, want the default", theme.Warning)
	}
}

func TestThemeFromMapRejectsBadValuesAndKeys(t *testing.T) {
	t.Parallel()

	if _, err := ui.ThemeFromMap(map[string]string{"accent": "neon"}); err == nil {
		t.Error("a non-color value must be rejected")
	}
	if _, err := ui.ThemeFromMap(map[string]string{"glow": "12"}); err == nil {
		t.Error("an unknown key must be rejected")
	}
}

func TestSetThemeAppliesToRendering(t *testing.T) {
	t.Parallel()

	theme, err := ui.ThemeFromMap(map[string]string{"other": "9"})
	if err != nil {
		t.Fatal(err)
	}
	ui.SetTheme(theme)
	if got := ui.CategoryColor(ui.CategoryOther); got != lipgloss.Color("9") {
		t.Fatalf("CategoryColor(other) = %v, want the installed theme's value", got)
	}

	// Restore the default so parallel package tests are unaffected.
	ui.SetTheme(ui.DefaultTheme())
}
