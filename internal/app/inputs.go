package app

import (
	"gridfm/internal/browser"

	tea "github.com/charmbracelet/bubbletea"
)

// This file owns the two modal input surfaces: the blocking sort menu
// overlay and the incremental filter input.

// handleSortKeys drives the sort menu overlay. It closes on esc, its own
// key, or the quit key, all honoring remapping.
func (m *Model) handleSortKeys(key string) (tea.Model, tea.Cmd) {
	switch {
	case key == keyEsc || m.binds.action(key) == "sort" || m.binds.action(key) == "quit":
		m.sortOpen = false

		return m, nil
	case key == keyUp || key == "k":
		m.sortCursor = max(m.sortCursor-1, 0)
	case key == keyDown || key == "j":
		m.sortCursor = min(m.sortCursor+1, len(sortModes)-1)
	case key == keyEnter:
		mode := sortModes[m.sortCursor]
		order := browser.SortAscending
		if m.browser.SortMode() == mode {
			// Selecting the active mode flips its direction.
			order = m.browser.SortOrder().Opposite()
		}
		m.browser.SetSort(mode, order)
		m.sortOpen = false
		m.clampScroll()

		return m, m.afterVisibleChange()
	case key == "o":
		// Reverse without leaving the menu; surface-specific, not
		// remapped.
		m.browser.SetSort(m.browser.SortMode(), m.browser.SortOrder().Opposite())
		m.clampScroll()

		return m, m.afterVisibleChange()
	}

	return m, nil
}

// afterVisibleChange re-derives state that tracks the focused entry after
// an operation rebuilt the visible list: when such an operation moves
// focus, the open inspector must follow it.
func (m *Model) afterVisibleChange() tea.Cmd {
	if m.inspectorOn {
		return m.followInspector()
	}

	return nil
}

// handleFilterKeys drives the incremental filter input. Editing updates the
// visible view live; esc abandons the filter entirely. Only single
// printable runes extend the query, so named keys (tab, arrows, f-keys)
// never leak text into it.
func (m *Model) handleFilterKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyEsc:
		m.filterInput = false
		m.browser.SetFilter("")

		return m, m.afterVisibleChange()
	case keyEnter:
		m.filterInput = false

		return m, nil
	case keyBackspace:
		query := m.browser.Filter()
		if query != "" {
			runes := []rune(query)
			m.browser.SetFilter(string(runes[:len(runes)-1]))
			m.clampScroll()
		}

		return m, m.afterVisibleChange()
	case "ctrl+u":
		m.browser.SetFilter("")
		m.clampScroll()

		return m, m.afterVisibleChange()
	case keyUp, keyDown, keyLeft, keyRight, keyPgUp, keyPgDown, keyHome, keyEnd, keyTab:
		// Navigation keys are swallowed while typing; the filter keeps
		// focus where the user left it.
		return m, nil
	}

	if runes := []rune(key); len(runes) == 1 && runes[0] >= 0x20 && runes[0] != 0x7f {
		m.browser.SetFilter(m.browser.Filter() + key)
		m.clampScroll()

		return m, m.afterVisibleChange()
	}

	return m, nil
}

// escalateClear clears transient state one layer at a time: notes, then
// the filter, then select mode, then the selection itself.
func (m *Model) escalateClear() {
	switch {
	case m.note != "":
		m.note = ""
	case m.browser.Filter() != "":
		m.browser.SetFilter("")
		m.filterInput = false
	case m.mode == ModeSelect:
		m.mode = ModeBrowse
	case m.browser.SelectedCount() > 0:
		m.browser.ClearSelection()
	}
}
