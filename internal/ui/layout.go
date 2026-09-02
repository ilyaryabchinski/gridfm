package ui

// Card dimensions and gaps follow the initial sizing targets from the
// design: normal cards are 14x5, compact 12x3, with 2 column and 1 row gaps.
// They are tuned through interactive use, not API guarantees.
const (
	NormalCardWidth   = 14
	NormalCardHeight  = 5
	CompactCardWidth  = 12
	CompactCardHeight = 3

	CardGapX = 2
	CardGapY = 1

	HeaderHeight    = 1
	StatusBarHeight = 1

	// CompactHeightThreshold switches cards to compact mode below this many
	// terminal rows.
	CompactHeightThreshold = 20
)

// CardSize holds the outer cell dimensions of one card, border included.
type CardSize struct {
	Width  int
	Height int
}

// Layout is the derived responsive geometry for one terminal size.
type Layout struct {
	Card CardSize

	Columns     int
	RowsVisible int

	// ContentWidth and ContentHeight are the cells available to the grid
	// after the header and status bar.
	ContentWidth  int
	ContentHeight int

	// Usable is false when the terminal is too small to show even one card.
	Usable bool
}

// ComputeLayout derives the grid geometry from terminal dimensions using the
// formulas from the design:
//
//	columns = max(1, floor((width + gapX) / (cardWidth + gapX)))
//	rowsVisible = max(1, floor((contentHeight + gapY) / (cardHeight + gapY)))
func ComputeLayout(width, height int) Layout {
	card := CardSize{Width: NormalCardWidth, Height: NormalCardHeight}
	if height < CompactHeightThreshold {
		card = CardSize{Width: CompactCardWidth, Height: CompactCardHeight}
	}

	contentWidth := width
	contentHeight := height - HeaderHeight - StatusBarHeight

	columns := (contentWidth + CardGapX) / (card.Width + CardGapX)
	rows := (contentHeight + CardGapY) / (card.Height + CardGapY)

	usable := columns >= 1 && rows >= 1 &&
		contentWidth >= card.Width && contentHeight >= card.Height
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}

	return Layout{
		Card:          card,
		Columns:       columns,
		RowsVisible:   rows,
		ContentWidth:  contentWidth,
		ContentHeight: contentHeight,
		Usable:        usable,
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
