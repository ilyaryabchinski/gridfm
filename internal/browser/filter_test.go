package browser_test

import (
	"testing"

	"gridfm/internal/browser"
)

func sampleEntries() []browser.Entry {
	return []browser.Entry{
		{Name: "alpha", Path: "/d/alpha"},
		{Name: ".hidden", Path: "/d/.hidden"},
		{Name: "beta.log", Path: "/d/beta.log"},
		{Name: ".hmirror", Path: "/d/.hmirror"},
		{Name: "gamma", Path: "/d/gamma"},
	}
}

func TestFilterHidesDotfilesByDefault(t *testing.T) {
	t.Parallel()

	visible := browser.FilterEntries(sampleEntries(), false, "")
	if len(visible) != 3 {
		t.Fatalf("visible = %d entries, want 3 (hidden excluded)", len(visible))
	}
	for _, e := range visible {
		if len(e.Name) > 0 && e.Name[0] == '.' {
			t.Errorf("hidden entry %q should be excluded", e.Name)
		}
	}
}

func TestFilterShowsDotfilesWhenRequested(t *testing.T) {
	t.Parallel()

	visible := browser.FilterEntries(sampleEntries(), true, "")
	if len(visible) != 5 {
		t.Fatalf("visible = %d entries, want 5", len(visible))
	}
}

func TestFilterIsCaseInsensitiveSubstring(t *testing.T) {
	t.Parallel()

	visible := browser.FilterEntries(sampleEntries(), true, "MIR")
	if len(visible) != 1 || visible[0].Name != ".hmirror" {
		t.Errorf("query %q matched %v, want .hmirror", "MIR", names(visible))
	}

	if got := browser.FilterEntries(sampleEntries(), true, "zzz"); len(got) != 0 {
		t.Errorf("non-matching query should hide everything, got %v", names(got))
	}

	if got := browser.FilterEntries(sampleEntries(), false, ""); len(got) != 3 {
		t.Errorf("empty query should show all non-hidden, got %v", names(got))
	}
}

func TestFilterCombinesWithHiddenRule(t *testing.T) {
	t.Parallel()

	// "h" matches alpha, .hidden, .hmirror, beta.log? no: beta.log lacks h...
	// alpha, .hidden, .hmirror match; without hidden only alpha survives.
	visible := browser.FilterEntries(sampleEntries(), false, "h")
	if len(visible) != 1 || visible[0].Name != "alpha" {
		t.Errorf("combined filter matched %v, want [alpha]", names(visible))
	}
}

func TestSetFilterPreservesFocusByIdentity(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "alpha", Path: "/d/alpha"},
		{Name: "beta", Path: "/d/beta"},
		{Name: "gamma", Path: "/d/gamma"},
	})
	b.SetFocusIndex(2)

	b.SetFilter("gam")
	visible := b.Visible()
	if len(visible) != 1 || visible[0].Name != "gamma" {
		t.Fatalf("filtered visible = %v, want [gamma]", names(visible))
	}
	if got := b.FocusedPath(); got != "/d/gamma" {
		t.Errorf("FocusedPath after filter = %q, want /d/gamma", got)
	}

	// Clearing the filter restores the same focused entry.
	b.SetFilter("")
	if got := b.FocusedPath(); got != "/d/gamma" {
		t.Errorf("FocusedPath after clearing filter = %q, want /d/gamma", got)
	}
}

func TestSetFilterFallsBackToNearestWhenFocusVanishes(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "alpha", Path: "/d/alpha"},
		{Name: "beta", Path: "/d/beta"},
		{Name: "gamma", Path: "/d/gamma"},
	})
	b.SetFocusIndex(2) // gamma

	b.SetFilter("alp")
	if got := b.FocusedPath(); got != "/d/alpha" {
		t.Errorf("FocusedPath = %q, want nearest survivor /d/alpha", got)
	}
}

func TestHiddenToggleAndSortPreserveVisibleFocus(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "zeta", Path: "/d/zeta"},
		{Name: ".dot", Path: "/d/.dot"},
		{Name: "alpha", Path: "/d/alpha"},
	})
	b.SetFocusIndex(0) // alpha (sorted first among visible)

	b.SetShowHidden(true)
	visible := b.Visible()
	if len(visible) != 3 {
		t.Fatalf("visible after showing hidden = %d, want 3", len(visible))
	}
	if got := b.FocusedPath(); got != "/d/alpha" {
		t.Errorf("FocusedPath after hidden toggle = %q, want /d/alpha", got)
	}

	b.SetSort(browser.SortBySize, browser.SortDescending)
	if got := b.FocusedPath(); got != "/d/alpha" {
		t.Errorf("FocusedPath after sort change = %q, want /d/alpha", got)
	}
}

func names(entries []browser.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}

	return out
}
