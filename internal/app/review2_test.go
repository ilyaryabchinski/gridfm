package app_test

import (
	"os"
	"path/filepath"
	"testing"

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

// TestInspectorReloadsAfterAppliedRefresh pins that any applied refresh
// re-requests the focused entry's metadata — an edit, a chmod/chown, or a
// replacement preserving mtime all change fields the panel shows — and
// that the fresh result replaces the panel content.
func TestInspectorReloadsAfterAppliedRefresh(t *testing.T) {
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

	// An applied refresh re-requests the focused entry, keeping the old
	// content visible until the fresh result lands.
	entries := []browser.Entry{{Name: "notes.txt", Path: victim}}
	next, cmd := m.Update(app.DirectoryLoadedMsg{RequestID: 1, Path: root, Entries: entries})
	m = next.(*app.Model)
	var reloaded bool
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.InspectorLoadedMsg); ok && loaded.Path == victim {
			reloaded = true
		}
	}
	if !reloaded {
		t.Fatal("an applied refresh must re-request the focused inspector")
	}
	if m.Inspector() == nil || m.Inspector().Size != 2 {
		t.Fatal("the panel should keep its content while reloading")
	}

	// The fresh result replaces the content.
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: victim,
		Info: &preview.Info{Path: victim, Name: "notes.txt", Size: 5},
	})
	if m.Inspector() == nil || m.Inspector().Size != 5 {
		t.Fatalf("panel = %+v, want the fresh size 5", m.Inspector())
	}
}

// TestRejectedLoadsLeaveInspectorAlone pins the stale-result gate: a
// rejected directory load carries no information about the focused entry,
// so it must not trigger a request or clear the panel.
func TestRejectedLoadsLeaveInspectorAlone(t *testing.T) {
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

	// A stale result, and a failed one, must leave the panel untouched.
	for _, msg := range []app.DirectoryLoadedMsg{
		{RequestID: 99, Path: root, Entries: nil},
		{RequestID: 1, Path: root, Err: errTestLoad},
	} {
		next, cmd := m.Update(msg)
		m = next.(*app.Model)
		for _, out := range runBatch(t, cmd) {
			if _, ok := out.(app.InspectorLoadedMsg); ok {
				t.Errorf("a rejected load (%+v) must not re-request the inspector", msg)
			}
		}
		if m.Inspector() == nil || m.Inspector().Size != 2 {
			t.Fatalf("rejected load (%+v) cleared the panel: %+v", msg, m.Inspector())
		}
	}
}
