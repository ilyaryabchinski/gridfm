package app

import (
	"gridfm/internal/browser"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the top-level Bubble Tea model. It owns domain state (the
// browser), request identity for stale-result rejection, and transient view
// state. Render functions read it and never mutate it.
type Model struct {
	browser   browser.Browser
	loadErr   error
	note      string
	requestID uint64
	width     int
	height    int
	scrollRow int
	loading   bool
}

// New returns a model that will browse startPath once the first directory
// load completes. All Model methods use pointer receivers so state changes
// can never be lost to a value copy.
func New(startPath string) *Model {
	return &Model{
		browser:   browser.New(startPath),
		requestID: 1,
		loading:   true,
	}
}

// Init issues the initial directory load.
func (m *Model) Init() tea.Cmd {
	return loadDirectoryCmd(m.requestID, m.browser.Path)
}

// Path returns the browsed directory.
func (m *Model) Path() string { return m.browser.Path }

// Entries returns the loaded entries.
func (m *Model) Entries() []browser.Entry { return m.browser.Entries }

// FocusedPath returns the focused entry path, or an empty string.
func (m *Model) FocusedPath() string { return m.browser.FocusedPath() }

// IsLoading reports whether a directory load is in flight.
func (m *Model) IsLoading() bool { return m.loading }

// LoadError returns the last directory load error, if any.
func (m *Model) LoadError() error { return m.loadErr }

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
