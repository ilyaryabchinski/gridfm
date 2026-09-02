package ui_test

import (
	"testing"

	"gridfm/internal/ui"
)

func TestComputeLayoutNormal(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(80, 24)
	if l.Card.Width != ui.NormalCardWidth || l.Card.Height != ui.NormalCardHeight {
		t.Errorf("card = %dx%d, want %dx%d", l.Card.Width, l.Card.Height, ui.NormalCardWidth, ui.NormalCardHeight)
	}
	if l.Columns != 5 {
		t.Errorf("Columns = %d, want 5 at 80 columns", l.Columns)
	}
	if l.RowsVisible != 3 {
		t.Errorf("RowsVisible = %d, want 3 at 24 rows", l.RowsVisible)
	}
	if !l.Usable {
		t.Error("80x24 should be usable")
	}
	if l.ContentHeight != 22 {
		t.Errorf("ContentHeight = %d, want 22 (24 minus header and status)", l.ContentHeight)
	}
}

func TestComputeLayoutCompactBelowHeightThreshold(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(60, 15)
	if l.Card.Width != ui.CompactCardWidth || l.Card.Height != ui.CompactCardHeight {
		t.Errorf("card = %dx%d, want compact %dx%d", l.Card.Width, l.Card.Height, ui.CompactCardWidth, ui.CompactCardHeight)
	}
	if l.Columns != 4 {
		t.Errorf("Columns = %d, want 4 at 60 columns", l.Columns)
	}
	if l.RowsVisible != 3 {
		t.Errorf("RowsVisible = %d, want 3 at 15 rows", l.RowsVisible)
	}
}

func TestComputeLayoutUnusable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
	}{
		{"too narrow", 10, 30},
		{"too short", 80, 4},
		{"both", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if l := ui.ComputeLayout(tt.width, tt.height); l.Usable {
				t.Errorf("%dx%d should not be usable", tt.width, tt.height)
			}
		})
	}
}

func TestComputeLayoutColumnsAndRowsNeverZero(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(1, 5)
	if l.Columns < 1 || l.RowsVisible < 1 {
		t.Errorf("columns/rows must be at least 1, got %d/%d", l.Columns, l.RowsVisible)
	}
}

func TestScrollOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		offset, focusRow, visible int
		want                      int
	}{
		{"inside window", 0, 1, 3, 0},
		{"above window", 5, 3, 3, 3},
		{"below window", 0, 3, 3, 1},
		{"at window edge", 0, 2, 3, 0},
		{"one past edge", 0, 3, 3, 1},
		{"degenerate visible", 0, 4, 0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ui.ScrollOffset(tt.offset, tt.focusRow, tt.visible); got != tt.want {
				t.Errorf("ScrollOffset(%d, %d, %d) = %d, want %d",
					tt.offset, tt.focusRow, tt.visible, got, tt.want)
			}
		})
	}
}
