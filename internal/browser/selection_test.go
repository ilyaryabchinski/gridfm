package browser_test

import (
	"testing"

	"gridfm/internal/browser"
)

func TestSelectionToggleAndCount(t *testing.T) {
	t.Parallel()

	s := browser.NewSelection()
	if s.Len() != 0 {
		t.Fatal("fresh selection should be empty")
	}

	if !s.Toggle("/d/a") {
		t.Error("first toggle should select")
	}
	if s.Toggle("/d/a") {
		t.Error("second toggle should deselect")
	}
	if s.Len() != 0 {
		t.Errorf("Len after toggling off = %d, want 0", s.Len())
	}

	s.Set("/d/b", true)
	if !s.Has("/d/b") {
		t.Error("Set should select")
	}
	s.Set("/d/b", false)
	if s.Has("/d/b") {
		t.Error("Set(false) should deselect")
	}
}

func TestSelectionPathsAreSorted(t *testing.T) {
	t.Parallel()

	s := browser.NewSelection()
	s.Toggle("/d/c")
	s.Toggle("/d/a")
	s.Toggle("/d/b")

	got := s.Paths()
	want := []string{"/d/a", "/d/b", "/d/c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Paths()[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestSelectionSurvivesDirectoryNavigation(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})
	b.ToggleFocused() // selects /d/a

	// Navigate away and back: the selection persists for multi-directory
	// operations.
	b.SetEntries("/other", []browser.Entry{{Name: "x", Path: "/other/x"}})
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})

	if got := b.SelectedCount(); got != 1 {
		t.Fatalf("SelectedCount after round trip = %d, want 1", got)
	}
	if got := b.SelectedPaths()[0]; got != "/d/a" {
		t.Errorf("SelectedPaths[0] = %q, want /d/a", got)
	}
}

func TestSelectionClear(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})
	b.SelectAllVisible()
	if b.SelectedCount() != 2 {
		t.Fatalf("SelectedCount after select-all = %d, want 2", b.SelectedCount())
	}

	b.ClearSelection()
	if b.SelectedCount() != 0 {
		t.Errorf("SelectedCount after clear = %d, want 0", b.SelectedCount())
	}
}

func TestSelectRangeMarksInclusiveSpan(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	entries := []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
		{Name: "c", Path: "/d/c"},
		{Name: "e", Path: "/d/e"},
	}
	b.SetEntries("/d", entries)

	b.SelectRange(0, 2)
	if b.SelectedCount() != 3 {
		t.Fatalf("SelectedCount after range = %d, want 3", b.SelectedCount())
	}

	// Ranges work in both directions.
	b.ClearSelection()
	b.SelectRange(2, 3)
	if !b.Selection().Has("/d/c") || !b.Selection().Has("/d/e") {
		t.Errorf("reversed range should select c and e")
	}
}

func TestFocusedSelectedTracksToggle(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})

	if b.FocusedSelected() {
		t.Fatal("nothing should be selected initially")
	}
	b.ToggleFocused()
	if !b.FocusedSelected() {
		t.Error("focused entry should be selected after toggle")
	}
	b.Down()
	if b.FocusedSelected() {
		t.Error("the next entry should not inherit selection state")
	}
}
