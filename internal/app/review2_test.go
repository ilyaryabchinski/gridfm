package app_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/preview"
)

// runBatch executes a command, flattening batches into their messages.
func runBatch(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}

	var msgs []tea.Msg
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			msgs = append(msgs, c())
		}
	default:
		msgs = append(msgs, msg)
	}

	return msgs
}

// TestSidebarToggleResyncsGridGeometry pins that panel toggles update the
// navigation grid: after hiding the sidebar the columns grow from five to
// six, so the next vertical move must cover six entries.
func TestSidebarToggleResyncsGridGeometry(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 100, 30)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 12), nil)

	// Docked sidebar: five columns. One move down lands five entries on.
	m = press(t, m, "j")
	if got := m.FocusedPath(); filepath.Base(got) != "entry-05.txt" {
		t.Fatalf("focus after first move = %q, want entry-05", got)
	}

	// Hide the sidebar: six columns now render. The next move must cover
	// six entries, which requires the grid to know the new geometry.
	m = press(t, m, "~")
	m = press(t, m, "j")
	if got := m.FocusedPath(); filepath.Base(got) != "entry-11.txt" {
		t.Fatalf("focus after toggle+move = %q, want entry-11 (six-column move)", got)
	}

	// Restoring the sidebar returns to five columns; from the last entry
	// there is nowhere further to go.
	m = press(t, m, "~")
	m = press(t, m, "j")
	if got := m.FocusedPath(); filepath.Base(got) != "entry-11.txt" {
		t.Fatalf("focus after restore+move = %q, want entry-11", got)
	}
}

// TestInspectorToggleResyncsGridGeometry pins the inspector's symmetric
// case: docking the panel shrinks the grid to three columns at this width
// and the navigation must follow.
func TestInspectorToggleResyncsGridGeometry(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 100, 30))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 12), nil)

	// No sidebar, no inspector: six columns.
	m = press(t, m, "j")
	if got := m.FocusedPath(); filepath.Base(got) != "entry-06.txt" {
		t.Fatalf("focus after first move = %q, want entry-06", got)
	}

	// Dock the inspector: the grid shrinks to four columns.
	m, _ = pressI(t, m)
	m = press(t, m, "j")
	if got := m.FocusedPath(); filepath.Base(got) != "entry-10.txt" {
		t.Fatalf("focus after inspector+move = %q, want entry-10 (four-column move)", got)
	}
}

// TestInspectorReloadsWhenFocusedFileChanges pins that an applied refresh
// reloads the panel when the focused file changed on disk under the same
// name; an unchanged file must not re-request.
func TestInspectorReloadsWhenFocusedFileChanges(t *testing.T) {
	t.Parallel()

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
	req1 := m.InspectorRequestID()
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: req1, Path: victim,
		Info: &preview.Info{Path: victim, Name: "notes.txt", Size: 2},
	})

	// A refresh with an unchanged file: no inspector request in the batch.
	entries := []browser.Entry{{Name: "notes.txt", Path: victim}}
	next, cmd := m.Update(app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: entries})
	m = next.(*app.Model)
	for _, msg := range runBatch(t, cmd) {
		if _, ok := msg.(app.InspectorLoadedMsg); ok {
			t.Error("an unchanged focused file must not re-request metadata")
		}
	}
	if m.Inspector() == nil || m.Inspector().Size != 2 {
		t.Fatal("the panel should keep its content for an unchanged file")
	}

	// The file is edited on disk: the next refresh must re-request.
	newer := time.Now().Add(2 * time.Second).Truncate(time.Second)
	if err := os.Chtimes(victim, newer, newer); err != nil {
		t.Fatal(err)
	}
	changed := []browser.Entry{{Name: "notes.txt", Path: victim, ModTime: newer, Size: 5}}
	_, cmd = m.Update(app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: changed})

	var reloaded bool
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.InspectorLoadedMsg); ok {
			reloaded = true
			if loaded.Path != victim {
				t.Errorf("reload path = %q, want %q", loaded.Path, victim)
			}
		}
	}
	if !reloaded {
		t.Fatal("a changed focused file must trigger an inspector reload")
	}
}
