package app

import (
	"gridfm/internal/browser"
	"gridfm/internal/places"
	"gridfm/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Region names the half of the interface that owns keyboard focus.
type Region int

// Focused interface regions.
const (
	RegionGrid Region = iota
	RegionSidebar
)

// Options are the start-up configuration switches.
type Options struct {
	Icons ui.IconMode
}

// Model is the top-level Bubble Tea model. It owns domain state (the
// browser), request identity for stale-result rejection, and transient view
// state. Render functions read it and never mutate it.
type Model struct {
	browser browser.Browser
	places  []places.Place

	icons     ui.IconMode
	zoom      ui.ZoomLevel
	region    Region
	sidebarOn bool
	placeIdx  int
	home      string

	// Sort menu state: the only blocking overlay in Milestone 1.
	sortOpen   bool
	sortCursor int

	// Filter input state: when active, printable keys extend the query.
	filterInput bool

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
	return &Model{
		browser:   browser.New(startPath),
		icons:     opts.Icons,
		zoom:      ui.ZoomNormal,
		sidebarOn: true,
		requestID: 1,
		loading:   true,
	}
}

// Init issues the initial directory load and place discovery.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		loadDirectoryCmd(m.requestID, m.browser.Path),
		loadPlacesCmd(),
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

// SelectedPlace returns the sidebar place under the cursor.
func (m *Model) SelectedPlace() (places.Place, bool) {
	if m.placeIdx < 0 || m.placeIdx >= len(m.places) {
		return places.Place{}, false
	}

	return m.places[m.placeIdx], true
}

// startRequest bumps the request counter and resets transient load state,
// invalidating any in-flight result. Callers build their command with the
// returned ID.
func (m *Model) startRequest() uint64 {
	m.requestID++
	m.loading = true
	m.loadErr = nil
	m.note = ""

	return m.requestID
}
