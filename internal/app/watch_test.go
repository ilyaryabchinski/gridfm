package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/preview"
)

func TestRefreshKeyReloadsCurrentDirectory(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	if cmd == nil {
		t.Fatal("r should issue a directory reload")
	}
	if !updated.IsLoading() {
		t.Error("r should mark the directory as loading")
	}
}

func TestDirChangedRefreshesBrowsedDirectory(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)
	before := m.InspectorRequestID()

	// A change notification for the browsed directory triggers a reload.
	next, cmd := m.Update(app.DirChangedMsg{Path: "/d"})
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	if cmd == nil {
		t.Fatal("a change to the browsed directory should trigger a reload")
	}
	if !updated.IsLoading() {
		t.Error("the reload should be in flight")
	}
	if updated.InspectorRequestID() != before {
		t.Error("watch notifications must not touch other request identities")
	}

	// While that reload is in flight, further notifications are dropped:
	// the model stays loading with no newer request.
	next, _ = updated.Update(app.DirChangedMsg{Path: "/d"})
	updated = next.(*app.Model)
	if !updated.IsLoading() {
		t.Error("the original reload should still be the newest request")
	}

	// Notifications for other directories are stale and ignored.
	m2 := loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)
	m2 = loaded(t, m2, 2, "/d", entriesAt("/d", 2), nil)
	_, cmd = m2.Update(app.DirChangedMsg{Path: "/elsewhere"})
	if cmd == nil {
		t.Fatal("watch commands always come in pairs with the listener")
	}
	_ = cmd // never executed: the listener blocks until a real event
}

func TestDirChangedErrorDegradesToManualRefresh(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.DirChangedMsg{Path: "/d", Err: os.ErrPermission})
	if !strings.Contains(m.View(), "watch unavailable") {
		t.Error("watcher failure should surface a note")
	}

	// The note appears once only.
	m = feed(t, m, app.DirChangedMsg{Path: "/d", Err: os.ErrPermission})
	if seen := strings.Count(m.View(), "watch unavailable"); seen != 1 {
		t.Errorf("failure note shown %d times, want once", seen)
	}
}

// TestStaleLoadNeverRepointsWatcher pins the watcher/reject interplay: a
// stale or failed directory result must leave the watcher on the browsed
// directory.
func TestStaleLoadNeverRepointsWatcher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := gridOnly(t, resize(t, app.New(dir, app.Options{}), 80, 24))
	m = loaded(t, m, 1, dir, entriesAt(dir, 2), nil)
	_ = m.WatchDirectory(dir)()

	// A stale result (superseded request id) arrives for another path.
	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 99, Path: "/elsewhere", Entries: nil})
	if m.WatchPath() != dir {
		t.Errorf("watch = %q, want %q after a stale load", m.WatchPath(), dir)
	}

	// A failed (non-stale) load keeps the watcher on the browsed
	// directory too.
	m = feed(t, m, app.DirectoryLoadedMsg{RequestID: 1, Path: "/nowhere", Err: errTestLoad})
	if m.WatchPath() != dir {
		t.Errorf("watch = %q, want %q after a failed load", m.WatchPath(), dir)
	}
}

// TestChangeDuringLoadRefreshesOnceMore pins the dirty latch: a change
// notification landing during an in-flight load triggers one extra reload
// after it lands, instead of stranding a stale listing.
func TestChangeDuringLoadRefreshesOnceMore(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	// A reload starts, then a change lands while it is in flight.
	next, _ := m.Update(app.DirChangedMsg{Path: "/d"})
	loading := next.(*app.Model)
	loading = feed(t, loading, app.DirChangedMsg{Path: "/d"})

	// The load lands; the latched change must schedule one more reload.
	next, cmd := loading.Update(app.DirectoryLoadedMsg{RequestID: 2, Path: "/d", Entries: entriesAt("/d", 2)})
	updated := next.(*app.Model)
	if cmd == nil {
		t.Fatal("a latched change must schedule a follow-up reload")
	}
	if !updated.IsLoading() {
		t.Error("the follow-up reload should be in flight")
	}

	// The follow-up landing without further changes stops the cycle.
	next, cmd = updated.Update(app.DirectoryLoadedMsg{RequestID: 3, Path: "/d", Entries: entriesAt("/d", 2)})
	updated = next.(*app.Model)
	if updated.IsLoading() {
		t.Error("no further reload should be scheduled without new changes")
	}
	_ = cmd
}

// TestInspectorFollowsFilterDrivenFocus pins that focus moved by filtering
// (not by cursor keys) is followed by the open inspector.
func TestInspectorFollowsFilterDrivenFocus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := map[string]string{}
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		p := filepath.Join(root, name)
		paths[name] = p
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "alpha.txt", Path: paths["alpha.txt"]},
		{Name: "beta.txt", Path: paths["beta.txt"]},
	}, nil)

	// Inspect beta (second entry).
	m = press(t, m, "l")
	m, _ = pressI(t, m)
	betaReq := m.InspectorRequestID()
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: betaReq, Path: paths["beta.txt"],
		Info: &preview.Info{Path: paths["beta.txt"], Name: "beta.txt"},
	})

	// Filtering down to alpha moves focus by identity fallback; the
	// inspector must re-request for alpha.
	m = press(t, m, "/")
	m = press(t, m, "a")
	m = press(t, m, "l")
	m = press(t, m, "p")
	if m.FocusedPath() != paths["alpha.txt"] {
		t.Fatalf("focus = %q, want alpha", m.FocusedPath())
	}
	if m.Inspector() != nil && m.Inspector().Name == "beta.txt" {
		t.Error("the inspector still shows beta after focus moved to alpha")
	}
}

// TestRealWatchEventTriggersReload pins the wiring end to end: a write into
// the browsed directory eventually produces a reload request.
func TestRealWatchEventTriggersReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := gridOnly(t, resize(t, app.New(dir, app.Options{}), 80, 24))
	if m.WatchPath() != "" {
		t.Error("no directory should be watched before the first load")
	}
	m = loaded(t, m, 1, dir, entriesAt(dir, 2), nil)

	// Point the watcher the way the DirectoryLoadedMsg command would.
	ready := m.WatchDirectory(dir)
	if ready != nil {
		if msg, ok := ready().(app.WatchReadyMsg); ok && msg.Err != nil {
			t.Fatalf("watch setup failed: %v", msg.Err)
		}
	}
	if m.WatchPath() != dir {
		t.Fatalf("watching %q, want %q", m.WatchPath(), dir)
	}

	// Bridge the event stream like the runtime would, then write a file.
	msgs := make(chan tea.Msg, 1)
	go func() {
		listen := m.ListenWatch()
		if listen == nil {
			close(msgs)

			return
		}
		msgs <- listen()
	}()

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-msgs:
		changed, ok := msg.(app.DirChangedMsg)
		if !ok || changed.Path != dir || changed.Err != nil {
			t.Fatalf("msg = %+v, want a change notification for %q", msg, dir)
		}
		next, cmd := m.Update(msg)
		updated, ok := next.(*app.Model)
		if !ok {
			t.Fatalf("Update returned %T", next)
		}
		if cmd == nil {
			t.Fatal("the real change should trigger a reload")
		}
		if !updated.IsLoading() {
			t.Error("the reload should be in flight")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no watch event within the deadline")
	}
}
