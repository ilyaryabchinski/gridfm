// Package ui renders the terminal presentation of browser state: text
// hygiene, responsive layout math, cards, and bars. Rendering is pure string
// work and performs no I/O.
package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// SanitizeName makes a filesystem-provided name safe for terminal display.
// Invalid UTF-8 is replaced and control characters (including tabs,
// newlines, and escape sequences) are stripped, so a name can never inject
// terminal markup.
func SanitizeName(name string) string {
	if utf8.ValidString(name) && strings.IndexFunc(name, unicode.IsControl) < 0 {
		return name
	}

	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return -1
		}

		return r
	}, name)
}

// TruncateName clips a display string to the given cell width, appending an
// ellipsis when content is cut.
func TruncateName(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}

	return ansi.Truncate(s, width, "…")
}
