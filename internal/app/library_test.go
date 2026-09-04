package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/places"
)

// pressCapture presses a key and returns the produced command, so tests can
// run commands that persist state.
func pressCapture(t *testing.T, m *app.Model, s string) (*app.Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}

	return updated, cmd
}

// runAndFeed executes a command and feeds its message back into the model,
// asserting library saves succeeded.
func runAndFeed(t *testing.T, m *app.Model, cmd tea.Cmd) *app.Model {
	t.Helper()
	if cmd == nil {
		return m
	}

	msg := cmd()
	if saved, ok := msg.(app.LibrarySavedMsg); ok {
		if saved.Err != nil {
			t.Fatalf("library save failed: %v", saved.Err)
		}

		return m
	}

	return feed(t, m, msg)
}

//nolint:paralleltest // t.Setenv mutates process env, so the test is serial
func TestBookmarkAddRemovePersists(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "project")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// The sidebar stays docked so the bookmarks section is visible.
	m := resize(t, app.New(sub, app.Options{}), 80, 24)
	m = loaded(t, m, 1, sub, nil, nil)

	// b bookmarks the browsed directory and persists it.
	m, cmd := pressCapture(t, m, "b")
	if !strings.Contains(m.View(), "bookmarked project") {
		t.Fatalf("note should confirm the bookmark, got %q", m.View())
	}
	if len(m.Bookmarks()) != 1 || m.Bookmarks()[0].Path != sub {
		t.Fatalf("bookmarks = %+v, want %q", m.Bookmarks(), sub)
	}
	m = runAndFeed(t, m, cmd)
	loadedBookmarks := places.LoadBookmarks()
	if len(loadedBookmarks) != 1 || loadedBookmarks[0].Path != sub {
		t.Fatalf("persisted bookmarks = %+v, want %q", loadedBookmarks, sub)
	}
	if !strings.Contains(m.View(), "bookmarks") {
		t.Error("the sidebar should show the bookmarks section")
	}

	// b again is a no-op.
	m, cmd = pressCapture(t, m, "b")
	if !strings.Contains(m.View(), "already bookmarked") {
		t.Error("re-bookmarking should say so instead of duplicating")
	}
	if cmd != nil {
		t.Error("a no-op bookmark must not rewrite the file")
	}

	// B removes the bookmark and persists the removal.
	m, cmd = pressCapture(t, m, "B")
	if !strings.Contains(m.View(), "bookmark removed") {
		t.Fatalf("note should confirm removal, got %q", m.View())
	}
	m = runAndFeed(t, m, cmd)
	if got := places.LoadBookmarks(); len(got) != 0 {
		t.Errorf("the bookmark file should be empty after removal, got %v", got)
	}

	// B with nothing bookmarked says so.
	m, cmd = pressCapture(t, m, "B")
	if !strings.Contains(m.View(), "not bookmarked") {
		t.Error("removing without a bookmark should say so")
	}
	if cmd != nil {
		t.Error("removing without a bookmark must not rewrite the file")
	}
}

//nolint:paralleltest // t.Setenv mutates process env, so the test is serial
func TestRecentsTrackLoadedDirectories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one")
	second := filepath.Join(root, "two")
	for _, dir := range []string{first, second} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := gridOnly(t, resize(t, app.New(first, app.Options{}), 80, 24))
	// Same request identity: the test feeds loads directly without
	// navigating, so each one supersedes nothing and applies.
	m = loaded(t, m, 1, first, nil, nil)
	m = loaded(t, m, 1, second, nil, nil)

	recents := m.Recents()
	if len(recents) != 2 || recents[0].Path != second || recents[1].Path != first {
		t.Fatalf("recents = %+v, want newest first two entries", recents)
	}

	// Reloading the same directory must not duplicate the entry.
	m = loaded(t, m, 1, second, nil, nil)
	if got := m.Recents(); len(got) != 2 {
		t.Fatalf("recents after reload = %+v, want two", got)
	}
}

func TestPlacesMessagePopulatesAllSections(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = feed(t, m, app.PlacesLoadedMsg{
		Places:    []places.Place{{Label: "Home", Path: "/home/x"}},
		Bookmarks: []places.Place{{Label: "proj", Path: "/code/proj"}},
		Mounts:    []places.Place{{Label: "Backup", Path: "/run/media/x/Backup"}},
		Recents:   []places.Place{{Label: "dl", Path: "/home/x/Downloads"}},
		Home:      "/home/x",
	})

	view := m.View()
	for _, want := range []string{"places", "Home", "bookmarks", "proj", "mounts", "Backup", "recents", "dl"} {
		if !strings.Contains(view, want) {
			t.Errorf("sidebar should show %q", want)
		}
	}

	// The cursor navigates the combined list end to end.
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	for range 2 {
		m = press(t, m, "j")
	}
	place, ok := m.SelectedPlace()
	if !ok || place.Path != "/run/media/x/Backup" {
		t.Fatalf("selected = %+v, %v; want the mounts entry", place, ok)
	}
}
