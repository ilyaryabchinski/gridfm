package app

import (
	"time"

	"gridfm/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// doubleClickWindow is the maximum gap between two presses that counts
// as a double click.
const doubleClickWindow = 500 * time.Millisecond

// lastClick remembers the previous left press for double-click detection.
type lastClick struct {
	at time.Time
	x  int
	y  int
}

// applyMouse routes one mouse event. Coordinates are terminal cells,
// 1-based as reported by SGR mouse mode. Motion and release events are
// ignored: hovering does nothing, so the interface stays keyboard-first.
func (m *Model) applyMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	ev := tea.MouseEvent(msg)

	if ev.Action == tea.MouseActionPress {
		switch ev.Button {
		case tea.MouseButtonWheelUp:
			m.scrollRow--
			m.clampScroll()

			return m, nil
		case tea.MouseButtonWheelDown:
			m.scrollRow++
			m.clampScroll()

			return m, nil
		case tea.MouseButtonLeft:
			return m.mouseLeftClick(ev)
		}
	}

	return m, nil
}

// mouseLeftClick focuses whatever sits under the pointer; ctrl-click
// toggles the entry's selection instead; a second click on the same
// entry inside the double-click window opens it.
func (m *Model) mouseLeftClick(ev tea.MouseEvent) (tea.Model, tea.Cmd) {
	l := m.layout()

	if ev.Ctrl {
		if region, index, ok := m.hitTest(l, ev.X, ev.Y); ok && region == RegionGrid {
			m.browser.SetFocusIndex(index)
			m.browser.ToggleFocused()
		}

		return m, nil
	}

	region, index, ok := m.hitTest(l, ev.X, ev.Y)
	if !ok {
		return m, nil
	}

	switch region {
	case RegionSidebar:
		m.region = RegionSidebar
		m.placeIdx = index

		return m, nil
	case RegionGrid:
		m.region = RegionGrid
		m.browser.SetFocusIndex(index)

		now := time.Now()
		double := now.Sub(m.lastClickAt) <= doubleClickWindow &&
			m.lastClickX == ev.X && m.lastClickY == ev.Y
		m.lastClickAt, m.lastClickX, m.lastClickY = now, ev.X, ev.Y
		if double {
			return m.openFocused()
		}
	}

	return m, nil
}

// hitTest maps terminal coordinates to a focused region and item index.
// The floating sidebar and inspector are not clickable: their cells
// belong to the grid beneath them, and keyboard navigation already
// covers them.
func (m *Model) hitTest(l ui.Layout, x, y int) (Region, int, bool) {
	if !l.Usable || m.loading || m.loadErr != nil || m.overlaysOpen() {
		return RegionGrid, 0, false
	}

	if l.SidebarVisible && x >= 1 && x <= l.SidebarWidth {
		if idx, ok := m.sidebarItemAt(l, y); ok {
			return RegionSidebar, idx, true
		}

		return RegionGrid, 0, false
	}

	gridX := 1
	if l.SidebarVisible {
		gridX += l.SidebarWidth
	}

	// Content starts below the header row; cards tile with fixed gaps.
	row := (y - 2) / (l.Card.Height + ui.CardGapY)
	col := (x - gridX) / (l.Card.Width + ui.CardGapX)
	if row < 0 || col < 0 || row >= l.RowsVisible || col >= l.Columns {
		return RegionGrid, 0, false
	}

	// Reject clicks in the gaps between cards.
	withinY := (y-2)%(l.Card.Height+ui.CardGapY) < l.Card.Height
	withinX := (x-gridX)%(l.Card.Width+ui.CardGapX) < l.Card.Width
	if !withinY || !withinX {
		return RegionGrid, 0, false
	}

	index := (m.scrollRow+row)*l.Columns + col
	entries := m.browser.Visible()
	if index >= len(entries) {
		return RegionGrid, 0, false
	}

	return RegionGrid, index, true
}

// sidebarItemAt maps a y coordinate inside the docked sidebar to an item
// index, mirroring RenderSidebar's line layout: one blank line plus a
// header before each section's items.
func (m *Model) sidebarItemAt(l ui.Layout, y int) (int, bool) {
	items := m.sidebarItems()
	if len(items) == 0 {
		return 0, false
	}

	// Body content starts below the header row.
	line := y - 2
	if line < 0 {
		return 0, false
	}

	section := ""
	cursor := 0
	for i, item := range items {
		name := item.Section
		if name == "" {
			name = "places"
		}
		if name != section {
			section = name
			cursor += 2 // blank spacer + section header
		}
		if cursor == line {
			return i, true
		}
		cursor++
	}

	return 0, false
}
