package app_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/places"
	"gridfm/internal/ui"
)

func placesLoaded(t *testing.T, m *app.Model, list []places.Place) *app.Model {
	t.Helper()

	return feed(t, m, app.PlacesLoadedMsg{Places: list, Home: "/home/tester"})
}

func samplePlaces() []places.Place {
	return []places.Place{
		{Label: "Home", Path: "/home/tester"},
		{Label: "Downloads", Path: "/home/tester/Downloads"},
		{Label: "Projects", Path: "/home/tester/Projects"},
	}
}

func TestSidebarLoadsAndTakesFocus(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{Icons: ui.IconModeUnicode}), 100, 30)
	m = placesLoaded(t, m, samplePlaces())

	if m.PlaceCount() != 3 {
		t.Fatalf("places = %d, want 3", m.PlaceCount())
	}
	if m.Region() != app.RegionGrid {
		t.Error("grid should own focus initially")
	}

	m = press(t, m, "tab")
	if m.Region() != app.RegionSidebar {
		t.Error("tab should move focus to the sidebar")
	}
	m = press(t, m, "tab")
	if m.Region() != app.RegionGrid {
		t.Error("second tab should return focus to the grid")
	}
}

func TestSidebarRendersPlacesAndMarker(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = placesLoaded(t, m, samplePlaces())
	m = press(t, m, "tab")

	view := m.View()
	if !strings.Contains(view, "places") {
		t.Errorf("sidebar should carry a title, got %q", view)
	}
	if !strings.Contains(view, "Downloads") {
		t.Errorf("sidebar should list places, got %q", view)
	}
	if !strings.Contains(view, "> Home") {
		t.Errorf("focused sidebar should mark the selection, got %q", view)
	}
}

func TestSidebarNavigationAndEnterOpensPlace(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = placesLoaded(t, m, samplePlaces())
	m = press(t, m, "tab")

	m = press(t, m, "j")
	place, ok := m.SelectedPlace()
	if !ok || place.Label != "Downloads" {
		t.Fatalf("selected place after j = %+v, want Downloads", place)
	}

	m = press(t, m, "enter")
	if !m.IsLoading() {
		t.Fatal("entering a place should start a load")
	}

	// The confirmed load lands in the browser.
	m = loaded(t, m, 2, place.Path, nil, nil)
	if m.Path() != "/home/tester/Downloads" {
		t.Errorf("Path after place load = %q, want the place path", m.Path())
	}
}

func TestEnterInSidebarWithoutPlacesIsSafe(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = placesLoaded(t, m, nil)
	m = press(t, m, "tab")

	// With no entries in any section, enter must produce no action at all.
	next, cmd := m.Update(keyMsg("enter"))
	if _, ok := next.(*app.Model); !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd != nil {
		t.Error("enter with an empty sidebar should do nothing")
	}
}

func TestSidebarToggleHidesAndRestores(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = placesLoaded(t, m, samplePlaces())
	if !strings.Contains(m.View(), "Downloads") {
		t.Fatal("sidebar should render by default at wide sizes")
	}

	m = press(t, m, "~")
	if strings.Contains(m.View(), "Downloads") {
		t.Error("~ should hide the sidebar")
	}

	// Focus cannot enter a hidden sidebar.
	m = press(t, m, "tab")
	if m.Region() != app.RegionGrid {
		t.Error("tab should not focus a hidden sidebar")
	}

	m = press(t, m, "~")
	if !strings.Contains(m.View(), "Downloads") {
		t.Error("~ should bring the sidebar back")
	}
}

func TestNarrowTerminalUsesOverlay(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 60, 20)
	m = placesLoaded(t, m, samplePlaces())
	m = loaded(t, m, 1, "/d", entriesAt("/d", 3), nil)

	// The grid keeps the full width and the sidebar floats over its left
	// edge: later cards stay visible beside the panel.
	if !strings.Contains(m.View(), "entry-02.txt") {
		t.Fatal("narrow overlay should keep the grid visible beside the panel")
	}
	if !strings.Contains(m.View(), "Downloads") {
		t.Error("narrow overlay should render the sidebar on top")
	}

	m = press(t, m, "tab")
	if m.Region() != app.RegionSidebar {
		t.Error("overlay sidebar should be focusable")
	}

	m = press(t, m, "~")
	if strings.Contains(m.View(), "Downloads") {
		t.Error("~ should dismiss the overlay")
	}
}

func TestZoomKeysChangeCardDensity(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	if m.Zoom() != ui.ZoomNormal {
		t.Fatalf("default zoom = %v, want normal", m.Zoom())
	}

	m = press(t, m, "+")
	if m.Zoom() != ui.ZoomDetailed {
		t.Errorf("zoom after + = %v, want detailed", m.Zoom())
	}

	m = press(t, m, "-")
	m = press(t, m, "-")
	if m.Zoom() != ui.ZoomCompact {
		t.Errorf("zoom after two - = %v, want compact", m.Zoom())
	}

	// Detailed cards surface metadata.
	m = press(t, m, "+")
	m = press(t, m, "+")
	m = loaded(t, m, 1, "/d", []browser.Entry{{
		Name: "big.bin", Path: "/d/big.bin", Size: 1536,
	}}, nil)
	if view := m.View(); !strings.Contains(view, "1.5K") {
		t.Errorf("detailed view should show human size, got %q", view)
	}
}

func TestZoomClampsAtBounds(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	for range 5 {
		m = press(t, m, "-")
	}
	if m.Zoom() != ui.ZoomCompact {
		t.Errorf("zoom should clamp at compact, got %v", m.Zoom())
	}
	for range 5 {
		m = press(t, m, "+")
	}
	if m.Zoom() != ui.ZoomDetailed {
		t.Errorf("zoom should clamp at detailed, got %v", m.Zoom())
	}
}

func TestSortMenuOpensAppliesAndReverses(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 4), nil)

	m = press(t, m, "s")
	if !strings.Contains(m.View(), " sort ") {
		t.Error("s should open the sort menu")
	}
	if !strings.Contains(m.View(), "name asc") {
		t.Error("sort menu should show the active order")
	}

	// Down twice selects "modified"; enter applies it and closes.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.SortMode() != browser.SortByModified {
		t.Errorf("sort mode after enter = %v, want modified", m.SortMode())
	}
	if strings.Contains(m.View(), " sort ") {
		t.Error("enter should close the sort menu")
	}

	// Reopening with the same selection and applying flips the order.
	m = press(t, m, "s")
	m = press(t, m, "enter")
	if m.SortOrder() != browser.SortDescending {
		t.Errorf("re-applying the mode should reverse order, got %v", m.SortOrder())
	}

	// o reverses without closing.
	m = press(t, m, "s")
	m = press(t, m, "o")
	if m.SortOrder() != browser.SortAscending {
		t.Errorf("o should reverse the order, got %v", m.SortOrder())
	}

	// esc closes without changes.
	m = press(t, m, "s")
	m = press(t, m, "esc")
	if strings.Contains(m.View(), " sort ") {
		t.Error("esc should close the sort menu")
	}
}

func TestSortReordersEntriesDeterministically(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	entries := []browser.Entry{
		{Name: "b.zip", Path: "/d/b.zip", Size: 20},
		{Name: "a.zip", Path: "/d/a.zip", Size: 10},
		{Name: "c.txt", Path: "/d/c.txt", Size: 30},
	}
	m = loaded(t, m, 1, "/d", entries, nil)

	// Sort by type asc: text sorts before zip groups, zip files break the
	// tie by name.
	m = press(t, m, "s")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "j") // cursor at "type"
	m = press(t, m, "enter")

	visible := m.Visible()
	if visible[0].Name != "c.txt" || visible[1].Name != "a.zip" || visible[2].Name != "b.zip" {
		t.Errorf("type sort order = %v", names(visible))
	}
}

func TestHiddenToggle(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: ".env", Path: "/d/.env"},
		{Name: "main.go", Path: "/d/main.go"},
	}, nil)

	if m.ShowHidden() {
		t.Fatal("hidden files start hidden")
	}
	if view := m.View(); strings.Contains(view, ".env") {
		t.Error("hidden entries should not render by default")
	}

	m = press(t, m, ".")
	if !m.ShowHidden() {
		t.Fatal(". should reveal hidden entries")
	}
	if view := m.View(); !strings.Contains(view, ".env") {
		t.Error("revealed hidden entries should render")
	}
	if view := m.View(); !strings.Contains(view, "hidden on") {
		t.Errorf("status bar should announce hidden mode, got %q", view)
	}

	m = press(t, m, ".")
	if m.ShowHidden() {
		t.Error(". should hide hidden entries again")
	}
}

func TestFilterTypesLiveAndCommits(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "main.go", Path: "/d/main.go"},
		{Name: "readme.md", Path: "/d/readme.md"},
		{Name: "notes.md", Path: "/d/notes.md"},
	}, nil)

	m = press(t, m, "/")
	if !m.FilterInput() {
		t.Fatal("/ should enter filter input mode")
	}

	m = press(t, m, "m")
	m = press(t, m, "a")
	if m.Filter() != "ma" {
		t.Fatalf("filter = %q, want ma", m.Filter())
	}
	visible := m.Visible()
	if len(visible) != 1 || visible[0].Name != "main.go" {
		t.Errorf("filter should narrow live, got %v", names(visible))
	}

	// Named keys never leak into the query.
	m = press(t, m, "tab")
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.Filter() != "ma" {
		t.Errorf("named keys must not extend the filter, got %q", m.Filter())
	}

	// Enter commits and leaves input mode; the filter stays active.
	m = press(t, m, "enter")
	if m.FilterInput() {
		t.Error("enter should leave filter input mode")
	}
	if m.Filter() != "ma" {
		t.Errorf("enter should keep the filter, got %q", m.Filter())
	}
	if view := m.View(); !strings.Contains(view, "1 of 3 items") {
		t.Errorf("status should show filtered counts, got %q", view)
	}
}

func TestFilterEscClearsEverything(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "main.go", Path: "/d/main.go"},
		{Name: "readme.md", Path: "/d/readme.md"},
	}, nil)

	m = press(t, m, "/")
	m = press(t, m, "m")
	m = press(t, m, "esc")
	if m.Filter() != "" || m.FilterInput() {
		t.Fatalf("esc during input should cancel and clear, got %q active=%v", m.Filter(), m.FilterInput())
	}

	// esc after a committed filter clears it too.
	m = press(t, m, "/")
	m = press(t, m, "m")
	m = press(t, m, "enter")
	m = press(t, m, "esc")
	if m.Filter() != "" {
		t.Errorf("esc should clear the committed filter, got %q", m.Filter())
	}
	if len(m.Visible()) != 2 {
		t.Error("clearing the filter should restore all entries")
	}
}

func TestFilterBackspaceAndCancel(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "main.go", Path: "/d/main.go"},
		{Name: "makefile", Path: "/d/makefile"},
	}, nil)

	m = press(t, m, "/")
	m = press(t, m, "m")
	m = press(t, m, "a")
	m = press(t, m, "x")
	if m.Filter() != "max" {
		t.Fatalf("filter = %q, want max", m.Filter())
	}

	m = press(t, m, "backspace")
	if m.Filter() != "ma" {
		t.Errorf("backspace should trim the query, got %q", m.Filter())
	}

	m = press(t, m, "esc")
	if m.Filter() != "" || m.FilterInput() {
		t.Error("esc should cancel input mode and clear the filter")
	}
}

func TestHistoryKeysNavigate(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/root", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/root", entriesAt("/root", 2), nil)

	m = pressBackspace(t, m)
	m = loaded(t, m, 2, "/tmp", entriesAt("/tmp", 2), nil)

	m = feed(t, m, tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if !m.IsLoading() {
		t.Fatal("alt+left should start a back navigation")
	}
	m = loaded(t, m, 3, "/root", entriesAt("/root", 2), nil)
	if m.Path() != "/root" {
		t.Errorf("Path after back = %q, want /root", m.Path())
	}

	m = feed(t, m, tea.KeyMsg{Type: tea.KeyRight, Alt: true})
	m = loaded(t, m, 4, "/tmp", entriesAt("/tmp", 2), nil)
	if m.Path() != "/tmp" {
		t.Errorf("Path after forward = %q, want /tmp", m.Path())
	}
}

func TestEscalatingEscClearsTransientState(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = press(t, m, "/")
	m = press(t, m, "a")
	m = press(t, m, "enter") // commit filter
	m = press(t, m, "esc")
	if m.Filter() != "" {
		t.Fatalf("first esc clears the filter, got %q", m.Filter())
	}

	// Second esc clears a note. The open outcome note arrives with the
	// OpenFinishedMsg that also refreshes the directory.
	m = feed(t, m, app.OpenFinishedMsg{Path: "/d/x.bin", RequestID: 1})
	m = press(t, m, "esc")
	if strings.Contains(m.View(), "opened x.bin") {
		t.Error("esc should clear the note")
	}
}

func TestDesktopOpenReportsOutcomeAndRefreshes(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{{Name: "big.bin", Path: "/d/big.bin"}}, nil)

	// Entering the file starts the async opener.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd == nil {
		t.Fatal("desktop open should produce a command")
	}

	// Success reports the opened file and refreshes the directory.
	opened = feed(t, opened, app.OpenFinishedMsg{Path: "/d/big.bin", RequestID: 1})
	if view := opened.View(); !strings.Contains(view, "opened big.bin") {
		t.Errorf("view should announce the opened file, got %q", view)
	}
	if !opened.IsLoading() {
		t.Error("an open completion should refresh the directory")
	}

	// Failure surfaces the error instead.
	failed := feed(t, opened, app.OpenFinishedMsg{Path: "/d/big.bin", RequestID: 1, Err: errTestLoad})
	if view := failed.View(); !strings.Contains(view, "open failed: permission denied") {
		t.Errorf("view should surface opener failures, got %q", view)
	}
}

func TestHeaderShowsHistoryAvailability(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/root", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/root", entriesAt("/root", 2), nil)

	// The indicators always render; availability styling is driven by the
	// browser state.
	view := m.View()
	if !strings.Contains(view, "←") || !strings.Contains(view, "→") {
		t.Errorf("header should show history indicators, got %q", view)
	}

	// After descending one level, back is available while forward is not;
	// the header keeps rendering both directions.
	m = pressBackspace(t, m)
	m = loaded(t, m, 2, "/root/deep", entriesAt("/root/deep", 2), nil)
	header, _, _ := strings.Cut(m.View(), "\n")
	if !strings.Contains(header, "←") || !strings.Contains(header, "→") {
		t.Errorf("header should keep both history indicators, got %q", header)
	}
}

func names(entries []browser.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}

	return out
}

func TestOpenCompletionDuringPendingBackDoesNotStrandHistory(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/root", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/root", entriesAt("/root", 2), nil)
	m = pressBackspace(t, m)
	// A desktop-managed file, not an editor target: the editor path
	// depends on $VISUAL/$EDITOR and would make this test env-dependent.
	desktopEntries := []browser.Entry{
		{Name: "thing.bin", Path: "/root/deep/thing.bin"},
		{Name: "other.bin", Path: "/root/deep/other.bin"},
	}
	m = loaded(t, m, 2, "/root/deep", desktopEntries, nil)

	// Open a desktop-managed file (open identity 1), then start a back
	// traversal (browse request 2) while the opener is still running.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd == nil {
		t.Fatal("enter on a desktop file should produce an open command")
	}

	m = feed(t, opened, tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if !m.IsLoading() {
		t.Fatal("alt+left should start a back navigation")
	}

	// The opener completes mid-navigation: it must not start a refresh that
	// would invalidate the pending back result...
	m = feed(t, m, app.OpenFinishedMsg{Path: "/root/deep/thing.bin", RequestID: 1})
	if got := m.Path(); got != "/root/deep" {
		t.Errorf("stale handling changed the location: %q", got)
	}

	// ...so the back result applies normally and history stays healthy.
	// The back request is ID 3 (1 = initial load, 2 = descend).
	m = loaded(t, m, 3, "/root", entriesAt("/root", 2), nil)
	if got := m.Path(); got != "/root" {
		t.Fatalf("Path after back = %q, want /root", got)
	}
	if m.IsLoading() {
		t.Fatal("loading should end once the back result applies")
	}

	// Forward must still work: the pending step was not stranded.
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyRight, Alt: true})
	m = loaded(t, m, 4, "/root/deep", entriesAt("/root/deep", 2), nil)
	if got := m.Path(); got != "/root/deep" {
		t.Errorf("Path after forward = %q, want /root/deep (history not stranded)", got)
	}
}
