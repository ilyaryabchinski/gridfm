package browser_test

import (
	"testing"

	"gridfm/internal/browser"
)

func TestGridLayoutCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		count    int
		columns  int
		wantRows int
	}{
		{"empty", 0, 3, 0},
		{"single incomplete row", 2, 3, 1},
		{"exact rows", 6, 3, 2},
		{"incomplete last row", 7, 3, 3},
		{"single column", 5, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := browser.NewGrid(tt.count, tt.columns)
			if got := g.Rows(); got != tt.wantRows {
				t.Errorf("Rows() = %d, want %d", got, tt.wantRows)
			}
			if got := g.Focus(); got != 0 {
				t.Errorf("Focus() = %d, want 0", got)
			}
			if got := g.Columns(); got != max(tt.columns, 1) {
				t.Errorf("Columns() = %d, want %d", got, max(tt.columns, 1))
			}
		})
	}
}

func TestGridRightMovesWithoutWrapping(t *testing.T) {
	t.Parallel()

	// 7 entries, 3 columns:
	//   row 0: 0 1 2
	//   row 1: 3 4 5
	//   row 2: 6
	g := browser.NewGrid(7, 3)

	if !g.Right() || g.Focus() != 1 {
		t.Errorf("Right moved focus to %d, want 1", g.Focus())
	}
	if !g.Right() || g.Focus() != 2 {
		t.Errorf("Right moved focus to %d, want 2", g.Focus())
	}
	if g.Right() {
		t.Error("Right from the right edge should not wrap rows")
	}
}

func TestGridLeftMovesWithoutWrapping(t *testing.T) {
	t.Parallel()

	g := browser.NewGrid(7, 3)
	g.SetFocus(2)

	if !g.Left() || g.Focus() != 1 {
		t.Errorf("Left moved focus to %d, want 1", g.Focus())
	}
	if !g.Left() || g.Focus() != 0 {
		t.Errorf("Left moved focus to %d, want 0", g.Focus())
	}
	if g.Left() {
		t.Error("Left from column 0 should be a no-op")
	}
}

func TestGridDownIntoShorterLastRow(t *testing.T) {
	t.Parallel()

	// 7 entries, 3 columns: moving down from index 2 (row 0, col 2) lands on
	// 5; moving down again enters the single-entry last row at index 6.
	g := browser.NewGrid(7, 3)
	g.SetFocus(2)

	if !g.Down() || g.Focus() != 5 {
		t.Errorf("Down moved focus to %d, want 5", g.Focus())
	}
	if !g.Down() || g.Focus() != 6 {
		t.Errorf("Down into shorter last row moved focus to %d, want 6", g.Focus())
	}
	if g.Down() {
		t.Error("Down from the last row should be a no-op")
	}
}

func TestGridVerticalPreferredColumn(t *testing.T) {
	t.Parallel()

	// 10 entries, 4 columns. Focus 3 (row 0, col 3), go down twice: land on
	// the nearest valid column of the shorter last row (index 9). Going up
	// must restore the preferred column, not the clamped column.
	g := browser.NewGrid(10, 4)
	g.SetFocus(3)

	if !g.Down() || g.Focus() != 7 {
		t.Errorf("Down moved focus to %d, want 7", g.Focus())
	}
	if !g.Down() || g.Focus() != 9 {
		t.Errorf("Down into shorter last row moved focus to %d, want 9", g.Focus())
	}
	if !g.Up() || g.Focus() != 7 {
		t.Errorf("Up restored focus %d, want 7 (preferred column preserved)", g.Focus())
	}
	if !g.Up() || g.Focus() != 3 {
		t.Errorf("Up restored focus %d, want 3", g.Focus())
	}
	if g.Up() {
		t.Error("Up from the first row should be a no-op")
	}
}

func TestGridPreferredColumnUpdatesOnlyHorizontally(t *testing.T) {
	t.Parallel()

	// 12 entries, 3 columns. Move right from 0 to 1, then down twice; the
	// preferred column is 1, so vertical motion must stay in column 1
	// (indices 4, 7).
	g := browser.NewGrid(12, 3)
	if !g.Right() || g.Focus() != 1 {
		t.Fatalf("Right moved focus to %d, want 1", g.Focus())
	}
	if !g.Down() || g.Focus() != 4 {
		t.Errorf("Down moved focus to %d, want 4", g.Focus())
	}
	if !g.Down() || g.Focus() != 7 {
		t.Errorf("Down moved focus to %d, want 7", g.Focus())
	}
	if !g.Up() || g.Focus() != 4 {
		t.Errorf("Up moved focus to %d, want 4", g.Focus())
	}
}

func TestGridPageDownClampsToLastRow(t *testing.T) {
	t.Parallel()

	// 21 entries, 3 columns (7 rows). Focus 7 (row 2, col 1), page down by 3
	// rows lands on row 5, col 1 = index 16.
	g := browser.NewGrid(21, 3)
	g.SetFocus(7)

	if !g.PageDown(3) || g.Focus() != 16 {
		t.Errorf("PageDown(3) moved focus to %d, want 16", g.Focus())
	}
	if !g.PageDown(3) || g.Focus() != 19 {
		t.Errorf("PageDown(3) clamped focus to %d, want 19", g.Focus())
	}
	if g.PageDown(3) {
		t.Error("PageDown from the last row should be a no-op")
	}
}

func TestGridPageUpClampsToFirstRow(t *testing.T) {
	t.Parallel()

	// 21 entries, 3 columns. Focus 19 (row 6, col 1), page up by 3 rows
	// lands on row 3, col 1 = index 10, then row 0, col 1 = index 1.
	g := browser.NewGrid(21, 3)
	g.SetFocus(19)

	if !g.PageUp(3) || g.Focus() != 10 {
		t.Errorf("PageUp(3) moved focus to %d, want 10", g.Focus())
	}
	if !g.PageUp(3) || g.Focus() != 1 {
		t.Errorf("PageUp(3) moved focus to %d, want 1", g.Focus())
	}
	if g.PageUp(3) {
		t.Error("PageUp from the first row should be a no-op")
	}
}

func TestGridPageSizeOfZeroMeansOneRow(t *testing.T) {
	t.Parallel()

	g := browser.NewGrid(21, 3)
	if !g.PageDown(0) || g.Focus() != 3 {
		t.Errorf("PageDown(0) moved focus to %d, want 3", g.Focus())
	}
}

func TestGridHomeEnd(t *testing.T) {
	t.Parallel()

	g := browser.NewGrid(7, 3)
	if g.Home() {
		t.Error("Home on the first entry should be a no-op")
	}
	if !g.End() || g.Focus() != 6 {
		t.Errorf("End moved focus to %d, want 6", g.Focus())
	}
	if !g.Home() || g.Focus() != 0 {
		t.Errorf("Home moved focus to %d, want 0", g.Focus())
	}
}

func TestGridEmptyMovementIsNoOp(t *testing.T) {
	t.Parallel()

	g := browser.NewGrid(0, 3)
	for name, move := range map[string]func() bool{
		"Left": g.Left, "Right": g.Right, "Up": g.Up, "Down": g.Down,
		"Home": g.Home, "End": g.End,
	} {
		if move() {
			t.Errorf("%s on an empty grid should be a no-op", name)
		}
	}
	if g.PageDown(1) || g.PageUp(1) {
		t.Error("Paging on an empty grid should be a no-op")
	}
}

func TestGridResizePreservesFocusedEntry(t *testing.T) {
	t.Parallel()

	// Focused entry 7 (row 2, col 2 of 4 columns) keeps its identity after
	// switching to 3 columns (row 2, col 1).
	g := browser.NewGrid(10, 4)
	g.SetFocus(7)

	g.SetColumns(3)
	if g.Focus() != 7 {
		t.Errorf("SetColumns moved focus to %d, want 7 (identity preserved)", g.Focus())
	}
	if g.Columns() != 3 {
		t.Errorf("Columns() = %d, want 3", g.Columns())
	}

	g.SetCount(5)
	if g.Focus() != 4 {
		t.Errorf("SetCount clamped focus to %d, want 4", g.Focus())
	}
}
