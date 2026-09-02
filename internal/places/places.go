// Package places discovers well-known user locations for the sidebar:
// the home directory plus standard user folders that exist on disk.
package places

import (
	"os"
	"path/filepath"
)

// Place is a sidebar shortcut to a well-known location.
type Place struct {
	Label string
	Path  string
}

// userDirs are the standard user folders checked for existence, in display
// order. Projects is a convention of this project's audience rather than an
// XDG folder, but is listed alongside them when present.
//
//nolint:gochecknoglobals // static candidate list, never mutated
var userDirs = []string{
	"Downloads",
	"Documents",
	"Pictures",
	"Music",
	"Videos",
	"Projects",
}

// List returns the places for the sidebar: Home first, then each standard
// user folder that exists. It never fails; without a home directory the
// list is simply empty.
func List() []Place {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}

	list := []Place{{Label: "Home", Path: home}}
	for _, dir := range userDirs {
		path := filepath.Join(home, dir)
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			continue
		}

		list = append(list, Place{Label: dir, Path: path})
	}

	return list
}

// Home returns the user's home directory, or an empty string when unknown.
func Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}
