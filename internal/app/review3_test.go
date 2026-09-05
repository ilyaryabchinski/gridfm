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
