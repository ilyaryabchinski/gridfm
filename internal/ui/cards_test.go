package ui_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gridfm/internal/browser"
	"gridfm/internal/ui"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry browser.Entry
		want  ui.Category
	}{
		{"directory", browser.Entry{Name: "src", IsDir: true}, ui.CategoryDir},
		{"go source", browser.Entry{Name: "main.go"}, ui.CategoryGo},
		{"image", browser.Entry{Name: "logo.PNG"}, ui.CategoryImage},
		{"archive", browser.Entry{Name: "source.zip"}, ui.CategoryArchive},
		{"media", browser.Entry{Name: "song.mp3"}, ui.CategoryMedia},
		{"text", browser.Entry{Name: "notes.md"}, ui.CategoryText},
		{"other", browser.Entry{Name: "data.bin"}, ui.CategoryOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ui.Classify(tt.entry); got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.entry.Name, got, tt.want)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		entry browser.Entry
		want  string
	}{
		{browser.Entry{Name: "src", IsDir: true}, "DIR"},
		{browser.Entry{Name: "main.go"}, "GO"},
		{browser.Entry{Name: "logo.png"}, "IMG"},
		{browser.Entry{Name: "source.zip"}, "ARC"},
		{browser.Entry{Name: "notes.md"}, "TXT"},
		{browser.Entry{Name: "data.bin"}, "FILE"},
	}
	for _, tt := range tests {
		if got := ui.Label(tt.entry); got != tt.want {
			t.Errorf("Label(%q) = %q, want %q", tt.entry.Name, got, tt.want)
		}
	}
}

func TestRenderCardIsExactlyCardSize(t *testing.T) {
	t.Parallel()

	size := ui.CardSize{Width: ui.NormalCardWidth, Height: ui.NormalCardHeight}
	card := ui.RenderCard(browser.Entry{Name: "main.go", Path: filepath.Join("/d", "main.go")}, size, false)

	lines := strings.Split(card, "\n")
	if len(lines) != size.Height {
		t.Fatalf("card has %d lines, want %d:\n%s", len(lines), size.Height, card)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != size.Width {
			t.Errorf("card line %d is %d cells wide, want %d (line %q)", i, w, size.Width, line)
		}
	}
}

func TestRenderCardFocusedDiffersFromUnfocused(t *testing.T) {
	t.Parallel()

	e := browser.Entry{Name: "main.go", Path: "/d/main.go"}
	size := ui.CardSize{Width: ui.NormalCardWidth, Height: ui.NormalCardHeight}
	if ui.RenderCard(e, size, false) == ui.RenderCard(e, size, true) {
		t.Error("focused and unfocused cards should render differently")
	}
}

func TestRenderCardSanitizesHostName(t *testing.T) {
	t.Parallel()

	size := ui.CardSize{Width: ui.NormalCardWidth, Height: ui.NormalCardHeight}
	card := ui.RenderCard(browser.Entry{Name: "evil\x1b[31mname", Path: "/d/evil"}, size, false)
	if strings.ContainsRune(card, '\x1b') {
		t.Error("card must not contain raw escape characters from file names")
	}
	if !strings.Contains(card, "[31mname") {
		t.Errorf("sanitized name fragment should be visible, got %q", card)
	}
}

func TestRenderCardCompactSingleContentLine(t *testing.T) {
	t.Parallel()

	size := ui.CardSize{Width: ui.CompactCardWidth, Height: ui.CompactCardHeight}
	card := ui.RenderCard(browser.Entry{Name: "notes.md", Path: "/d/notes.md"}, size, false)
	lines := strings.Split(card, "\n")
	if len(lines) != size.Height {
		t.Fatalf("compact card has %d lines, want %d", len(lines), size.Height)
	}
}
