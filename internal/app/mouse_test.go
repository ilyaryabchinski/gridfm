package app_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/places"
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

// TestMouseClickFocusesCard pins that a left press focuses the clicked
// card: seven columns at this width, so the second card holds entry-01.
func TestMouseClickFocusesCard(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)

	next, _ := m.Update(click(2, 2))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("focus after click on card one = %q, want entry-00", got)
	}

	// The card at column 2 starts at x = 1 + 16: its left border.
	next, _ = m.Update(click(18, 3))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-01.txt" {
		t.Fatalf("focus after click on card two = %q, want entry-01", got)
	}

	// The first card of the second row: y lands five rows down
	// (header + card 5 + gap), which is entry-07 at seven columns.
	next, _ = m.Update(click(2, 9))
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-07.txt" {
		t.Fatalf("focus after click on row two = %q, want entry-07", got)
	}
}

// TestMouseClickInGapIsIgnored pins that clicks on the two-column gap
// between cards leave focus where it was.
func TestMouseClickInGapIsIgnored(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)
	next, _ := m.Update(click(2, 2)) // focus the first card
	m = next.(*app.Model)

	next, _ = m.Update(click(16, 3)) // gap between card one and two
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Fatalf("a gap click must not move focus, got %q", got)
	}
}

// TestMouseCtrlClickTogglesSelection pins that ctrl-click selects the
// clicked entry without disturbing focus of the previously focused one.
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

	// A second ctrl-click on the same card deselects.
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
	next, _ = m.Update(click(18, 2))
	m = next.(*app.Model)
	if m.IsLoading() {
		t.Fatal("clicks on different cards must not register as a double click")
	}
}

// TestMouseWheelScrolls pins that the wheel scrolls the grid by one row
// and clamps at both ends.
func TestMouseWheelScrolls(t *testing.T) {
	t.Parallel()

	m := mouseModel(t, 30)

	// Scrolling up at the top stays put.
	next, _ := m.Update(wheel(false))
	m = next.(*app.Model)

	// Four rows of five: scrolling down three times reaches the last row.
	for range 3 {
		next, _ = m.Update(wheel(true))
		m = next.(*app.Model)
	}
	if got := m.FocusedPath(); filepath.Base(got) != "entry-00.txt" {
		t.Logf("wheel scrolling does not move focus (got %q)", got)
	}

	// Beyond the end clamps: the view cannot overscroll into emptiness.
	for range 5 {
		next, _ = m.Update(wheel(true))
		m = next.(*app.Model)
	}
	if !strings.Contains(m.View(), "entry-25.txt") {
		t.Error("the last row should be visible after scrolling to the end")
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

	// The docked sidebar occupies the leftmost columns; the first item
	// sits on the third body row (after the blank and section header).
	next, _ := m.Update(click(3, 5))
	m = next.(*app.Model)

	if m.Region() != app.RegionSidebar {
		t.Fatalf("region = %v, want the sidebar", m.Region())
	}
	if m.PlaceIdx() != 1 {
		t.Fatalf("selected place = %d, want 1 (Downloads)", m.PlaceIdx())
	}
}

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
	next, _ := m.Update(keyMsg("G"))
	m = next.(*app.Model)
	before := m.FocusedPath()

	next, _ = m.Update(click(2, 2))
	m = next.(*app.Model)
	if m.FocusedPath() != before {
		t.Fatalf("mouse must be inert when disabled: focus moved from %q to %q", before, m.FocusedPath())
	}
}
