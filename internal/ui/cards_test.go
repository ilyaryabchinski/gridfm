package ui_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		{"symlink", browser.Entry{Name: "link", Symlink: true}, ui.CategoryOther},
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

func TestIconModesProvideGlyphForEveryCategory(t *testing.T) {
	t.Parallel()

	categories := []ui.Category{
		ui.CategoryDir, ui.CategoryGo, ui.CategoryImage, ui.CategoryArchive,
		ui.CategoryMedia, ui.CategoryText, ui.CategoryOther,
	}
	modes := []ui.IconMode{ui.IconModeLabel, ui.IconModeUnicode, ui.IconModeNerdFont}

	for _, mode := range modes {
		for _, c := range categories {
			if got := mode.Glyph(c); got == "" {
				t.Errorf("mode %s has no glyph for category %v", mode, c)
			}
		}
	}

	// Modes are distinguishable: the three representations of a directory
	// must all differ so a missing font is obvious.
	if ui.IconModeLabel.Glyph(ui.CategoryDir) == ui.IconModeNerdFont.Glyph(ui.CategoryDir) {
		t.Error("label and nerdfont glyphs should differ")
	}
}

func TestIconModeParseAndString(t *testing.T) {
	t.Parallel()

	for _, mode := range []ui.IconMode{ui.IconModeLabel, ui.IconModeUnicode, ui.IconModeNerdFont} {
		parsed, err := ui.ParseIconMode(mode.String())
		if err != nil || parsed != mode {
			t.Errorf("round trip of %s failed: %v, %v", mode, parsed, err)
		}
	}

	_, err := ui.ParseIconMode("sparkles")
	if err == nil {
		t.Error("unknown icon mode should fail to parse")
	}
}

func TestRenderCardMatchesZoomGeometry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		zoom ui.ZoomLevel
	}{
		{ui.ZoomCompact}, {ui.ZoomNormal}, {ui.ZoomDetailed},
	}
	for _, tt := range tests {
		t.Run(tt.zoom.String(), func(t *testing.T) {
			t.Parallel()

			size := tt.zoom.CardSize()
			entry := browser.Entry{Name: "main.go", Path: filepath.Join("/d", "main.go")}
			card := ui.RenderCard(entry, tt.zoom, false, ui.IconModeLabel)

			lines := strings.Split(card, "\n")
			if len(lines) != size.Height {
				t.Fatalf("card has %d lines, want %d:\n%s", len(lines), size.Height, card)
			}
			for i, line := range lines {
				if w := ansi.StringWidth(line); w != size.Width {
					t.Errorf("card line %d is %d cells wide, want %d", i, w, size.Width)
				}
			}
		})
	}
}

func TestRenderCompactCardShowsIconAndName(t *testing.T) {
	t.Parallel()

	entry := browser.Entry{Name: "main.go", Path: "/d/main.go"}
	card := ui.RenderCard(entry, ui.ZoomCompact, false, ui.IconModeUnicode)

	if !strings.Contains(card, "🐹") {
		t.Errorf("compact card should include the type glyph, got:\n%s", card)
	}
	if !strings.Contains(card, "main.go") {
		t.Errorf("compact card should include the name, got:\n%s", card)
	}
	// A long name truncates but keeps both icon and name visible.
	entry.Name = "a-very-long-file-name.go"
	card = ui.RenderCard(entry, ui.ZoomCompact, false, ui.IconModeUnicode)
	if !strings.Contains(card, "🐹") || !strings.Contains(card, "…") {
		t.Errorf("compact card should truncate the name with an ellipsis, got:\n%s", card)
	}
	lines := strings.Split(card, "\n")
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != ui.CompactCardWidth {
			t.Errorf("compact card line %d is %d cells, want %d", i, w, ui.CompactCardWidth)
		}
	}
}

func TestRenderDetailedCardShowsMetadata(t *testing.T) {
	t.Parallel()

	modTime := time.Unix(1700000000, 0)
	entry := browser.Entry{
		Name: "main.go", Path: "/d/main.go", Size: 4321,
		Mode: 0o644, ModTime: modTime,
	}

	card := ui.RenderCard(entry, ui.ZoomDetailed, false, ui.IconModeLabel)
	if !strings.Contains(card, "4.2K") {
		t.Errorf("detailed card should show the human size, got:\n%s", card)
	}
	if !strings.Contains(card, "-rw-r--r--") {
		t.Errorf("detailed card should show permissions, got:\n%s", card)
	}
	if !strings.Contains(card, modTime.Format("2006-01-02 15:04")) {
		t.Errorf("detailed card should show the modification time, got:\n%s", card)
	}
}

func TestRenderCardIconModesChangeGlyph(t *testing.T) {
	t.Parallel()

	entry := browser.Entry{Name: "src", Path: "/d/src", IsDir: true}
	zoom := ui.ZoomNormal

	label := ui.RenderCard(entry, zoom, false, ui.IconModeLabel)
	unicode := ui.RenderCard(entry, zoom, false, ui.IconModeUnicode)
	if label == unicode {
		t.Error("label and unicode modes should render differently")
	}
	if !strings.Contains(label, "DIR") {
		t.Errorf("label mode should contain the text label, got:\n%s", label)
	}
	if !strings.Contains(unicode, "📁") {
		t.Errorf("unicode mode should contain the glyph, got:\n%s", unicode)
	}
}

func TestRenderCardFocusedDiffersFromUnfocused(t *testing.T) {
	t.Parallel()

	e := browser.Entry{Name: "main.go", Path: "/d/main.go"}
	zoom := ui.ZoomNormal
	if ui.RenderCard(e, zoom, false, ui.IconModeLabel) == ui.RenderCard(e, zoom, true, ui.IconModeLabel) {
		t.Error("focused and unfocused cards should render differently")
	}
}

func TestRenderCardSanitizesHostName(t *testing.T) {
	t.Parallel()

	card := ui.RenderCard(browser.Entry{Name: "evil\x1b[31mname", Path: "/d/evil"}, ui.ZoomNormal, false, ui.IconModeLabel)
	if strings.ContainsRune(card, '\x1b') {
		t.Error("card must not contain raw escape characters from file names")
	}
	if !strings.Contains(card, "[31mname") {
		t.Errorf("sanitized name fragment should be visible, got %q", card)
	}
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{4321, "4.2K"},
		{1024 * 1024, "1.0M"},
		{1536 * 1024 * 1024, "1.5G"},
	}
	for _, tt := range tests {
		if got := ui.HumanBytes(tt.bytes); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestTimestamp(t *testing.T) {
	t.Parallel()

	if got := ui.Timestamp(time.Time{}); got != "----  --:--" {
		t.Errorf("Timestamp(zero) = %q, want dashes", got)
	}

	ts := time.Unix(1700000000, 0)
	want := ts.Format("2006-01-02 15:04")
	if got := ui.Timestamp(ts); got != want {
		t.Errorf("Timestamp = %q, want %q", got, want)
	}
}
