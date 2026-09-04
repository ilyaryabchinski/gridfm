// Package app wires the browser domain model to the Bubble Tea runtime: the
// top-level model, message routing, asynchronous commands, and rendering.
// Filesystem work happens only inside commands; the update loop never waits
// on I/O.
package app

import (
	"gridfm/internal/browser"
	"gridfm/internal/operations"
	"gridfm/internal/places"
	"gridfm/internal/preview"
)

// DirectoryLoadedMsg carries the result of an asynchronous directory read.
// Results are applied only when RequestID matches the newest request, which
// rejects stale loads issued before a more recent navigation.
type DirectoryLoadedMsg struct {
	Entries   []browser.Entry
	Path      string
	Err       error
	RequestID uint64
}

// EntryResolvedMsg reports that an explicitly opened entry resolved to a
// non-directory target (typically a symlink to a file). The entry should be
// opened with the file opener. Like all load results it carries the request
// ID so stale attempts are discarded.
type EntryResolvedMsg struct {
	Path      string
	RequestID uint64
}

// PlacesLoadedMsg carries the discovered sidebar sources and the home path
// used for breadcrumb abbreviation: standard places plus the persisted
// bookmarks, mounted volumes, and recent locations.
type PlacesLoadedMsg struct {
	Places    []places.Place
	Bookmarks []places.Place
	Mounts    []places.Place
	Recents   []places.Place
	Home      string
}

// LibrarySavedMsg reports the outcome of persisting bookmarks or recents.
// Name is the library file involved; a nil Err means the save succeeded.
type LibrarySavedMsg struct {
	Name string
	Err  error
}

// OpenFinishedMsg reports the completion of an externally opened file:
// an editor session launched with terminal hand-off, or the desktop
// opener's process. A nil Err means it exited cleanly. RequestID is the
// open that produced the completion; stale completions are discarded so a
// late opener can never supersede newer navigation.
type OpenFinishedMsg struct {
	Path      string
	Err       error
	RequestID uint64
}

// OperationEventMsg wraps one event from the operation manager: progress,
// a conflict question, or a finished result. The listener command re-arms
// after every delivery so exactly one listener blocks at a time.
type OperationEventMsg struct {
	Event operations.Event
}

// InspectorLoadedMsg carries the metadata result for the inspector panel.
// Results apply only when RequestID matches the newest inspector request,
// so a slow load can never overwrite a newer focus.
type InspectorLoadedMsg struct {
	RequestID uint64
	Path      string
	Info      *preview.Info
	Err       error
}

// DirChangedMsg is one debounced filesystem change notification for the
// browsed directory. It is a hint to refresh, never a statement about what
// changed. A nil Err carries the change; an Err reports the watcher's
// degradation.
type DirChangedMsg struct {
	Path string
	Err  error
}

// WatchReadyMsg reports the outcome of pointing the watcher at a directory.
// A nil Err means the directory is now watched.
type WatchReadyMsg struct {
	Path string
	Err  error
}
