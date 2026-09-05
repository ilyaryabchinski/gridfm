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
// zero-based as Bubble Tea reports them. Motion and release events are
// ignored: hovering does nothing, so the interface stays keyboard-first.
func (m *Model) applyMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.mouseOn {
		// The program only enables capture when configured, but a stale
		// event after a config change must stay inert.
		return m, nil
	}

	ev := tea.MouseEvent(msg)

	if ev.Action == tea.MouseActionPress {
		switch ev.Button {
		case tea.MouseButtonWheelUp:
			return m.wheel(ev, -1)
		case tea.MouseButtonWheelDown:
			return m.wheel(ev, 1)
		case tea.MouseButtonLeft:
			return m.mouseLeftClick(ev)
		}
	}

	return m, nil
}

// wheel scrolls the region under the pointer: the grid viewport. Panels
// without scroll state — sidebar, inspector — and events outside the
// grid do nothing; a pointer in a gap between cards still scrolls the
// grid, because the gap belongs to the grid region.
func (m *Model) wheel(ev tea.MouseEvent, dir int) (tea.Model, tea.Cmd) {
	if m.regionAt(m.layout(), ev.X, ev.Y) != RegionGrid {
		return m, nil
	}

	m.scrollRow += dir
	m.clampScrollBounds()

	return m, nil
}

// regionAt reports which region owns a coordinate without requiring a
// hit on an item: gaps between cards still belong to the grid. Chrome
// (header, status bar, operation shelf), blocking overlays, and states
// without a real grid own nothing — none of them may scroll the grid
// from underneath.
func (m *Model) regionAt(l ui.Layout, x, y int) Region {
	if !l.Usable || m.loading || m.loadErr != nil || m.overlaysOpen() {
		return RegionNone
	}

	// Floating panels own their rectangles, like in hitTest.
	if m.sidebarOn && !l.SidebarVisible {
		if x < min(ui.SidebarWidth, l.ContentWidth) {
			return RegionSidebar
		}
	}
	if m.inspectorOn && l.InspectorWidth == 0 {
		w := min(ui.InspectorWidth, l.ContentWidth)
		if x >= l.ContentWidth-w {
			return RegionInspector
		}
	}

	// Row 0 is the breadcrumbs header; the operation shelf and status
	// bar own everything below the grid body.
	bodyRows := l.ContentHeight
	if m.opProgress != nil {
		bodyRows--
	}
	if y < 1 || y-1 >= bodyRows {
		return RegionNone
	}

	if l.SidebarVisible && x < l.SidebarWidth {
		return RegionSidebar
	}
	if l.InspectorWidth > 0 && x >= l.ContentWidth-l.InspectorWidth {
		return RegionInspector
	}

	gridX := 0
	if l.SidebarVisible {
		gridX = l.SidebarWidth
	}
	if x >= gridX {
		return RegionGrid
	}

	return RegionNone
}

// mouseLeftClick focuses whatever sits under the pointer; ctrl-click
// toggles the entry's selection instead; a second click on the same
// entry inside the double-click window opens it. Focus changes follow
// the keyboard contract: an open inspector clears and reloads for the
// newly focused entry.
func (m *Model) mouseLeftClick(ev tea.MouseEvent) (tea.Model, tea.Cmd) {
	l := m.layout()

	if ev.Ctrl {
		region, index, ok := m.hitTest(l, ev.X, ev.Y)
		if !ok || region != RegionGrid {
			return m, nil
		}

		m.region = RegionGrid
		m.browser.SetFocusIndex(index)
		m.browser.ToggleFocused()
		m.clampScroll()

		if m.inspectorOn {
			return m, m.requestInspector()
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
		m.clampScroll()

		now := time.Now()
		double := now.Sub(m.lastClickAt) <= doubleClickWindow &&
			m.lastClickX == ev.X && m.lastClickY == ev.Y
		m.lastClickAt, m.lastClickX, m.lastClickY = now, ev.X, ev.Y
		if double {
			return m.openFocused()
		}

		if m.inspectorOn {
			return m, m.requestInspector()
		}
	}

	return m, nil
}

// clampScrollBounds keeps the scroll window inside the content without
// pulling it back to the focused row — the wheel scrolls freely, unlike
// keyboard movement which always keeps focus in view.
func (m *Model) clampScrollBounds() {
	l := m.layout()
	rows := (len(m.browser.Visible()) + l.Columns - 1) / l.Columns
	maxOffset := max(rows-l.RowsVisible, 0)
	m.scrollRow = min(max(m.scrollRow, 0), maxOffset)
}

// hitTest maps terminal coordinates to a focused region and item index.
// Floating sidebar and inspector panels occlude the grid: their cells
// belong to the visible panel, never to the hidden cards beneath.
func (m *Model) hitTest(l ui.Layout, x, y int) (Region, int, bool) {
	if !l.Usable || m.loading || m.loadErr != nil || m.overlaysOpen() {
		return RegionGrid, 0, false
	}

	// Floating panels draw over the grid in narrow mode; their
	// rectangles swallow clicks instead of poking through.
	if m.sidebarOn && !l.SidebarVisible {
		if x < min(ui.SidebarWidth, l.ContentWidth) {
			return RegionGrid, 0, false
		}
	}
	if m.inspectorOn && l.InspectorWidth == 0 {
		w := min(ui.InspectorWidth, l.ContentWidth)
		if x >= l.ContentWidth-w {
			return RegionGrid, 0, false
		}
	}

	if l.SidebarVisible && x < l.SidebarWidth {
		if idx, ok := m.sidebarItemAt(l, y); ok {
			return RegionSidebar, idx, true
		}

		return RegionGrid, 0, false
	}

	gridX := 0
	if l.SidebarVisible {
		gridX = l.SidebarWidth
	}

	// The header occupies row 0; cards tile below it with fixed gaps.
	if x < gridX || y < 1 {
		return RegionGrid, 0, false
	}

	row := (y - 1) / (l.Card.Height + ui.CardGapY)
	col := (x - gridX) / (l.Card.Width + ui.CardGapX)
	if row < 0 || col < 0 || row >= l.RowsVisible || col >= l.Columns {
		return RegionGrid, 0, false
	}

	// Reject clicks in the gaps between cards.
	withinY := (y-1)%(l.Card.Height+ui.CardGapY) < l.Card.Height
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
// header before each section's items. Content starts at row 1, below
// the header row.
func (m *Model) sidebarItemAt(l ui.Layout, y int) (int, bool) {
	items := m.sidebarItems()
	if len(items) == 0 {
		return 0, false
	}

	line := y - 1
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
