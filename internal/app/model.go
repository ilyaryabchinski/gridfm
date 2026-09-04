package app

import (
	"strconv"
	"strings"

	"gridfm/internal/browser"
	"gridfm/internal/operations"
	"gridfm/internal/places"
	"gridfm/internal/preview"
	"gridfm/internal/ui"
	"gridfm/internal/watcher"

	tea "github.com/charmbracelet/bubbletea"
)

// Region names the half of the interface that owns keyboard focus.
type Region int

// Focused interface regions.
const (
	RegionGrid Region = iota
	RegionSidebar
)

// Mode is the browsing mode: normal navigation or range selection.
type Mode int

// Browsing modes: normal navigation and range-oriented selection.
const (
	ModeBrowse Mode = iota
	ModeSelect
)

// String renders the mode for the status bar.
func (m Mode) String() string {
	if m == ModeSelect {
		return "SELECT"
	}

	return "NORMAL"
}

// ClipboardKind records what a paste will do with the staged paths.
type ClipboardKind int

// Clipboard kinds: what the next paste will do with the staged paths.
const (
	ClipboardNone ClipboardKind = iota
	ClipboardCopy
	ClipboardMove
)

// inputKind identifies the active text input overlay, if any.
type inputKind int

const (
	inputNone inputKind = iota
	inputCreateFile
	inputCreateDir
	inputRename
)

// confirmKind identifies the active confirmation overlay, if any.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmTrash
	confirmDelete
	confirmQuit
)

// Options are the start-up configuration switches.
type Options struct {
	Icons ui.IconMode
}

// pendingQuestion holds an open conflict question from the operation
// manager until the user answers it in the overlay.
type pendingQuestion struct {
	answerCh chan<- operations.Answer
	target   string
}

// opProgress is the display state of the running operation.
type opProgress struct {
	kind   operations.Kind
	done   int
	total  int
	target string
	// bytes and bytesTotal describe the running item's copy progress; both
	// are zero when the item has no measurable size.
	bytes      int64
	bytesTotal int64
}

// Model is the top-level Bubble Tea model. It owns domain state (the
// browser), request identity for stale-result rejection, and transient view
// state. Render functions read it and never mutate it.
type Model struct {
	browser browser.Browser
	places  []places.Place

	// Sidebar library sections beyond the standard places.
	bookmarks []places.Place
	mounts    []places.Place
	recents   []places.Place

	icons     ui.IconMode
	zoom      ui.ZoomLevel
	region    Region
	sidebarOn bool
	placeIdx  int
	home      string

	// Sort menu state: one of the blocking overlays.
	sortOpen   bool
	sortCursor int

	// Filter input state: when active, printable keys extend the query.
	filterInput bool

	// Mutation state.
	mode          Mode
	selectAnchor  string
	clipboard     ClipboardKind
	clipboardPath []string

	// Operation pipeline.
	ops        *operations.Manager
	opProgress *opProgress
	lastResult *operations.Result
	jobLog     []operations.Result

	// Input overlay state.
	input       inputKind
	inputValue  string
	inputTarget string

	// Confirmation overlay state.
	confirm       confirmKind
	confirmDetail []string
	confirmTyped  bool
	confirmInput  string

	// Conflict overlay state.
	question *pendingQuestion
	applyAll bool

	// Results overlay state.
	showResults bool

	// Help overlay state: the keyboard legend.
	helpOpen bool

	// Inspector panel state. The request id rejects stale loads; the panel
	// content clears the moment focus moves elsewhere.
	inspectorOn   bool
	inspectorReq  uint64
	inspectorPath string
	inspector     *preview.Info
	inspectorErr  error

	// Filesystem watching. A nil watcher means watching is unavailable and
	// the app degrades to manual refresh; watchFailed limits the failure
	// note to one appearance. watchDirty latches a change that arrived
	// during an in-flight load, so the snapshot is refreshed once more.
	watch         *watcher.Watcher
	watchFailed   bool
	watchDirty    bool
	watchDirtyFor string

	width     int
	height    int
	requestID uint64
	openID    uint64
	loading   bool
	loadErr   error
	scrollRow int
	note      string
}

// New returns a model that will browse startPath once the first directory
// load completes. All Model methods use pointer receivers so state changes
// can never be lost to a value copy.
func New(startPath string, opts Options) *Model {
	// Watching is an enhancement: without an inotify instance the app runs
	// with manual refresh only.
	w, _ := watcher.New()

	return &Model{
		browser:   browser.New(startPath),
		icons:     opts.Icons,
		zoom:      ui.ZoomNormal,
		sidebarOn: true,
		requestID: 1,
		loading:   true,
		ops:       operations.NewManager(),
		watch:     w,
	}
}

// Init issues the initial directory load and place discovery.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		loadDirectoryCmd(m.requestID, m.browser.Path),
		loadPlacesCmd(),
		listenOperations(m.ops),
		listenWatch(m.watch),
	)
}

// Path returns the browsed directory.
func (m *Model) Path() string { return m.browser.Path }

// Entries returns every loaded entry, hidden ones included.
func (m *Model) Entries() []browser.Entry { return m.browser.Entries }

// Visible returns the entries surviving hidden and filter rules.
func (m *Model) Visible() []browser.Entry { return m.browser.Visible() }

// FocusedPath returns the focused entry path, or an empty string.
func (m *Model) FocusedPath() string { return m.browser.FocusedPath() }

// IsLoading reports whether a directory load is in flight.
func (m *Model) IsLoading() bool { return m.loading }

// LoadError returns the last directory load error, if any.
func (m *Model) LoadError() error { return m.loadErr }

// Zoom returns the current zoom level.
func (m *Model) Zoom() ui.ZoomLevel { return m.zoom }

// SortMode and SortOrder expose the active ordering for tests and status.
func (m *Model) SortMode() browser.SortMode { return m.browser.SortMode() }

// SortOrder returns the active sort direction.
func (m *Model) SortOrder() browser.SortOrder { return m.browser.SortOrder() }

// ShowHidden reports whether hidden entries are visible.
func (m *Model) ShowHidden() bool { return m.browser.ShowHidden() }

// Filter returns the active filter query.
func (m *Model) Filter() string { return m.browser.Filter() }

// FilterInput reports whether the filter input mode is active.
func (m *Model) FilterInput() bool { return m.filterInput }

// Region returns the focused region.
func (m *Model) Region() Region { return m.region }

// PlaceCount returns the number of discovered sidebar places.
func (m *Model) PlaceCount() int { return len(m.places) }

// SelectedPlace returns the sidebar entry under the cursor.
func (m *Model) SelectedPlace() (places.Place, bool) {
	items := m.sidebarItems()
	if m.placeIdx < 0 || m.placeIdx >= len(items) {
		return places.Place{}, false
	}

	item := items[m.placeIdx]

	return places.Place{Label: item.Label, Path: item.Path}, true
}

// Mode returns the active browsing mode.
func (m *Model) Mode() Mode { return m.mode }

// SelectedCount returns the number of selected paths across directories.
func (m *Model) SelectedCount() int { return m.browser.SelectedCount() }

// ClipboardKind returns what a paste would do.
func (m *Model) ClipboardKind() ClipboardKind { return m.clipboard }

// ClipboardPaths returns the staged paths.
func (m *Model) ClipboardPaths() []string { return m.clipboardPath }

// Busy reports whether an operation is queued or running.
func (m *Model) Busy() bool { return m.ops.Busy() }

// EnqueueOperation submits a mutation directly, bypassing the interactive
// flows. It exists for programmatic use and tests; the update loop's key
// handlers are the normal route.
func (m *Model) EnqueueOperation(op operations.Operation) error {
	return m.ops.Enqueue(op)
}

// Inspector accessor methods for tests and status rendering.

// InspectorOn reports whether the inspector panel is toggled open.
func (m *Model) InspectorOn() bool { return m.inspectorOn }

// Inspector returns the panel's current metadata, if loaded.
func (m *Model) Inspector() *preview.Info { return m.inspector }

// InspectorRequestID returns the current inspector request identity.
func (m *Model) InspectorRequestID() uint64 { return m.inspectorReq }

// Bookmarks returns the bookmarked directories.
func (m *Model) Bookmarks() []places.Place { return m.bookmarks }

// Recents returns the recently visited directories, newest first.
func (m *Model) Recents() []places.Place { return m.recents }

// WatchPath returns the currently watched directory, empty when watching
// is unavailable or inactive.
func (m *Model) WatchPath() string {
	if m.watch == nil {
		return ""
	}

	return m.watch.Path()
}

// degradeWatch surfaces a watcher failure once and records the manual
// refresh fallback.
func (m *Model) degradeWatch(err error) {
	if err == nil || m.watchFailed {
		return
	}
	m.watchFailed = true
	m.note = "watch unavailable; r refreshes"
}

// WatchDirectory points the watcher at a directory and returns the
// readiness command. Production flows reach it through Update; it is
// exported for tests and programmatic use.
func (m *Model) WatchDirectory(path string) tea.Cmd {
	return watchDirCmd(m.watch, path)
}

// ListenWatch returns the watch event listener command, nil when watching
// is unavailable.
func (m *Model) ListenWatch() tea.Cmd {
	return listenWatch(m.watch)
}

// Close releases long-lived resources: the filesystem watcher and its
// goroutine. Call it once when the program exits.
func (m *Model) Close() error {
	if m.watch == nil {
		return nil
	}

	return m.watch.Close()
}

// LastResult returns the most recent finished operation result, if any.
func (m *Model) LastResult() *operations.Result { return m.lastResult }

// jobLogDepth bounds the finished-job summaries kept for the sidebar shelf.
const jobLogDepth = 6

// rememberJob records a finished result, newest first, capped so the shelf
// cannot grow without bound across a long session.
func (m *Model) rememberJob(result operations.Result) {
	m.jobLog = append([]operations.Result{result}, m.jobLog...)
	if len(m.jobLog) > jobLogDepth {
		m.jobLog = m.jobLog[:jobLogDepth]
	}
}

// operationLines renders the sidebar operation shelf: a running job line
// plus the recent finished summaries, newest first.
func (m *Model) operationLines() []string {
	lines := make([]string, 0, 1+len(m.jobLog))
	if p := m.opProgress; p != nil {
		line := "• " + p.kind.String() + " " + strconv.Itoa(p.done) + "/" + strconv.Itoa(p.total)
		if p.target != "" {
			line += " " + p.target
		}
		lines = append(lines, line)
	}
	for _, r := range m.jobLog {
		marker := "✓"
		if r.Cancelled {
			marker = "x"
		} else if r.Failed > 0 {
			marker = "✗"
		}
		lines = append(lines, marker+" "+resultSummary(r))
	}

	return lines
}

// recentsDepth bounds the remembered recent locations.
const recentsDepth = 8

// rememberRecent records a successfully loaded directory at the front of
// the recents list, deduplicated, and reports the save command.
func (m *Model) rememberRecent(path string) tea.Cmd {
	updated := make([]places.Place, 0, recentsDepth)
	updated = append(updated, places.Place{Label: recentLabel(path), Path: path})
	for _, p := range m.recents {
		if p.Path == path || len(updated) >= recentsDepth {
			continue
		}
		updated = append(updated, p)
	}
	m.recents = updated

	return saveLibraryCmd(places.RecentsName, placePaths(m.recents))
}

// recentLabel names a recent entry by its final path component.
func recentLabel(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 && i < len(trimmed)-1 {
		return trimmed[i+1:]
	}

	return path
}

// placePaths extracts the paths for persistence.
func placePaths(src []places.Place) []string {
	out := make([]string, 0, len(src))
	for _, p := range src {
		out = append(out, p.Path)
	}

	return out
}

// addBookmark bookmarks the browsed directory. Re-bookmarking an existing
// entry is a no-op that keeps the current position.
func (m *Model) addBookmark() (tea.Model, tea.Cmd) {
	path := m.browser.Path
	for _, p := range m.bookmarks {
		if p.Path == path {
			m.note = "already bookmarked"

			return m, nil
		}
	}

	m.bookmarks = append(m.bookmarks, places.Place{Label: recentLabel(path), Path: path})
	m.note = "bookmarked " + recentLabel(path)

	return m, saveLibraryCmd(places.BookmarksName, placePaths(m.bookmarks))
}

// removeBookmark drops the browsed directory from the bookmarks. The cursor
// steps back when the removed entry sat before it so the highlight stays on
// the same entry.
func (m *Model) removeBookmark() (tea.Model, tea.Cmd) {
	path := m.browser.Path
	for i, p := range m.bookmarks {
		if p.Path != path {
			continue
		}

		removedIdx := len(m.sidebarItems())
		for idx, item := range m.sidebarItems() {
			if item.Section == "bookmarks" && item.Path == path {
				removedIdx = idx

				break
			}
		}
		m.bookmarks = append(m.bookmarks[:i], m.bookmarks[i+1:]...)
		if m.placeIdx >= removedIdx && m.placeIdx > 0 {
			m.placeIdx--
		}
		m.note = "bookmark removed"

		return m, saveLibraryCmd(places.BookmarksName, placePaths(m.bookmarks))
	}

	m.note = "not bookmarked"

	return m, nil
}

// listenOperations subscribes to the operation event stream. Update re-arms
// it after every event so exactly one listener blocks at a time.
func listenOperations(ops *operations.Manager) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ops.Events()
		if !ok {
			return nil
		}

		return OperationEventMsg{Event: ev}
	}
}

// DrainEvents consumes every pending operation event so the serial worker
// can proceed. The Bubble Tea listener normally does this; it is useful for
// driving the model from tests and for programmatic use.
func (m *Model) DrainEvents() {
	for {
		select {
		case <-m.ops.Events():
		default:
			return
		}
	}
}

// startRequest bumps the browse request counter and resets transient load
// state, invalidating any in-flight result. Callers build their command
// with the returned ID.
func (m *Model) startRequest() uint64 {
	m.requestID++
	m.loading = true
	m.loadErr = nil
	m.note = ""

	return m.requestID
}
