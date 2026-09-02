package browser

// Grid is a pure spatial-navigation model over a fixed list of entries laid
// out in row-major order: index = row*columns + column. It knows nothing
// about rendering or the filesystem.
//
// Vertical movement tracks a preferred column so repeated up/down motion
// stays stable across incomplete rows; only horizontal movement updates it.
type Grid struct {
	columns int
	count   int
	focus   int
	prefCol int
}

// NewGrid returns a grid for count entries at the given column count, with
// focus on the first entry.
func NewGrid(count, columns int) Grid {
	if columns < 1 {
		columns = 1
	}
	if count < 0 {
		count = 0
	}

	return Grid{columns: columns, count: count, focus: 0, prefCol: 0}
}

// Columns returns the current column count.
func (g *Grid) Columns() int { return g.columns }

// Count returns the number of entries in the grid.
func (g *Grid) Count() int { return g.count }

// Focus returns the focused index, or 0 for an empty grid.
func (g *Grid) Focus() int { return g.focus }

// FocusedColumn returns the column of the focused entry.
func (g *Grid) FocusedColumn() int {
	if g.count == 0 {
		return 0
	}

	return g.focus % g.columns
}

// FocusedRow returns the row of the focused entry.
func (g *Grid) FocusedRow() int {
	if g.count == 0 {
		return 0
	}

	return g.focus / g.columns
}

// Rows returns the number of rows needed to display all entries.
func (g *Grid) Rows() int {
	if g.count == 0 {
		return 0
	}

	return g.count/g.columns + min(g.count%g.columns, 1)
}

// SetColumns resizes the grid. The focused entry is preserved by identity:
// the list is unchanged, so the focused index stays, and the preferred
// column is re-derived from the focused entry's new visual column so
// vertical movement continues from where focus actually sits.
func (g *Grid) SetColumns(columns int) {
	if columns < 1 {
		columns = 1
	}
	g.columns = columns
	g.clamp()
	g.prefCol = g.FocusedColumn()
}

// SetCount adjusts the entry count, clamping focus to the nearest surviving
// index when the list shrinks.
func (g *Grid) SetCount(count int) {
	if count < 0 {
		count = 0
	}
	g.count = count
	g.clamp()
}

// SetFocus moves focus to a specific index, clamped to the valid range.
func (g *Grid) SetFocus(index int) {
	if index < 0 {
		index = 0
	}
	if index >= g.count {
		index = max(g.count-1, 0)
	}
	g.focus = index
	g.prefCol = g.FocusedColumn()
}

// Left moves focus one entry left without wrapping across rows. It reports
// whether focus changed.
func (g *Grid) Left() bool {
	if g.count == 0 || g.focus%g.columns == 0 {
		return false
	}
	g.focus--
	g.prefCol = g.FocusedColumn()

	return true
}

// Right moves focus one entry right without wrapping across rows. It reports
// whether focus changed.
func (g *Grid) Right() bool {
	next := g.focus + 1
	if next >= g.count || next%g.columns == 0 {
		return false
	}
	g.focus = next
	g.prefCol = g.FocusedColumn()

	return true
}

// Up moves focus to the preferred column of the row above. It reports
// whether focus changed.
func (g *Grid) Up() bool {
	row := g.FocusedRow()
	if row == 0 {
		return false
	}
	g.focus = (row-1)*g.columns + g.prefCol

	return true
}

// Down moves focus toward the preferred column of the row below. When the
// row below is the shorter last row, focus lands on the nearest valid
// column. It reports whether focus changed.
func (g *Grid) Down() bool {
	row := g.FocusedRow()
	if row >= g.lastRow() {
		return false
	}
	target := (row+1)*g.columns + g.prefCol
	if target >= g.count {
		target = g.count - 1
	}
	g.focus = target

	return true
}

// PageDown moves focus down by the given number of rows, preserving the
// preferred column. It reports whether focus changed.
func (g *Grid) PageDown(rows int) bool {
	row := g.FocusedRow()
	if row >= g.lastRow() {
		return false
	}
	target := min(row+max(rows, 1), g.lastRow())
	g.focus = min(target*g.columns+g.prefCol, g.count-1)

	return true
}

// PageUp moves focus up by the given number of rows, preserving the
// preferred column. It reports whether focus changed.
func (g *Grid) PageUp(rows int) bool {
	row := g.FocusedRow()
	if row == 0 {
		return false
	}
	target := max(row-max(rows, 1), 0)
	g.focus = target*g.columns + g.prefCol

	return true
}

// Home moves focus to the first entry. It reports whether focus changed.
func (g *Grid) Home() bool {
	if g.focus == 0 {
		return false
	}
	g.focus = 0
	g.prefCol = 0

	return true
}

// End moves focus to the last entry. It reports whether focus changed.
func (g *Grid) End() bool {
	if g.count == 0 || g.focus == g.count-1 {
		return false
	}
	g.focus = g.count - 1
	g.prefCol = g.FocusedColumn()

	return true
}

func (g *Grid) lastRow() int {
	if g.count == 0 {
		return 0
	}

	return (g.count - 1) / g.columns
}

func (g *Grid) clamp() {
	if g.count == 0 {
		g.focus = 0
		g.prefCol = 0

		return
	}
	if g.focus >= g.count {
		g.focus = g.count - 1
	}
	if g.focus < 0 {
		g.focus = 0
	}
	g.prefCol = min(g.prefCol, g.columns-1)
}
