package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
)

// Theme names the semantic color roles of the interface. Render code
// asks for roles, never raw colors, so a user's palette applies
// everywhere at once.
type Theme struct {
	// Accent colors titles, section headers, and breadcrumbs.
	Accent lipgloss.Color
	// Muted colors panel borders and quiet chrome.
	Muted lipgloss.Color
	// Warning colors notes, read-only markers, and confirmations.
	Warning lipgloss.Color
	// Info colors filter and selection indicators.
	Info lipgloss.Color
	// Strong colors the focused border and the mode label.
	Strong lipgloss.Color
	// Categories maps file-type categories to their colors.
	Categories map[Category]lipgloss.Color
}

// DefaultTheme is the palette the interface shipped with.
func DefaultTheme() Theme {
	return Theme{
		Accent:  lipgloss.Color("12"), // bright cyan
		Muted:   lipgloss.Color("8"),  // gray
		Warning: lipgloss.Color("3"),  // yellow
		Info:    lipgloss.Color("6"),  // cyan
		Strong:  lipgloss.Color("15"), // bright white
		Categories: map[Category]lipgloss.Color{
			CategoryDir:     lipgloss.Color("4"),  // blue
			CategoryGo:      lipgloss.Color("14"), // bright cyan
			CategoryImage:   lipgloss.Color("5"),  // magenta
			CategoryArchive: lipgloss.Color("3"),  // yellow
			CategoryMedia:   lipgloss.Color("6"),  // cyan
			CategoryText:    lipgloss.Color("2"),  // green
			CategoryOther:   lipgloss.Color("7"),  // gray
		},
	}
}

// categoryKeys maps config spelling to categories.
var categoryKeys = map[string]Category{
	"dir": CategoryDir, "go": CategoryGo, "image": CategoryImage,
	"archive": CategoryArchive, "media": CategoryMedia,
	"text": CategoryText, "other": CategoryOther,
}

// ThemeFromMap builds a theme from config strings, filling every absent
// or empty entry from the default palette. Values must be ANSI 0-255 or
// #hex; anything else is an error naming the key.
func ThemeFromMap(values map[string]string) (Theme, error) {
	theme := DefaultTheme()

	roles := map[string]*lipgloss.Color{
		"accent":  &theme.Accent,
		"muted":   &theme.Muted,
		"warning": &theme.Warning,
		"info":    &theme.Info,
		"strong":  &theme.Strong,
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for _, key := range keys {
		value := values[key]
		if value == "" {
			continue
		}
		if err := validColor(value); err != nil {
			return theme, fmt.Errorf("theme: invalid %s %q: %w", key, value, err)
		}

		if cat, ok := categoryKeys[key]; ok {
			theme.Categories[cat] = lipgloss.Color(value)

			continue
		}
		if role, ok := roles[key]; ok {
			*role = lipgloss.Color(value)

			continue
		}

		return theme, fmt.Errorf("theme: unknown key %q", key)
	}

	return theme, nil
}

// validColor accepts ANSI color numbers 0-255 and #hex (3 or 6 digits).
func validColor(value string) error {
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		if len(hex) == 3 || len(hex) == 6 {
			if _, err := strconv.ParseUint(hex, 16, 64); err == nil {
				return nil
			}
		}

		return fmt.Errorf("want #rgb or #rrggbb")
	}

	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 255 {
		return fmt.Errorf("want an ANSI color 0-255 or #hex")
	}

	return nil
}

// current is the active palette, replaced atomically: rendering runs on
// its own goroutine while tests install themes concurrently.
var current atomic.Pointer[Theme]

func init() {
	def := DefaultTheme()
	current.Store(&def)
}

// SetTheme installs the palette used by all rendering.
func SetTheme(t Theme) {
	if t.Categories == nil {
		// A theme without categories keeps the default category colors.
		t.Categories = DefaultTheme().Categories
	}
	current.Store(&t)
}

// CurrentTheme returns the active palette.
func CurrentTheme() Theme { return *current.Load() }

// CategoryColor returns the active color for a file-type category.
func CategoryColor(c Category) lipgloss.Color {
	return CurrentTheme().Categories[c]
}
