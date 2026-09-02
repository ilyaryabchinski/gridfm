package app

import (
	"fmt"
	"os"

	"gridfm/internal/browser"

	tea "github.com/charmbracelet/bubbletea"
)

// loadDirectoryCmd reads a directory off the update loop and delivers the
// typed result, tagged with the request ID that produced it.
func loadDirectoryCmd(requestID uint64, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := browser.ReadDir(path)

		return DirectoryLoadedMsg{RequestID: requestID, Path: path, Entries: entries, Err: err}
	}
}

// openEntryCmd resolves an entry and enters it when it is a directory.
// Symlinks are followed only on explicit open, per the filesystem rules.
func openEntryCmd(requestID uint64, path string) tea.Cmd {
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return DirectoryLoadedMsg{
				RequestID: requestID,
				Path:      path,
				Err:       fmt.Errorf("stat %q: %w", path, err),
			}
		}
		if !info.IsDir() {
			return EntryNotDirectoryMsg{Path: path, RequestID: requestID}
		}
		entries, err := browser.ReadDir(path)

		return DirectoryLoadedMsg{RequestID: requestID, Path: path, Entries: entries, Err: err}
	}
}
