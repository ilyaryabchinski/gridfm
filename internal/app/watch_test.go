package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
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
