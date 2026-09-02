package app

import (
	"context"
	"os/exec"
	"path/filepath"

	"gridfm/internal/browser"
	"gridfm/internal/open"
	"gridfm/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// sortModes is the order the sort menu presents modes in.
//
//nolint:gochecknoglobals // static presentation order, never mutated
var sortModes = []browser.SortMode{
	browser.SortByName,
	browser.SortBySize,
	browser.SortByModified,
	browser.SortByType,
}

// Key identifiers as reported by Bubble Tea's KeyMsg.String().
//
//nolint:gochecknoglobals // stable vocabulary shared by every key handler
var (
	keyUp        = "up"
	keyDown      = "down"
	keyLeft      = "left"
	keyRight     = "right"
	keyHome      = "home"
	keyEnd       = "end"
	keyPgUp      = "pgup"
	keyPgDown    = "pgdown"
	keyEnter     = "enter"
	keyEsc       = "esc"
	keyTab       = "tab"
	keyBackspace = "backspace"
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
		return m.applyEntryNotDirectory(msg)

	case PlacesLoadedMsg:
		m.places = msg.Places
		m.home = msg.Home
		m.placeIdx = 0

		return m, nil

	case OpenFinishedMsg:
		if msg.Err != nil {
			m.note = "editor exited: " + msg.Err.Error()
		}

		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}

	return m, nil
}

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

// applyEntryNotDirectory completes an open attempt on a file: the request
// ends, the loaded directory stays, and the outcome shows as a note.
func (m *Model) applyEntryNotDirectory(msg EntryNotDirectoryMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.requestID {
		return m, nil // stale result from a superseded request
	}

	m.loading = false
	m.note = "not a directory: " + filepath.Base(msg.Path)

	return m, nil
}

// syncGridColumns keeps the spatial grid aligned with the responsive layout
// so vertical movement matches what is actually rendered.
func (m *Model) syncGridColumns() {
	m.browser.SetColumns(m.layout().Columns)
}

// handleKey applies Milestone 1 keybindings, routed by the active surface:
// the sort menu overlay, the filter input, or normal browsing.
func (m *Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.sortOpen:
		return m.handleSortKeys(key)
	case m.filterInput:
		return m.handleFilterKeys(key)
	}

	return m.handleNormalKeys(key)
}

// handleSortKeys drives the sort menu overlay.
func (m *Model) handleSortKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyEsc, "s", "q":
		m.sortOpen = false

		return m, nil
	case keyUp, "k":
		m.sortCursor = max(m.sortCursor-1, 0)
	case keyDown, "j":
		m.sortCursor = min(m.sortCursor+1, len(sortModes)-1)
	case keyEnter:
		mode := sortModes[m.sortCursor]
		order := browser.SortAscending
		if m.browser.SortMode() == mode {
			// Selecting the active mode flips its direction.
			order = m.browser.SortOrder().Opposite()
		}
		m.browser.SetSort(mode, order)
		m.sortOpen = false
	case "o":
		m.browser.SetSort(m.browser.SortMode(), m.browser.SortOrder().Opposite())
	}

	return m, nil
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

		return m, nil
	case keyEnter:
		m.filterInput = false

		return m, nil
	case keyBackspace:
		query := m.browser.Filter()
		if query != "" {
			runes := []rune(query)
			m.browser.SetFilter(string(runes[:len(runes)-1]))
		}

		return m, nil
	case "ctrl+u":
		m.browser.SetFilter("")

		return m, nil
	case keyUp, keyDown, keyLeft, keyRight, keyPgUp, keyPgDown, keyHome, keyEnd, keyTab:
		// Navigation keys are swallowed while typing; the filter keeps
		// focus where the user left it.
		return m, nil
	}

	if runes := []rune(key); len(runes) == 1 && runes[0] >= 0x20 && runes[0] != 0x7f {
		m.browser.SetFilter(m.browser.Filter() + key)
	}

	return m, nil
}

// handleNormalKeys applies browsing keys outside overlays and inputs: it
// owns quit, focus region switching, and transient-state clearing, then
// delegates to the display and entry handlers.
func (m *Model) handleNormalKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case keyTab:
		return m.switchRegion(), nil
	case keyEsc:
		m.escalateClear()

		return m, nil
	}

	return m.handleDisplayKeys(key)
}

// handleDisplayKeys owns interface visibility and presentation: sidebar
// toggle, sort menu, hidden files, zoom, and the filter input.
func (m *Model) handleDisplayKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "~":
		m.sidebarOn = !m.sidebarOn
		if !m.sidebarEffective() {
			m.region = RegionGrid
		}

		return m, nil
	case "s":
		m.sortOpen = true
		m.sortCursor = int(m.browser.SortMode())

		return m, nil
	case ".":
		m.browser.SetShowHidden(!m.browser.ShowHidden())

		return m, nil
	case "+", "=":
		return m.stepZoom(m.zoom.ZoomIn())
	case "-", "_":
		return m.stepZoom(m.zoom.ZoomOut())
	case "/":
		m.region = RegionGrid
		m.filterInput = true

		return m, nil
	}

	return m.handleEntryKeys(key)
}

// stepZoom applies a zoom change and reflows the grid geometry.
func (m *Model) stepZoom(next ui.ZoomLevel) (tea.Model, tea.Cmd) {
	m.zoom = next
	m.syncGridColumns()
	m.clampScroll()

	return m, nil
}

// handleEntryKeys owns entry-level actions: entering, opening, parent
// navigation, and location history.
func (m *Model) handleEntryKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyEnter:
		return m.handleEnter()
	case keyBackspace:
		return m.goToParent()
	case "alt+left":
		return m.historyStep(m.browser.Back)
	case "alt+right":
		return m.historyStep(m.browser.Forward)
	case "o":
		return m.openFocused()
	}

	return m.handleMovement(key)
}

// escalateClear clears transient state one layer at a time.
func (m *Model) escalateClear() {
	switch {
	case m.note != "":
		m.note = ""
	case m.browser.Filter() != "":
		m.browser.SetFilter("")
		m.filterInput = false
	}
}

// switchRegion toggles grid and sidebar focus when the sidebar is visible
// in either docked or overlay form.
func (m *Model) switchRegion() *Model {
	if m.sidebarEffective() {
		if m.region == RegionGrid {
			m.region = RegionSidebar
		} else {
			m.region = RegionGrid
		}
	}

	return m
}

// sidebarEffective reports whether the sidebar renders at all in the
// current terminal shape.
func (m *Model) sidebarEffective() bool {
	return m.sidebarOn && m.width >= 1
}

// handleEnter dispatches enter per region: places in the sidebar, entries
// in the grid.
func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.region == RegionSidebar {
		place, ok := m.SelectedPlace()
		if !ok {
			return m, nil
		}

		return m, loadDirectoryCmd(m.startRequest(), place.Path)
	}

	return m.openFocused()
}

// historyStep moves through location history and issues the load.
func (m *Model) historyStep(step func() (string, bool)) (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}

	target, ok := step()
	if !ok {
		return m, nil
	}

	return m, loadDirectoryCmd(m.startRequest(), target)
}

// openFocused enters directories and opens files. Directories and symlinks
// resolve through the filesystem (a symlink may point at a directory);
// regular text files open in the editor with terminal hand-off, and
// everything else goes to the desktop opener.
func (m *Model) openFocused() (tea.Model, tea.Cmd) {
	entry, ok := m.browser.Focused()
	if !ok {
		return m, nil
	}
	if entry.IsDir || entry.Symlink {
		return m, openEntryCmd(m.startRequest(), entry.Path)
	}

	return m, m.openFile(entry.Path)
}

// openFile builds the command for opening a single file.
func (m *Model) openFile(path string) tea.Cmd {
	switch open.TargetFor(path) {
	case open.TargetDesktop:
		err := open.DesktopOpen(path)
		if err != nil {
			m.note = "open failed: " + err.Error()

			return nil
		}
		m.note = "opened " + filepath.Base(path)

		return nil
	case open.TargetEditor:
		program, args, ok := open.EditorCommand()
		if !ok {
			m.note = "no editor configured (set $VISUAL or $EDITOR)"

			return nil
		}

		// ExecProcess suspends the program, runs the editor on the real
		// terminal, and restores the TUI afterwards.
		//nolint:gosec // program and arguments come from trusted env config; no shell involved
		editor := exec.CommandContext(context.Background(), program, append(args, path)...)
		m.note = ""

		return tea.ExecProcess(editor, func(err error) tea.Msg {
			return OpenFinishedMsg{Err: err}
		})
	}

	return nil
}

// handleMovement applies spatial navigation keys for the focused region.
func (m *Model) handleMovement(key string) (tea.Model, tea.Cmd) {
	if m.region == RegionSidebar {
		return m.movePlaceCursor(key)
	}

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

// movePlaceCursor moves the sidebar selection.
func (m *Model) movePlaceCursor(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyUp, "k":
		m.placeIdx = max(m.placeIdx-1, 0)
	case keyDown, "j":
		m.placeIdx = min(m.placeIdx+1, len(m.places)-1)
	case keyHome:
		m.placeIdx = 0
	case keyEnd:
		m.placeIdx = max(len(m.places)-1, 0)
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
	switch key {
	case keyLeft, "h":
		left := m.browser.Left()

		return left, true
	case keyRight, "l":
		return m.browser.Right(), false
	case keyUp, "k":
		return m.browser.Up(), false
	case keyDown, "j":
		return m.browser.Down(), false
	case keyPgUp:
		return m.browser.PageUp(m.rowsVisible()), false
	case keyPgDown:
		return m.browser.PageDown(m.rowsVisible()), false
	case keyHome:
		return m.browser.Home(), false
	case keyEnd:
		return m.browser.End(), false
	}

	return false, false
}

// goToParent navigates to the parent directory. At the filesystem root it
// is a no-op.
func (m *Model) goToParent() (tea.Model, tea.Cmd) {
	if !m.hasParent() || m.region != RegionGrid {
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
	return m.layout().RowsVisible
}

// clampScroll keeps the focused row inside the visible window.
func (m *Model) clampScroll() {
	m.scrollRow = ui.ScrollOffset(
		m.scrollRow,
		m.browser.FocusedRow(),
		m.rowsVisible(),
	)
}
