package ui

import (
	"path/filepath"
	"strings"

	"gridfm/internal/browser"

	"github.com/charmbracelet/lipgloss"
)

// Category is the placeholder file-type classification used for labels and
// colors until icon themes arrive.
type Category int

// Placeholder file-type categories, ordered by their numeric identity for
// stable styling.
const (
	CategoryDir Category = iota
	CategoryImage
	CategoryArchive
	CategoryMedia
	CategoryText
	CategoryOther
)

// categoryExt maps file extensions to their placeholder category.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var categoryExt = map[string]Category{
	".png": CategoryImage, ".jpg": CategoryImage, ".jpeg": CategoryImage,
	".gif": CategoryImage, ".webp": CategoryImage, ".svg": CategoryImage,
	".bmp": CategoryImage, ".ico": CategoryImage,

	".zip": CategoryArchive, ".tar": CategoryArchive, ".gz": CategoryArchive,
	".tgz": CategoryArchive, ".xz": CategoryArchive, ".7z": CategoryArchive,
	".rar": CategoryArchive, ".bz2": CategoryArchive, ".zst": CategoryArchive,

	".mp3": CategoryMedia, ".flac": CategoryMedia, ".ogg": CategoryMedia,
	".wav": CategoryMedia, ".mp4": CategoryMedia, ".mkv": CategoryMedia,
	".webm": CategoryMedia, ".avi": CategoryMedia, ".mov": CategoryMedia,

	".txt": CategoryText, ".md": CategoryText, ".toml": CategoryText,
	".yaml": CategoryText, ".yml": CategoryText, ".json": CategoryText,
	".conf": CategoryText, ".ini": CategoryText, ".log": CategoryText,
}

// categoryLabel maps categories to their short display label.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var categoryLabel = map[Category]string{
	CategoryDir:     "DIR",
	CategoryImage:   "IMG",
	CategoryArchive: "ARC",
	CategoryMedia:   "AV",
	CategoryText:    "TXT",
	CategoryOther:   "FILE",
}

// categoryColor maps categories to their ANSI color.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var categoryColor = map[Category]lipgloss.Color{
	CategoryDir:     lipgloss.Color("4"), // blue
	CategoryImage:   lipgloss.Color("5"), // magenta
	CategoryArchive: lipgloss.Color("3"), // yellow
	CategoryMedia:   lipgloss.Color("6"), // cyan
	CategoryText:    lipgloss.Color("2"), // green
	CategoryOther:   lipgloss.Color("7"), // gray
}

// Classify maps an entry to its placeholder category. The .go extension gets
// a dedicated label as a nod to the project's own working directory.
func Classify(e browser.Entry) Category {
	if e.IsDir {
		return CategoryDir
	}
	if strings.EqualFold(filepath.Ext(e.Name), ".go") {
		return CategoryText
	}
	if c, ok := categoryExt[strings.ToLower(filepath.Ext(e.Name))]; ok {
		return c
	}

	return CategoryOther
}

// Label returns the short placeholder type label for an entry.
func Label(e browser.Entry) string {
	if e.IsDir {
		return categoryLabel[CategoryDir]
	}
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(e.Name)), "."); ext == "go" {
		return "GO"
	}

	return categoryLabel[Classify(e)]
}

// RenderCard draws one fixed-size card. Size includes the border.
func RenderCard(e browser.Entry, size CardSize, focused bool) string {
	innerWidth := size.Width - 2
	innerHeight := size.Height - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	category := Classify(e)
	label := categoryStyle(category).Render(Label(e))
	name := categoryStyle(category).Bold(focused).Render(TruncateName(SanitizeName(e.Name), innerWidth))

	var lines []string
	if innerHeight >= 3 {
		lines = append(lines,
			lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, label),
			"",
			name,
		)
	} else {
		lines = append(lines, name)
	}

	border := lipgloss.RoundedBorder()
	borderForeground := categoryColor[category]
	if focused {
		// Strong border plus high-contrast foreground for the focused card.
		border = lipgloss.DoubleBorder()
		borderForeground = lipgloss.Color("15") // bright white
	}
	body := lipgloss.JoinVertical(lipgloss.Center, lines...)

	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderForeground).
		Width(innerWidth).
		Height(innerHeight).
		Render(body)
}

func categoryStyle(c Category) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(categoryColor[c])
}
