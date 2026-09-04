package places

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Library file names under the gridfm config directory.
const (
	bookmarksFile = "bookmarks.conf"
	recentsFile   = "recents.conf"
)

// Library names exposed for callers building save commands.
const (
	BookmarksName = bookmarksFile
	RecentsName   = recentsFile
)

// ConfigDir returns $XDG_CONFIG_HOME/gridfm, falling back to
// ~/.config/gridfm. It does not create the directory; saving does.
func ConfigDir() string {
	if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
		return filepath.Join(home, "gridfm")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "gridfm")
	}

	return ""
}

// LoadLines reads one library file as trimmed, non-empty, non-comment
// lines in file order. A missing file is an empty list, not an error.
func LoadLines(name string) []string {
	dir := ConfigDir()
	if dir == "" {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // path is built from fixed config roots
	if err != nil {
		return nil
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}

	return lines
}

// SaveLines writes one library file atomically: the new content lands in a
// temp file that replaces the target only after a full write. The config
// directory is created as needed.
func SaveLines(name string, lines []string) error {
	dir := ConfigDir()
	if dir == "" {
		return errors.New("no config directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	content := "# gridfm " + name + ": one path per line\n"
	for _, line := range lines {
		content += line + "\n"
	}

	tmp, err := os.CreateTemp(dir, name+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // no-op after a successful rename

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close() //nolint:errcheck // the write error is the one reported

		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}

// LoadBookmarks returns the bookmarked directories, in file order.
func LoadBookmarks() []Place {
	return placesFromLines(LoadLines(bookmarksFile))
}

// SaveBookmarks persists the bookmark paths.
func SaveBookmarks(paths []string) error {
	return SaveLines(bookmarksFile, paths)
}

// LoadRecents returns the recently visited directories, newest first.
func LoadRecents() []Place {
	return placesFromLines(LoadLines(recentsFile))
}

// SaveRecents persists the recent paths.
func SaveRecents(paths []string) error {
	return SaveLines(recentsFile, paths)
}

func placesFromLines(lines []string) []Place {
	out := make([]Place, 0, len(lines))
	for _, line := range lines {
		info, err := os.Stat(line)
		if err != nil || !info.IsDir() {
			continue // vanished entries simply do not appear
		}
		out = append(out, Place{Label: labelForPath(line), Path: line})
	}

	return out
}

func labelForPath(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}

	return path
}

// exists reports whether the path is present on disk. Used by tests.
func exists(path string) bool {
	_, err := os.Stat(path)

	return !errors.Is(err, fs.ErrNotExist)
}
