package app

import (
	"path/filepath"

	"gridfm/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Update routes messages to handlers. It never performs filesystem I/O.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncGridColumns()
		m.clampScroll()

		return m, nil

	case DirectoryLoadedMsg:
		m.applyDirectoryLoaded(msg)

		return m, nil

	case EntryNotDirectoryMsg:
		if msg.RequestID != m.requestID {
			return m, nil // stale result from a superseded request
		}

		// Completing the open attempt ends its request; the note surfaces
		// the outcome without disturbing the loaded directory.
		m.loading = false
		m.note = "not a directory: " + filepath.Base(msg.Path)

		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}

	return m, nil
}

// applyDirectoryLoaded applies a directory read result, discarding it when
// the request was superseded or the read failed.
func (m *Model) applyDirectoryLoaded(msg DirectoryLoadedMsg) {
	if msg.RequestID != m.requestID {
		return // stale result from a superseded request
	}

	m.loading = false

	if msg.Err != nil {
		m.loadErr = msg.Err
		// Old entries stay visible; the failure is surfaced as a status
		// note so failed navigation is never silent.
		m.note = "error: " + msg.Err.Error()

		return
	}

	m.loadErr = nil
	m.note = ""
	m.browser.SetEntries(msg.Path, msg.Entries)
	m.syncGridColumns()
	m.clampScroll()
}

// syncGridColumns keeps the spatial grid aligned with the responsive layout
// so vertical movement matches what is actually rendered.
func (m *Model) syncGridColumns() {
	m.browser.Nav().SetColumns(m.layout().Columns)
}

// handleKey applies Milestone 0 keybindings. Decisions settled for the
// prototype: h/l and arrows always move cards without wrapping rows;
// backspace goes to the parent; enter opens the focused entry; h pressed at
// the left edge also goes to the parent.
func (m *Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		return m.openFocused()
	case "backspace":
		return m.goToParent()
	}

	return m.handleMovement(key)
}

// handleMovement applies spatial navigation keys and keeps the focused row
// visible. A blocked h at the left edge acts as the parent-directory
// shortcut.
func (m *Model) handleMovement(key string) (tea.Model, tea.Cmd) {
	moved, parentShortcut := m.moveFocus(key)
	if moved {
		m.clampScroll()

		return m, nil
	}
	if parentShortcut {
		return m.goToParent()
	}

	return m, nil
}

// moveFocus applies the movement bound to key, mutating the browser in
// place, and reports whether focus changed. The second result reports
// whether the key doubles as the parent-directory shortcut when blocked.
//
// It must use a pointer receiver: with a value receiver the movement would
// land in a copy of the model and silently vanish.
func (m *Model) moveFocus(key string) (bool, bool) {
	nav := m.browser.Nav()

	switch key {
	case "left", "h":
		left := nav.Left()

		return left, true
	case "right", "l":
		return nav.Right(), false
	case "up", "k":
		return nav.Up(), false
	case "down", "j":
		return nav.Down(), false
	case "pgup":
		return nav.PageUp(m.rowsVisible()), false
	case "pgdown":
		return nav.PageDown(m.rowsVisible()), false
	case "home":
		return nav.Home(), false
	case "end":
		return nav.End(), false
	}

	return false, false
}

// openFocused enters the focused entry when it resolves to a directory.
func (m *Model) openFocused() (tea.Model, tea.Cmd) {
	entry, ok := m.browser.Focused()
	if !ok {
		return m, nil
	}

	return m, openEntryCmd(m.startRequest(), entry.Path)
}

// goToParent navigates to the parent directory. At the filesystem root it
// is a no-op.
func (m *Model) goToParent() (tea.Model, tea.Cmd) {
	if !m.hasParent() {
		return m, nil
	}

	return m, loadDirectoryCmd(m.startRequest(), filepath.Dir(m.browser.Path))
}

// hasParent reports whether the current location has a parent directory.
func (m *Model) hasParent() bool {
	return filepath.Dir(m.browser.Path) != m.browser.Path
}

// rowsVisible returns how many grid rows fit the current terminal size.
func (m *Model) rowsVisible() int {
	return ui.ComputeLayout(m.width, m.height).RowsVisible
}

// clampScroll keeps the focused row inside the visible window.
func (m *Model) clampScroll() {
	m.scrollRow = ui.ScrollOffset(
		m.scrollRow,
		m.browser.Nav().FocusedRow(),
		m.rowsVisible(),
	)
}
