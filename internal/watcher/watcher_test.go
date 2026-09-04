package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// expectEvent reads one event within the deadline and fails otherwise.
func expectEvent(t *testing.T, w *Watcher) Event {
	t.Helper()

	select {
	case ev := <-w.Events():
		return ev
	case <-time.After(3 * time.Second):
		t.Fatal("no watcher event within the deadline")

		return Event{}
	}
}

// expectSilence asserts no event arrives within the window.
func expectSilence(t *testing.T, w *Watcher) {
	t.Helper()

	select {
	case ev := <-w.Events():
		t.Fatalf("unexpected event %+v", ev)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestDebounceCollapsesBurstIntoOneEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(dir); err != nil {
		t.Fatal(err)
	}

	for i := range 5 {
		if err := os.WriteFile(filepath.Join(dir, "f"+string(rune('a'+i))), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ev := expectEvent(t, w)
	if ev.Path != dir || ev.Err != nil {
		t.Fatalf("event = %+v, want path %q", ev, dir)
	}
	expectSilence(t, w)
}

func TestWatchSwitchDropsOldDirectory(t *testing.T) {
	t.Parallel()

	old := t.TempDir()
	cur := t.TempDir()

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(old); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(cur); err != nil {
		t.Fatal(err)
	}
	if w.Path() != cur {
		t.Fatalf("path = %q, want %q", w.Path(), cur)
	}

	// Writes to the abandoned directory produce nothing.
	if err := os.WriteFile(filepath.Join(old, "stale"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectSilence(t, w)

	// Writes to the current directory do.
	if err := os.WriteFile(filepath.Join(cur, "fresh"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := expectEvent(t, w); ev.Path != cur {
		t.Fatalf("event = %+v, want %q", ev, cur)
	}
}

func TestSameWatchIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(dir); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(dir); err != nil {
		t.Fatal(err)
	}
}

// TestCloseStopsEventStream pins the shutdown contract: the events channel
// closes when the watcher stops.
func TestCloseStopsEventStream(t *testing.T) {
	t.Parallel()

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case _, ok := <-w.Events():
		if ok {
			t.Error("events should be closed after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("the event stream never closed")
	}
}

// TestErrorSurfaced pins that watcher failures reach the consumer instead
// of vanishing.
func TestErrorSurfaced(t *testing.T) {
	t.Parallel()

	notify, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	w := newWith(notify, 10*time.Millisecond)
	defer w.Close()
	if err := w.Watch(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	notify.Errors <- os.ErrPermission

	ev := expectEvent(t, w)
	if ev.Err == nil {
		t.Fatal("the failure should carry the error")
	}
}
