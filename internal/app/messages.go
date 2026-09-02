// Package app wires the browser domain model to the Bubble Tea runtime: the
// top-level model, message routing, asynchronous commands, and rendering.
// Filesystem work happens only inside commands; the update loop never waits
// on I/O.
package app

import "gridfm/internal/browser"

// DirectoryLoadedMsg carries the result of an asynchronous directory read.
// Results are applied only when RequestID matches the newest request, which
// rejects stale loads issued before a more recent navigation.
type DirectoryLoadedMsg struct {
	Entries   []browser.Entry
	Path      string
	Err       error
	RequestID uint64
}

// EntryNotDirectoryMsg reports that the user tried to enter a non-directory
// entry. It is a nonfatal, expected outcome.
type EntryNotDirectoryMsg struct {
	Path string
}
