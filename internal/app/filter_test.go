package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/places"
)

// filteredModel builds a grid-only model browsing root with the filter
// "sub" active, and one real subdirectory entry focused.
func filteredModel(t *testing.T) *app.Model {
	t.Helper()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "notes.txt", Path: filepath.Join(root, "notes.txt")},
		{Name: "sub", Path: filepath.Join(root, "sub"), IsDir: true},
	}, nil)

	// Type a filter: "/" then "sub" then enter commits it.
	m = press(t, m, "/")
	for _, r := range []string{"s", "u", "b"} {
		m = press(t, m, r)
	}
	m = press(t, m, "enter")

	if m.Filter() != "sub" {
		t.Fatalf("filter = %q, want sub", m.Filter())
	}

	return m
}

// TestEnteringDirectoryClearsFilter pins that navigating into a
// subdirectory drops the filter: carrying a query into a listing it
// matched nothing in hides the new directory's contents.
func TestEnteringDirectoryClearsFilter(t *testing.T) {
	t.Parallel()

	m := filteredModel(t)
	root := m.Path()

	// Enter on the focused directory resolves it, and the resolution
	// delivers the new directory listing.
	next, cmd := m.Update(keyMsg("enter"))
	m = next.(*app.Model)
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.DirectoryLoadedMsg); ok && loaded.Path == filepath.Join(root, "sub") {
			m = feed(t, m, loaded)
		}
	}

	if m.Filter() != "" {
		t.Fatalf("filter = %q after entering a directory, want it cleared", m.Filter())
	}
	if m.Path() != filepath.Join(root, "sub") {
		t.Fatalf("path = %q, want the entered directory", m.Path())
	}
}

// TestSidebarEnterClearsFilterAndFocusesGrid pins the sidebar contract:
// choosing a place clears the filter like any other navigation, and
// keyboard focus moves back to the grid.
func TestSidebarEnterClearsFilterAndFocusesGrid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	placePath := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(placePath, 0o755); err != nil {
		t.Fatal(err)
	}

	m := app.New(root, app.Options{})
	m = resize(t, m, 120, 30)
	m = placesLoaded(t, m, []places.Place{{Label: "Elsewhere", Path: placePath}})
	m = loaded(t, m, 1, root, entriesAt(root, 5), nil)

	// Filter something, then focus the sidebar and pick the place.
	m = press(t, m, "/")
	m = press(t, m, "e")
	m = press(t, m, "enter")
	if m.Filter() != "e" {
		t.Fatalf("filter = %q, want e", m.Filter())
	}
	m = press(t, m, "tab")
	if m.Region() != app.RegionSidebar {
		t.Fatal("expected sidebar focus")
	}

	next, cmd := m.Update(keyMsg("enter"))
	m = next.(*app.Model)

	if m.Region() != app.RegionGrid {
		t.Fatalf("region = %v, want the grid after choosing a place", m.Region())
	}

	var loadedMsg *app.DirectoryLoadedMsg
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.DirectoryLoadedMsg); ok && loaded.Path == placePath {
			loadedMsg = &loaded
		}
	}
	if loadedMsg == nil {
		t.Fatal("choosing a place must start a load for it")
	}

	m = feed(t, m, *loadedMsg)
	if m.Filter() != "" {
		t.Fatalf("filter = %q after sidebar navigation, want it cleared", m.Filter())
	}
	if m.Path() != placePath {
		t.Fatalf("path = %q, want %q", m.Path(), placePath)
	}
}

// TestRefreshKeepsFilter pins the boundary: a same-directory refresh is
// not navigation and must keep the filter.
func TestRefreshKeepsFilter(t *testing.T) {
	t.Parallel()

	m := filteredModel(t)
	root := m.Path()

	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: []browser.Entry{
		{Name: "notes.txt", Path: filepath.Join(root, "notes.txt")},
		{Name: "sub", Path: filepath.Join(root, "sub"), IsDir: true},
	}})

	if m.Filter() != "sub" {
		t.Fatalf("filter = %q after a same-directory refresh, want it kept", m.Filter())
	}
}

// TestFilterStillHidesEntries pins that the filter does its normal job
// while active, so the clearing tests mean something.
func TestFilterStillHidesEntries(t *testing.T) {
	t.Parallel()

	m := filteredModel(t)

	visible := m.Visible()
	if len(visible) != 1 || visible[0].Name != "sub" {
		t.Fatalf("visible = %+v, want only sub", visible)
	}
}
