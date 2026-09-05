package app_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/places"
	"gridfm/internal/preview"
)

// mouseModel builds a wide grid-only model with mouse enabled and a
// loaded directory.
func mouseModel(t *testing.T, count int) *app.Model {
	t.Helper()

	root := t.TempDir()
	m := app.New(root, app.Options{Mouse: true})
	m = gridOnly(t, resize(t, m, 120, 30))

	return loaded(t, m, 1, root, entriesAt(root, count), nil)
}

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func ctrlClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Ctrl: true}
}

func wheel(down bool) tea.MouseMsg {
	b := tea.MouseButtonWheelUp
	if down {
		b = tea.MouseButtonWheelDown
	}

	return tea.MouseMsg{Action: tea.MouseActionPress, Button: b}
}

// Card geometry at 120x30 grid-only, zero-based like Bubble Tea's mouse
// coordinates: seven columns of 14-wide cards with 2-column gaps, first
// row starting at y=1 below the header. Card one spans x 0-13, card two
// x 16-29, the gap between them is x 14-15.

// TestMouseClickFocusesCard pins that a left press focuses the clicked
// card.
func TestMouseClickFocusesCard(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)

	next, _ := m.Update(click(2, 2))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("focus after click on card one = %q, want entry-00", got)
	}

	// Card two starts at x = 16: its left border column.
	next, _ = m.Update(click(16, 2))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-01.txt" {
		t.Fatalf("focus after click on card two = %q, want entry-01", got)
	}

	// The first card of the second row: y = 7 (one card plus gap below
	// the y=1 content start), which is entry-07 at seven columns.
	next, _ = m.Update(click(2, 7))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-07.txt" {
		t.Fatalf("focus after click on row two = %q, want entry-07", got)
	}

	// The header row and the far edge are nobody's cells.
	next, _ = m.Update(click(2, 0))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-07.txt" {
		t.Fatalf("a header click must do nothing, moved to %q", got)
	}
}

// TestMouseClickInGapIsIgnored pins that clicks on the two-column gap
// between cards leave focus where it was.
func TestMouseClickInGapIsIgnored(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)
	next, _ := m.Update(click(2, 2)) // focus the first card
	m = next.(*app.Model)

	next, _ = m.Update(click(15, 2)) // gap between card one and two
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("a gap click must not move focus, got %q", got)
	}
}

// TestMouseCtrlClickTogglesSelection pins that ctrl-click selects the
// clicked entry and a second ctrl-click deselects.
func TestMouseCtrlClickTogglesSelection(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)
	next, _ := m.Update(ctrlClick(2, 2))
	m = next.(*app.Model)

	if m.SelectedCount() != 1 {
		t.Fatalf("selected = %d, want 1 after ctrl-click", m.SelectedCount())
	}
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("focus after ctrl-click = %q, want the clicked entry", got)
	}

	next, _ = m.Update(ctrlClick(2, 2))
	m = next.(*app.Model)
	if m.SelectedCount() != 0 {
		t.Fatalf("selected = %d, want 0 after a second ctrl-click", m.SelectedCount())
	}
}

// TestMouseDoubleClickOpens pins that two quick clicks on the same card
// open it, and that two clicks on different cards do not.
func TestMouseDoubleClickOpens(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := app.New(root, app.Options{Mouse: true})
	m = gridOnly(t, resize(t, m, 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "sub", Path: filepath.Join(root, "sub"), IsDir: true},
	}, nil)

	next, _ := m.Update(click(2, 2))
	m = next.(*app.Model)
	next, _ = m.Update(click(2, 2))
	m = next.(*app.Model)

	if !m.IsLoading() {
		t.Fatal("a double click must open the directory")
	}

	// Clicks on different cards, even in quick succession, must not.
	m = mouseModel(t, 10)
	next, _ = m.Update(click(2, 2))
	m = next.(*app.Model)
	next, _ = m.Update(click(16, 2))
	m = next.(*app.Model)
	if m.IsLoading() {
		t.Fatal("clicks on different cards must not register as a double click")
	}
}

// TestMouseWheelScrollsFreely pins that the wheel scrolls the view
// without snapping back to the focused entry — the snap made scrolling
// impossible whenever focus sat on the first entry.
func TestMouseWheelScrollsFreely(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30) // five rows at seven columns; four visible

	next, _ := m.Update(wheel(true))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("wheel scrolling must not move focus, got %q", got)
	}
	if !strings.Contains(m.View(), "entry-28.txt") {
		t.Error("the last row should be visible after scrolling down")
	}

	// Scrolling up at the top clamps instead of overscrolling.
	for range 3 {
		next, _ = m.Update(wheel(false))
		m = next.(*app.Model)
	}
	if strings.Contains(m.View(), "entry-28.txt") {
		t.Error("the view must not scroll past the first row")
	}
}

// TestMouseSidebarClickSelectsPlace pins that clicking the docked sidebar
// focuses it and selects the clicked entry.
func TestMouseSidebarClickSelectsPlace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := app.New(root, app.Options{Mouse: true})
	m = resize(t, m, 120, 30)
	m = placesLoaded(t, m, []places.Place{
		{Label: "Home", Path: "/home/x"},
		{Label: "Downloads", Path: "/home/x/Downloads"},
	})
	m = loaded(t, m, 1, root, entriesAt(root, 3), nil)

	// Sidebar lines start at y=1: blank, header, Home, Downloads.
	next, _ := m.Update(click(3, 4))
	m = next.(*app.Model)

	if m.Region() != app.RegionSidebar {
		t.Fatalf("region = %v, want the sidebar", m.Region())
	}
	if m.PlaceIdx() != 1 {
		t.Fatalf("selected place = %d, want 1 (Downloads)", m.PlaceIdx())
	}
}

// TestMouseClickRefreshesInspector pins that clicking a new card clears
// the panel and re-requests metadata for the newly focused entry —
// keyboard movement already does this; the mouse must not fall behind.
func TestMouseClickRefreshesInspector(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := app.New(root, app.Options{Mouse: true})
	m = gridOnly(t, resize(t, m, 120, 30))
	m = loaded(t, m, 1, root, entriesAt(root, 10), nil)

	m, _ = pressI(t, m)
	req := m.InspectorRequestID()
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: req, Path: m.FocusedPath(),
		Info: &preview.Info{Path: m.FocusedPath(), Name: "entry-00.txt"},
	})

	next, cmd := m.Update(click(16, 2)) // card two
	m = next.(*app.Model)
	if m.Inspector() != nil {
		t.Fatal("clicking another card must clear the panel")
	}

	var asked bool
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.InspectorLoadedMsg); ok && loaded.Path == filepath.Join(root, "entry-01.txt") {
			asked = true
		}
	}
	if !asked {
		t.Fatal("the click must request metadata for the newly focused entry")
	}
}

// TestMouseFloatingSidebarOccludesGrid pins that clicks on the floating
// sidebar's rectangle do not poke through to hidden cards.
func TestMouseFloatingSidebarOccludesGrid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// 60 wide is below the narrow threshold: toggling the sidebar floats
	// it over the grid. Start with it off so the first click lands.
	m := app.New(root, app.Options{Mouse: true, Sidebar: boolPtr(false)})
	m = resize(t, m, 60, 30)
	m = loaded(t, m, 1, root, entriesAt(root, 10), nil)

	next, _ := m.Update(click(2, 2)) // real card: focus moves
	m = next.(*app.Model)
	next, _ = m.Update(keyMsg("~")) // float the sidebar over the grid
	m = next.(*app.Model)
	next, _ = m.Update(click(2, 2)) // same cell, now covered
	m = next.(*app.Model)

	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("a click on the floating panel must not reach the grid, got %q", got)
	}
}

func boolPtr(v bool) *bool { return &v }

// TestMouseDisabledByDefault pins that without the opt-in, mouse events
// do nothing: a click on another card cannot move focus.
func TestMouseDisabledByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := app.New(root, app.Options{})
	m = gridOnly(t, resize(t, m, 120, 30))
	m = loaded(t, m, 1, root, entriesAt(root, 10), nil)

	// Focus the last card via the keyboard, then click the first: the
	// click must be inert.
	next, _ := m.Update(keyMsg("end"))
	m = next.(*app.Model)
	before := m.FocusedPath()

	next, _ = m.Update(click(2, 2))
	m = next.(*app.Model)
	if m.FocusedPath() != before {
		t.Fatalf("mouse must be inert when disabled: focus moved from %q to %q", before, m.FocusedPath())
	}
}
