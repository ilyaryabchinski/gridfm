package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gridfm/internal/ui"
)

func TestSanitizeNameStripsControlCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "README.md", "README.md"},
		{"escape sequence", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"tab and newline", "a\tb\nc", "abc"},
		{"carriage return", "bad\r\nname", "badname"},
		{"null byte", "nul\x00name", "nulname"},
		{"invalid utf-8", "bad\xffutf8", "badutf8"},
		{"unicode kept", "日本語-émoji🎉", "日本語-émoji🎉"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ui.SanitizeName(tt.input); got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeNameOutputNeverContainsControlRunes(t *testing.T) {
	t.Parallel()

	inputs := []string{"\x1b]0;title\x07x", "a\rb\nc\td\x07e\x08f", "\xff\xfe"}
	for _, in := range inputs {
		for _, r := range ui.SanitizeName(in) {
			if r < 0x20 || r == 0x7f {
				t.Errorf("SanitizeName(%q) emitted control rune %q", in, r)
			}
		}
	}
}

func TestTruncateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		width int
	}{
		{"fits", "main.go", "main.go", 10},
		{"exact fit", "main.go", "main.go", 7},
		{"truncated", "main.go", "main…", 5},
		{"zero width", "main.go", "", 0},
		{"negative width", "main.go", "", -1},
		{"wide runes", "日本語", "日…", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ui.TruncateName(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("TruncateName(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
			if tt.width > 0 && ansi.StringWidth(got) > tt.width {
				t.Errorf("result %q is %d cells wide, want <= %d", got, ansi.StringWidth(got), tt.width)
			}
		})
	}
}

func TestSanitizeThenTruncatePreservesSingleLine(t *testing.T) {
	t.Parallel()

	got := ui.TruncateName(ui.SanitizeName("a\nb"), 10)
	if strings.Contains(got, "\n") {
		t.Errorf("sanitized and truncated name should be a single line, got %q", got)
	}
	if got != "ab" {
		t.Errorf("sanitized and truncated name = %q, want %q", got, "ab")
	}
}
