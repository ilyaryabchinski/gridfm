package ui

import (
	"errors"
	"fmt"

	"gridfm/internal/browser"
)

// IconMode selects how file types are represented on cards and in lists.
type IconMode int

// iconModeLabels is the display and parse name of IconModeLabel.
const iconModeLabels = "labels"

const (
	// IconModeLabel shows short text labels such as DIR or GO: the base
	// experience that renders everywhere.
	IconModeLabel IconMode = iota
	// IconModeUnicode shows plain Unicode glyphs available in most fonts.
	IconModeUnicode
	// IconModeNerdFont shows Nerd Font glyphs; it requires a patched font
	// and falls back cleanly to labels when the terminal cannot render it.
	IconModeNerdFont
)

// ErrUnknownIconMode is returned by ParseIconMode for unrecognized input.
var ErrUnknownIconMode = errors.New("unknown icon mode")

// String renders the icon mode for display and flag help.
func (m IconMode) String() string {
	switch m {
	case IconModeUnicode:
		return "unicode"
	case IconModeNerdFont:
		return "nerdfont"
	case IconModeLabel:
		return iconModeLabels
	}

	return iconModeLabels
}

// ParseIconMode parses an icon mode from a flag or config value.
func ParseIconMode(s string) (IconMode, error) {
	switch s {
	case iconModeLabels:
		return IconModeLabel, nil
	case "unicode":
		return IconModeUnicode, nil
	case "nerdfont":
		return IconModeNerdFont, nil
	}

	return IconModeLabel, fmt.Errorf("%w: %q", ErrUnknownIconMode, s)
}

// unicodeGlyphs are widely available pictographs for each category.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var unicodeGlyphs = map[Category]string{
	CategoryDir:     "📁",
	CategoryGo:      "🐹",
	CategoryImage:   "🖼",
	CategoryArchive: "📦",
	CategoryMedia:   "🎵",
	CategoryText:    "📄",
	CategoryOther:   "📄",
}

// nerdFontGlyphs use the Nerd Font private use area (v3 glyph names).
//
//nolint:gochecknoglobals // static lookup table, never mutated
var nerdFontGlyphs = map[Category]string{
	CategoryDir:     "\uf07b",
	CategoryGo:      "\ue626",
	CategoryImage:   "\uf1c5",
	CategoryArchive: "\uf1c6",
	CategoryMedia:   "\uf001",
	CategoryText:    "\uf15c",
	CategoryOther:   "\uf15b",
}

// Glyph returns the representation of an entry's type for the mode. Label
// mode returns the short text label; unknown categories in glyph modes fall
// back to the plain-text label so unsupported icons degrade cleanly.
func (m IconMode) Glyph(c Category) string {
	switch m {
	case IconModeUnicode:
		if g, ok := unicodeGlyphs[c]; ok {
			return g
		}
	case IconModeNerdFont:
		if g, ok := nerdFontGlyphs[c]; ok {
			return g
		}
	case IconModeLabel:
		return categoryLabel[c]
	}

	return categoryLabel[c]
}

// GlyphFor returns the representation for an entry under the mode.
func (m IconMode) GlyphFor(e browser.Entry) string {
	return m.Glyph(Classify(e))
}
