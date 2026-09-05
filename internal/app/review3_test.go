package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/preview"
)

// inspectorFixture builds a grid-only model showing one file with the
// inspector open and loaded.
func inspectorFixture(t *testing.T) *app.Model {
	t.Helper()

	root := t.TempDir()
	victim := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(victim, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "notes.txt", Path: victim},
	}, nil)

	m, _ = pressI(t, m)
	return feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: victim,
		Info: &preview.Info{Path: victim, Name: "notes.txt", Size: 2},
	})
}

// TestInspectorClearsWhenFocusedEntryVanishes pins that an applied
// refresh showing the file gone empties the panel: the displayed metadata
// describes something that no longer exists, and no request for it may
// stay pending.
func TestInspectorClearsWhenFocusedEntryVanishes(t *testing.T) {
	t.Parallel()

	m := inspectorFixture(t)
	victim := filepath.Join(m.Path(), "notes.txt")

	next, cmd := m.Update(app.DirectoryLoadedMsg{RequestID: 1, Path: m.Path(), Entries: nil})
	m = next.(*app.Model)
	if cmd != nil {
		for _, out := range runBatch(t, cmd) {
			if _, ok := out.(app.InspectorLoadedMsg); ok {
				t.Error("a vanished focused entry must not be re-requested")
			}
		}
	}
	if m.Inspector() != nil {
		t.Fatalf("panel still shows %+v after the entry was removed", m.Inspector())
	}

	// A late result for the removed file must be rejected, not resurrect
	// the deleted entry's metadata in the panel.
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID() - 1, Path: victim,
		Info: &preview.Info{Path: victim, Name: "notes.txt", Size: 2},
	})
	if m.Inspector() != nil {
		t.Fatalf("a stale result resurrected the removed entry: %+v", m.Inspector())
	}
}

// TestInspectorSuccessClearsPriorError pins that a fresh successful load
// supersedes an earlier failure: the error must stop hiding new metadata.
func TestInspectorSuccessClearsPriorError(t *testing.T) {
	t.Parallel()

	m := inspectorFixture(t)
	victim := filepath.Join(m.Path(), "notes.txt")

	// A refresh whose inspect fails leaves the error state up; the model
	// keeps the old content and the view masks it with the error.
	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 1, Path: m.Path(), Entries: []browser.Entry{
		{Name: "notes.txt", Path: victim},
	}})
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: victim,
		Err: errTestLoad,
	})

	// The next refresh succeeds: the content must show again — a fresh
	// success must supersede the earlier failure.
	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 1, Path: m.Path(), Entries: []browser.Entry{
		{Name: "notes.txt", Path: victim},
	}})
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: victim,
		Info: &preview.Info{Path: victim, Name: "notes.txt", Size: 5},
	})

	if m.Inspector() == nil || m.Inspector().Size != 5 {
		t.Fatalf("panel = %+v, want the fresh metadata", m.Inspector())
	}
}

// TestInspectorTracksWithRefreshedFocus pins the tracked-path contract:
// a refresh that lands focus on a different entry must move
// inspectorPath with the request. Otherwise returning to the previously
// shown file suppresses its inspection and the panel keeps displaying
// the other file.
func TestInspectorTracksWithRefreshedFocus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	// Focus starts at index 0: A.
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "a.txt", Path: a},
		{Name: "b.txt", Path: b},
	}, nil)

	m, _ = pressI(t, m)
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: a,
		Info: &preview.Info{Path: a, Name: "a.txt", Size: 1},
	})

	// Refresh with A gone from the listing: focus index 0 is now B. The
	// background request must retarget the tracked path to B.
	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: []browser.Entry{
		{Name: "b.txt", Path: b},
	}})
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: b,
		Info: &preview.Info{Path: b, Name: "b.txt", Size: 7},
	})
	if m.Inspector() == nil || m.Inspector().Size != 7 {
		t.Fatalf("panel = %+v, want B's fresh metadata", m.Inspector())
	}

	// A comes back. Focus preservation keeps focus on B (the previously
	// focused path), so move back to A: the panel must follow with a
	// fresh request for A — inspectorPath must have moved to B during
	// the refresh, otherwise this movement is suppressed as "already
	// showing A".
	next, cmd := m.Update(app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: []browser.Entry{
		{Name: "a.txt", Path: a},
		{Name: "b.txt", Path: b},
	}})
	m = next.(*app.Model)

	next, cmd = m.Update(keyMsg("h")) // focus moves left, back to a.txt
	var asked bool
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.InspectorLoadedMsg); ok && loaded.Path == a {
			asked = true
		}
	}
	if !asked {
		t.Fatal("returning to a previously shown file must re-request its metadata")
	}
}
