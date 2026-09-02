package ui_test

import (
	"testing"

	"gridfm/internal/ui"
)

func TestComputeLayoutNormalWithSidebar(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(80, 24, ui.ZoomNormal, true)
	if l.Card.Width != ui.NormalCardWidth || l.Card.Height != ui.NormalCardHeight {
		t.Errorf("card = %dx%d, want %dx%d", l.Card.Width, l.Card.Height, ui.NormalCardWidth, ui.NormalCardHeight)
	}
	if !l.SidebarVisible || l.SidebarOverlay {
		t.Errorf("80 columns should dock the sidebar, got visible=%v overlay=%v", l.SidebarVisible, l.SidebarOverlay)
	}
	if l.SidebarWidth != 20 {
		t.Errorf("SidebarWidth = %d, want 20", l.SidebarWidth)
	}
	// 80 minus the 20-cell sidebar leaves 60 for the grid: three columns
	// of 14-cell cards with 2-cell gaps (4 columns would need 62).
	if l.Columns != 3 {
		t.Errorf("Columns = %d, want 3 at 60 grid columns", l.Columns)
	}
	if l.GridWidth != 60 {
		t.Errorf("GridWidth = %d, want 60", l.GridWidth)
	}
	if l.RowsVisible != 3 {
		t.Errorf("RowsVisible = %d, want 3 at 24 rows", l.RowsVisible)
	}
	if !l.Usable {
		t.Error("80x24 should be usable")
	}
}

func TestComputeLayoutSidebarHidden(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(80, 24, ui.ZoomNormal, false)
	if l.SidebarVisible || l.SidebarOverlay {
		t.Error("sidebar should be fully hidden when toggled off")
	}
	if l.GridWidth != 80 || l.Columns != 5 {
		t.Errorf("grid should span the full width: %d cells, %d columns", l.GridWidth, l.Columns)
	}
}

func TestComputeLayoutNarrowOverlay(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(60, 15, ui.ZoomNormal, true)
	if l.SidebarVisible {
		t.Error("narrow terminals should not dock the sidebar")
	}
	if !l.SidebarOverlay {
		t.Error("narrow terminals should report overlay mode when the sidebar is requested")
	}
	if l.GridWidth != 60 {
		t.Errorf("overlay mode keeps the full grid width, got %d", l.GridWidth)
	}

	// Without the request there is no overlay either.
	if l := ui.ComputeLayout(60, 15, ui.ZoomNormal, false); l.SidebarOverlay {
		t.Error("no overlay when the sidebar is toggled off")
	}
}

func TestSidebarWidthClamped(t *testing.T) {
	t.Parallel()

	if got := ui.SidebarWidthFor(80); got != 20 {
		t.Errorf("SidebarWidthFor(80) = %d, want 20", got)
	}
	// Tight wide layouts shrink toward the 16-cell floor.
	if got := ui.SidebarWidthFor(71); got != 17 {
		t.Errorf("SidebarWidthFor(71) = %d, want 17", got)
	}
	// Very wide layouts stay at the requested width.
	if got := ui.SidebarWidthFor(1000); got != 20 {
		t.Errorf("SidebarWidthFor(1000) = %d, want 20", got)
	}
	if got := ui.SidebarWidthFor(40); got != 0 {
		t.Errorf("SidebarWidthFor(40) = %d, want 0 (narrow)", got)
	}
}

func TestComputeLayoutZoomChangesCards(t *testing.T) {
	t.Parallel()

	normal := ui.ComputeLayout(80, 24, ui.ZoomNormal, false)
	if normal.Card.Width != ui.NormalCardWidth {
		t.Errorf("normal card width = %d", normal.Card.Width)
	}

	detailed := ui.ComputeLayout(80, 24, ui.ZoomDetailed, false)
	if detailed.Card.Width != ui.DetailedCardWidth || detailed.Card.Height != ui.DetailedCardHeight {
		t.Errorf("detailed card = %dx%d", detailed.Card.Width, detailed.Card.Height)
	}
	if detailed.Columns != 4 {
		t.Errorf("Columns = %d, want 4 with wider detailed cards", detailed.Columns)
	}
}

func TestComputeLayoutCompactForcesZoom(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(60, 15, ui.ZoomDetailed, false)
	if l.Zoom != ui.ZoomCompact {
		t.Errorf("short terminals force compact, got %v", l.Zoom)
	}
	if l.Card.Height != ui.CompactCardHeight {
		t.Errorf("card height = %d, want compact", l.Card.Height)
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

			if l := ui.ComputeLayout(tt.width, tt.height, ui.ZoomNormal, false); l.Usable {
				t.Errorf("%dx%d should not be usable", tt.width, tt.height)
			}
		})
	}
}

func TestComputeLayoutColumnsAndRowsNeverZero(t *testing.T) {
	t.Parallel()

	l := ui.ComputeLayout(1, 5, ui.ZoomCompact, false)
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
