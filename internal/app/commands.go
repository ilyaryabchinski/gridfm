package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"gridfm/internal/browser"
	"gridfm/internal/places"
	"gridfm/internal/preview"

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

// openEntryCmd resolves an entry explicitly opened by the user. Symlinks
// are followed only here, per the filesystem rules: directories load, and
// file targets come back as a typed resolution for the file opener.
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
			return EntryResolvedMsg{Path: path, RequestID: requestID}
		}

		entries, err := browser.ReadDir(path)

		return DirectoryLoadedMsg{RequestID: requestID, Path: path, Entries: entries, Err: err}
	}
}

// desktopOpenCmd starts xdg-open off the update loop and waits for it, so
// post-start failures surface as a typed message instead of vanishing. The
// open ID tags the completion for stale-result rejection.
func desktopOpenCmd(openID uint64, path string) tea.Cmd {
	return func() tea.Msg {
		//nolint:gosec // xdg-open is a fixed program; the path is passed as an argument, never a shell string
		cmd := exec.CommandContext(context.Background(), "xdg-open", path)

		err := cmd.Start()
		if err != nil {
			return OpenFinishedMsg{Path: path, RequestID: openID, Err: fmt.Errorf("start xdg-open: %w", err)}
		}

		err = cmd.Wait()
		if err != nil {
			return OpenFinishedMsg{Path: path, RequestID: openID, Err: fmt.Errorf("xdg-open: %w", err)}
		}

		return OpenFinishedMsg{Path: path, RequestID: openID}
	}
}

// loadPlacesCmd discovers sidebar places. It reads the user's home area,
// standard folder names, mounted volumes, and the persisted bookmark and
// recent libraries.
func loadPlacesCmd() tea.Cmd {
	return func() tea.Msg {
		return PlacesLoadedMsg{
			Places:    places.List(),
			Bookmarks: places.LoadBookmarks(),
			Mounts:    places.Mounts(),
			Recents:   places.LoadRecents(),
			Home:      places.Home(),
		}
	}
}

// saveLibraryCmd persists one library file off the update loop and reports
// the outcome, since a lost bookmark write should not vanish silently.
func saveLibraryCmd(name string, paths []string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch name {
		case places.BookmarksName:
			err = places.SaveBookmarks(paths)
		case places.RecentsName:
			err = places.SaveRecents(paths)
		default:
			err = fmt.Errorf("unknown library %q", name)
		}

		return LibrarySavedMsg{Name: name, Err: err}
	}
}

// inspectCmd collects one entry's inspector metadata off the update loop.
// The request ID tags the result for stale rejection.
func inspectCmd(requestID uint64, path string) tea.Cmd {
	return func() tea.Msg {
		info, err := preview.Inspect(path)

		return InspectorLoadedMsg{RequestID: requestID, Path: path, Info: info, Err: err}
	}
}
