package ui

import (
	"fmt"
	"time"

	"gridfm/internal/browser"
)

// ZoomLevel selects card density: compact, normal, or detailed.
type ZoomLevel int

// Zoom levels ordered from most to least dense.
const (
	ZoomCompact ZoomLevel = iota
	ZoomNormal
	ZoomDetailed
)

// ZoomIn steps from compact toward detailed; at detailed it stays.
func (z ZoomLevel) ZoomIn() ZoomLevel { return min(z+1, ZoomDetailed) }

// ZoomOut steps from detailed toward compact; at compact it stays.
func (z ZoomLevel) ZoomOut() ZoomLevel { return max(z-1, ZoomCompact) }

// String renders the zoom level for the status bar.
func (z ZoomLevel) String() string {
	switch z {
	case ZoomCompact:
		return "compact"
	case ZoomDetailed:
		return "detailed"
	case ZoomNormal:
		return "normal"
	}

	return "normal"
}

// CardSize holds the outer cell dimensions of one card, border included.
type CardSize struct {
	Width  int
	Height int
}

// CardSize returns the outer card dimensions for the zoom level, following
// the initial sizing targets from the design. Detailed adds room for
// metadata lines.
func (z ZoomLevel) CardSize() CardSize {
	switch z {
	case ZoomCompact:
		return CardSize{Width: CompactCardWidth, Height: CompactCardHeight}
	case ZoomDetailed:
		return CardSize{Width: DetailedCardWidth, Height: DetailedCardHeight}
	case ZoomNormal:
		return CardSize{Width: NormalCardWidth, Height: NormalCardHeight}
	}

	return CardSize{Width: NormalCardWidth, Height: NormalCardHeight}
}

// HumanBytes renders a byte count in compact human form: raw bytes below
// 1 KiB, then one decimal for kibibytes and above.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}

	value := float64(n)
	units := []string{"K", "M", "G", "T", "P"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, suffix)
			}

			return fmt.Sprintf("%.0f%s", value, suffix)
		}
	}

	return fmt.Sprintf("%.0fP", value/unit)
}

// Timestamp renders a modification time in the fixed short form used on
// detailed cards. A zero time renders as dashes.
func Timestamp(t time.Time) string {
	if t.IsZero() {
		return "----  --:--"
	}

	return t.Format("2006-01-02 15:04")
}

// EntryMeta returns the two metadata lines shown on detailed cards: size
// with permissions, and the modification timestamp.
func EntryMeta(e browser.Entry) (string, string) {
	return HumanBytes(e.Size) + "  " + e.Permissions(), Timestamp(e.ModTime)
}
