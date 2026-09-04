package app

import (
	"path/filepath"
	"strconv"

	"gridfm/internal/browser"
	"gridfm/internal/operations"
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
		save, watch := m.applyDirectoryLoaded(msg)

		// The refreshed listing may have moved focus to a different entry;
		// the inspector follows it when open. The watcher only moves for
		// results that actually applied: a stale or failed load must never
		// re-point it away from the browsed directory.
		cmds := tea.Batch(save, watch)
		if m.inspectorOn {
			return m, tea.Batch(cmds, m.followInspector())
		}

		return m, cmds

	case DirChangedMsg:
		return m.applyDirChanged(msg)

	case WatchReadyMsg:
		m.degradeWatch(msg.Err)

		return m, nil

	case InspectorLoadedMsg:
		return m.applyInspectorLoaded(msg)

	case EntryResolvedMsg:
		return m.applyEntryResolved(msg)

	case PlacesLoadedMsg:
		m.places = msg.Places
		m.bookmarks = msg.Bookmarks
		m.mounts = msg.Mounts
		m.recents = msg.Recents
		m.home = msg.Home
		m.placeIdx = 0

		return m, nil

	case LibrarySavedMsg:
		if msg.Err != nil {
			m.note = "save " + msg.Name + ": " + msg.Err.Error()
		}

		return m, nil

	case OpenFinishedMsg:
		return m.applyOpenFinished(msg)

	case OperationEventMsg:
		return m.applyOperationEvent(msg.Event)

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}

	return m, nil
}

// applyOperationEvent consumes one operation manager event and re-arms the
// listener.
func (m *Model) applyOperationEvent(ev operations.Event) (tea.Model, tea.Cmd) {
	switch e := ev.(type) {
	case operations.ProgressEvent:
		m.opProgress = &opProgress{
			kind:       e.Kind,
			done:       e.Done,
			total:      e.Total,
			target:     e.Target,
			bytes:      e.ItemBytes,
			bytesTotal: e.ItemBytesTotal,
		}

	case operations.QuestionEvent:
		// Only one blocking overlay may be active; the question wins.
		m.sortOpen = false
		m.filterInput = false
		m.input = inputNone
		m.confirm = confirmNone
		m.showResults = false
		m.question = &pendingQuestion{answerCh: e.AnswerCh, target: e.Target}

	case operations.FinishedEvent:
		m.opProgress = nil
		m.question = nil
		m.applyAll = false
		// Only one blocking overlay may be armed: the results overlay
		// claims the keyboard, so any open input, confirmation, sort menu,
		// or filter closes with the finished job.
		m.input = inputNone
		m.confirm = confirmNone
		m.sortOpen = false
		m.filterInput = false
		result := e.Result
		m.lastResult = &result
		m.rememberJob(result)
		m.showResults = len(result.Failures) > 0
		var refresh tea.Cmd
		if !m.loading {
			// The mutation may have changed the browsed directory; refresh
			// unless a navigation is already fetching fresher state. The
			// listener is re-armed alongside the refresh: dropping it here
			// would stall the serial manager on its next publication.
			refresh = loadDirectoryCmd(m.startRequest(), m.browser.Path)
		}
		// The summary is set after startRequest, which resets transient
		// state: otherwise a successful job's summary is cleared before it
		// is ever shown.
		m.note = resultSummary(result)
		if refresh != nil {
			return m, tea.Batch(refresh, listenOperations(m.ops))
		}
	}

	return m, listenOperations(m.ops)
}

// resultSummary renders the accurate outcome of a finished operation.
func resultSummary(r operations.Result) string {
	summary := r.Kind.String() + ": " +
		itoaPlural(r.Succeeded, "ok") + ", " +
		itoaPlural(r.Skipped, "skipped") + ", " +
		itoaPlural(r.Failed, "failed")
	if r.Cancelled {
		summary = "cancelled " + summary
	}

	return summary
}

func itoaPlural(n int, word string) string {
	plain := strconv.Itoa(n) + " " + word
	if n == 1 {
		return plain
	}

	return plain + "s"
}

// applyDirectoryLoaded applies a directory load. It reports the command
// persisting the new recent location and the command re-pointing the
// watcher: the watcher follows only genuinely applied results, and a
// failed load keeps it on the still-current browsed directory. Stale
// results return nil for both.
func (m *Model) applyDirectoryLoaded(msg DirectoryLoadedMsg) (tea.Cmd, tea.Cmd) {
	if msg.RequestID != m.requestID {
		return nil, nil // stale result from a superseded request
	}

	m.loading = false

	if msg.Err != nil {
		m.loadErr = msg.Err
		// A failed back or forward leaves the history cursor on the current
		// location so the traversal can be retried.
		m.browser.CancelHistoryStep()
		// Old entries stay visible; the failure is surfaced as a status
		// note so failed navigation is never silent.
		m.note = "error: " + msg.Err.Error()

		return nil, watchDirCmd(m.watch, m.browser.Path)
	}

	// Recovering from a failed load: its error note must not outlive the
	// retry that fixed it. Any other note — action feedback, open errors,
	// result summaries — is sticky until the next event replaces it or esc
	// clears it, so a refresh can no longer wipe a summary it raced with.
	if m.loadErr != nil {
		m.note = ""
	}
	m.loadErr = nil
	m.browser.SetEntries(msg.Path, msg.Entries)
	m.syncGridColumns()
	m.clampScroll()

	// A change notification dropped while this load was in flight means
	// the snapshot is already stale: schedule one more reload.
	var reschedule tea.Cmd
	if m.watchDirty && m.watchDirtyFor == msg.Path {
		m.watchDirty = false
		m.watchDirtyFor = ""
		reschedule = loadDirectoryCmd(m.startRequest(), m.browser.Path)
	}

	return m.rememberRecent(msg.Path), tea.Batch(watchDirCmd(m.watch, msg.Path), reschedule)
}

// applyEntryResolved completes an open attempt whose target turned out to
// be a file rather than a directory: the resolve request ends and the file
// goes to the opener.
func (m *Model) applyEntryResolved(msg EntryResolvedMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.requestID {
		return m, nil // stale result from a superseded request
	}

	m.loading = false

	return m, m.openFile(msg.Path)
}

// applyOpenFinished reports the outcome of an external open and refreshes
// the current directory, since editors may create, rename, or delete files.
// Stale completions are discarded entirely: refreshing on one would
// invalidate an in-flight navigation and strand a pending history step.
func (m *Model) applyOpenFinished(msg OpenFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.openID {
		return m, nil // stale result from a superseded open
	}

	note := openedLabel(msg.Path)
	if msg.Err != nil {
		note = "open failed: " + msg.Err.Error()
	}

	// A browse request in flight wins; it reloads the directory anyway.
	if m.loading {
		m.note = note

		return m, nil
	}

	refresh := loadDirectoryCmd(m.startRequest(), m.browser.Path)
	m.note = note

	return m, refresh
}

// syncGridColumns keeps the spatial grid aligned with the responsive layout
// so vertical movement matches what is actually rendered.
func (m *Model) syncGridColumns() {
	m.browser.SetColumns(m.layout().Columns)
}

// requestInspector issues an asynchronous metadata load for the focused
// entry. Any current panel content is dropped immediately, so stale data
// never lingers beside a new focus.
func (m *Model) requestInspector() tea.Cmd {
	m.inspectorReq++
	m.inspector = nil
	m.inspectorErr = nil

	entry, ok := m.browser.Focused()
	if !ok {
		m.inspectorPath = ""

		return nil
	}
	m.inspectorPath = entry.Path

	return inspectCmd(m.inspectorReq, entry.Path)
}

// followInspector re-requests the panel when focus has moved to an entry
// the panel does not already show.
func (m *Model) followInspector() tea.Cmd {
	if m.browser.FocusedPath() == m.inspectorPath {
		return nil
	}

	return m.requestInspector()
}

// applyInspectorLoaded applies one inspector load, rejecting results that
// no longer match the newest request.
func (m *Model) applyInspectorLoaded(msg InspectorLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.inspectorReq {
		return m, nil // stale result from a superseded request
	}
	if msg.Err != nil {
		m.inspectorErr = msg.Err

		return m, nil
	}

	m.inspector = msg.Info

	return m, nil
}

// applyDirChanged consumes one debounced watch notification: a change to
// the browsed directory refreshes it, anything else is stale, and a
// failure degrades to manual refresh with a single note.
func (m *Model) applyDirChanged(msg DirChangedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.degradeWatch(msg.Err)

		return m, listenWatch(m.watch)
	}

	// Changes outside the browsed directory are stale.
	if msg.Path != m.browser.Path {
		return m, listenWatch(m.watch)
	}

	// While a load is already in flight the notification cannot refresh
	// anything, but it must not be lost either: the snapshot being fetched
	// predates the change. Latch it and re-refresh once the load lands.
	if m.loading {
		m.watchDirty = true
		m.watchDirtyFor = msg.Path

		return m, listenWatch(m.watch)
	}

	return m, tea.Batch(
		loadDirectoryCmd(m.startRequest(), m.browser.Path),
		listenWatch(m.watch),
	)
}

// handleKey applies Milestone 1 keybindings, routed by the active surface:
// the sort menu overlay, the filter input, the results overlay, or normal
// browsing.
func (m *Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.input != inputNone:
		return m.handleInputOverlayKeys(key)
	case m.question != nil:
		return m.handleQuestionKeys(key)
	case m.confirm != confirmNone:
		return m.handleConfirmKeys(key)
	case m.showResults:
		return m.handleResultsKeys(key)
	case m.sortOpen:
		return m.handleSortKeys(key)
	case m.filterInput:
		return m.handleFilterKeys(key)
	}

	return m.handleNormalKeys(key)
}

// handleResultsKeys drives the blocking results overlay: only its close
// keys act, so hidden grid shortcuts never fire while it is up.
func (m *Model) handleResultsKeys(key string) (tea.Model, tea.Cmd) {
	if key == keyEsc || key == "e" {
		m.showResults = false
	}

	return m, nil
}

// handleNormalKeys applies browsing keys outside overlays and inputs: it
// owns quit, focus region switching, and transient-state clearing, then
// delegates to the display and entry handlers.
func (m *Model) handleNormalKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		// Quitting with active jobs is an explicit decision, so ctrl+c
		// takes the same confirmed path as q.
		return m.requestQuit()
	case "q":
		return m.requestQuit()
	case keyTab:
		return m.switchRegion(), nil
	case keyEsc:
		m.escalateClear()

		return m, nil
	}

	if handler, ok := normalKeyHandlers[key]; ok {
		return handler(m)
	}

	return m.handleDisplayKeys(key)
}

// normalKeyHandlers maps the browsing keys that act on entries and their
// selection: toggling, range mode, staging, mutations, and results.
//
//nolint:gochecknoglobals // static key vocabulary, never mutated
var normalKeyHandlers = map[string]func(*Model) (tea.Model, tea.Cmd){
	" ": func(m *Model) (tea.Model, tea.Cmd) {
		if m.region == RegionGrid {
			m.browser.ToggleFocused()
		}

		return m, nil
	},
	"v": (*Model).toggleSelectMode,
	"ctrl+a": func(m *Model) (tea.Model, tea.Cmd) {
		if m.region == RegionGrid {
			m.browser.SelectAllVisible()
		}

		return m, nil
	},
	"y": func(m *Model) (tea.Model, tea.Cmd) { return m.stageClipboard(ClipboardCopy) },
	"x": func(m *Model) (tea.Model, tea.Cmd) { return m.stageClipboard(ClipboardMove) },
	"b": (*Model).addBookmark,
	"B": (*Model).removeBookmark,
	"p": (*Model).pasteClipboard,
	"n": (*Model).openCreateInput,
	"R": (*Model).openRenameInput,
	"d": func(m *Model) (tea.Model, tea.Cmd) { return m.confirmMutation(confirmTrash) },
	"D": func(m *Model) (tea.Model, tea.Cmd) { return m.confirmMutation(confirmDelete) },
	"c": func(m *Model) (tea.Model, tea.Cmd) {
		m.ops.CancelActive()

		return m, nil
	},
	"e": func(m *Model) (tea.Model, tea.Cmd) {
		if m.lastResult != nil {
			m.showResults = !m.showResults
		}

		return m, nil
	},
}

// requestQuit quits immediately when idle, or asks for confirmation while
// operations are running so quitting is always an explicit decision.
func (m *Model) requestQuit() (tea.Model, tea.Cmd) {
	if !m.ops.Busy() {
		return m, tea.Quit
	}

	m.confirm = confirmQuit
	m.confirmTyped = false
	m.confirmInput = ""
	m.confirmDetail = []string{"operations are still running"}

	return m, nil
}

// toggleSelectMode enters or leaves range-selection mode. Entering pins an
// anchor at the focused entry; movement then extends the selection.
func (m *Model) toggleSelectMode() (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}
	if m.mode == ModeSelect {
		m.mode = ModeBrowse

		return m, nil
	}

	m.mode = ModeSelect
	m.selectAnchor = m.browser.FocusedPath()
	m.browser.ToggleFocused()

	return m, nil
}

// anchorIndex resolves the select-mode anchor to a visible index by entry
// identity, so sorting, filtering, or navigation between the anchor press
// and a movement key cannot detach the range from its entry. When the
// anchored entry is no longer visible, the anchor moves to the focused
// entry so the next movement selects from where the user is looking.
func (m *Model) anchorIndex() int {
	if m.selectAnchor != "" {
		if idx := m.browser.VisibleIndexOf(m.selectAnchor); idx >= 0 {
			return idx
		}
	}

	m.selectAnchor = m.browser.FocusedPath()

	return m.browser.FocusIndex()
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
		m.clampScroll()

		return m, m.afterVisibleChange()
	case "+", "=":
		return m.stepZoom(m.zoom.ZoomIn())
	case "-", "_":
		return m.stepZoom(m.zoom.ZoomOut())
	case "/":
		m.region = RegionGrid
		m.filterInput = true

		return m, nil
	case "i":
		if m.region != RegionGrid {
			return m, nil
		}
		m.inspectorOn = !m.inspectorOn
		if !m.inspectorOn {
			// Closing drops the panel content and invalidates any load in
			// flight.
			m.inspector = nil
			m.inspectorErr = nil
			m.inspectorPath = ""
			m.inspectorReq++

			return m, nil
		}

		return m, m.requestInspector()
	case "r":
		// Manual refresh: the fallback when watching is unavailable and
		// the escape hatch from stale listings in general.
		return m, loadDirectoryCmd(m.startRequest(), m.browser.Path)
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

// handleMovement applies spatial navigation keys for the focused region.
func (m *Model) handleMovement(key string) (tea.Model, tea.Cmd) {
	if m.region == RegionSidebar {
		return m.movePlaceCursor(key)
	}

	moved, parentShortcut := m.moveFocus(key)
	if moved {
		if m.mode == ModeSelect {
			m.browser.SelectRange(m.anchorIndex(), m.browser.FocusIndex())
		}
		m.clampScroll()

		var follow tea.Cmd
		if m.inspectorOn {
			follow = m.followInspector()
		}

		return m, follow
	}
	if parentShortcut {
		return m.goToParent()
	}

	return m, nil
}

// movePlaceCursor moves the sidebar selection across every section.
func (m *Model) movePlaceCursor(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyUp, "k":
		m.placeIdx = max(m.placeIdx-1, 0)
	case keyDown, "j":
		m.placeIdx = min(m.placeIdx+1, len(m.sidebarItems())-1)
	case keyHome:
		m.placeIdx = 0
	case keyEnd:
		m.placeIdx = max(len(m.sidebarItems())-1, 0)
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
