package ui

// Card dimensions and gaps follow the initial sizing targets from the
// design: normal cards are 14x5, compact 12x3, detailed 18x6, with 2 column
// and 1 row gaps. They are tuned through interactive use, not API
// guarantees.
const (
	NormalCardWidth    = 14
	NormalCardHeight   = 5
	CompactCardWidth   = 12
	CompactCardHeight  = 3
	DetailedCardWidth  = 18
	DetailedCardHeight = 6

	CardGapX = 2
	CardGapY = 1

	HeaderHeight    = 1
	StatusBarHeight = 1

	// CompactHeightThreshold switches cards to compact mode below this many
	// terminal rows regardless of the requested zoom.
	CompactHeightThreshold = 20

	// NarrowThreshold is the width below which the docked sidebar hides and
	// becomes a toggleable overlay.
	NarrowThreshold = 70

	// SidebarWidth, MinSidebarWidth, and MaxSidebarWidth bound the docked
	// sidebar width.
	SidebarWidth    = 20
	MinSidebarWidth = 16
	MaxSidebarWidth = 28
)

// Layout is the derived responsive geometry for one terminal size, zoom,
// and sidebar request.
type Layout struct {
	// Zoom is the effective zoom: the requested one unless the terminal is
	// too short, which forces compact.
	Zoom ZoomLevel
	Card CardSize

	Columns     int
	RowsVisible int

	// SidebarWidth is the docked sidebar width, 0 when the sidebar is
	// hidden or narrow-overlay mode is active.
	SidebarWidth int
	// SidebarOverlay is true in narrow mode when the sidebar is toggled on:
	// the grid keeps the full width and the sidebar floats above it.
	SidebarOverlay bool
	SidebarVisible bool

	// GridWidth is the width available to the grid; the full terminal
	// width except in docked-sidebar mode.
	GridWidth int

	// ContentWidth and ContentHeight are the cells available below the
	// header and above the status bar.
	ContentWidth  int
	ContentHeight int

	// Usable is false when the terminal is too small to show even one card.
	Usable bool
}

// SidebarWidthFor returns the docked sidebar width for a terminal width.
// Wide layouts get the requested 20 cells, shrinking toward the 16-cell
// floor on tight widths; zero means no docked sidebar.
func SidebarWidthFor(width int) int {
	if width < NarrowThreshold {
		return 0
	}

	return min(max(width/4, MinSidebarWidth), SidebarWidth)
}

// ComputeLayout derives the grid geometry from terminal dimensions using
// the formulas from the design:
//
//	columns = max(1, floor((gridWidth + gapX) / (cardWidth + gapX)))
//	rowsVisible = max(1, floor((contentHeight + gapY) / (cardHeight + gapY)))
func ComputeLayout(width, height int, zoom ZoomLevel, sidebarRequested bool) Layout {
	narrow := width < NarrowThreshold
	sidebarWidth := SidebarWidthFor(width)
	sidebarVisible := sidebarRequested && sidebarWidth > 0
	overlay := sidebarRequested && narrow

	if height < CompactHeightThreshold {
		zoom = ZoomCompact
	}
	card := zoom.CardSize()

	gridWidth := width
	if sidebarVisible {
		gridWidth = width - sidebarWidth
	}

	contentHeight := height - HeaderHeight - StatusBarHeight

	columns := (gridWidth + CardGapX) / (card.Width + CardGapX)
	rows := (contentHeight + CardGapY) / (card.Height + CardGapY)

	usable := columns >= 1 && rows >= 1 &&
		gridWidth >= card.Width && contentHeight >= card.Height
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}

	return Layout{
		Zoom:           zoom,
		Card:           card,
		Columns:        columns,
		RowsVisible:    rows,
		SidebarWidth:   sidebarWidth,
		SidebarOverlay: overlay,
		SidebarVisible: sidebarVisible,
		GridWidth:      gridWidth,
		ContentWidth:   width,
		ContentHeight:  contentHeight,
		Usable:         usable,
	}
}

// ScrollOffset returns the first visible row so the focused row stays inside
// the rowsVisible window. offset is the current first visible row.
func ScrollOffset(offset, focusRow, rowsVisible int) int {
	if rowsVisible < 1 {
		rowsVisible = 1
	}
	switch {
	case focusRow < offset:
		return focusRow
	case focusRow >= offset+rowsVisible:
		return focusRow - rowsVisible + 1
	default:
		return offset
	}
}
