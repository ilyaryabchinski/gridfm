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

// TestWatchSwitchOnAliasKeepsEvents pins the alias contract: switching the
// watch to a symlink of the watched directory reuses the same inotify
// watch, so the switch must not remove it — events keep flowing, and the
// consumer hears about changes under the new spelling.
func TestWatchSwitchOnAliasKeepsEvents(t *testing.T) {
	t.Parallel()

	real := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(real); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(alias); err != nil {
		t.Fatal(err)
	}
	if w.Path() != alias {
		t.Fatalf("path = %q, want %q", w.Path(), alias)
	}

	if err := os.WriteFile(filepath.Join(real, "through-alias"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := expectEvent(t, w)
	if ev.Path != alias {
		t.Fatalf("event path = %q, want the browsed alias %q", ev.Path, alias)
	}

	// Switching back to the real path keeps working too.
	if err := w.Watch(real); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "back-again"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := expectEvent(t, w); ev.Path != real {
		t.Fatalf("event path = %q, want %q", ev.Path, real)
	}
}

// TestWatchSurvivesMultipleAliases pins the registration contract across
// chained aliases: fsnotify keeps labeling events with the path the watch
// was first registered under, so the filter must accept that spelling no
// matter how many aliases the browser has moved through. Moving to an
// unrelated directory afterwards must remove the original watch, not a
// never-registered alias.
func TestWatchSurvivesMultipleAliases(t *testing.T) {
	t.Parallel()

	real := t.TempDir()
	alias1 := filepath.Join(t.TempDir(), "alias1")
	alias2 := filepath.Join(t.TempDir(), "alias2")
	if err := os.Symlink(real, alias1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, alias2); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(real); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(alias1); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(alias2); err != nil {
		t.Fatal(err)
	}

	// fsnotify labels events with the registration spelling (real), so a
	// write must still surface even though only aliases were browsed since.
	if err := os.WriteFile(filepath.Join(real, "deep-alias"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := expectEvent(t, w); ev.Path != alias2 {
		t.Fatalf("event path = %q, want the browsed %q", ev.Path, alias2)
	}

	// The switch away must remove the original registration: afterwards,
	// writes to the abandoned directory produce nothing at all.
	if err := w.Watch(other); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "abandoned"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectSilence(t, w)

	if err := os.WriteFile(filepath.Join(other, "current"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := expectEvent(t, w); ev.Path != other {
		t.Fatalf("event path = %q, want %q", ev.Path, other)
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

// TestFailedWatchDoesNotCommitPath pins the switch contract: a failed Add
// must not claim the path. Otherwise a retry of the same path would
// early-return success while no events ever arrive — the consumer would
// believe it is watching when it is not.
func TestFailedWatchDoesNotCommitPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Watch(filepath.Join(dir, "ghost")); err == nil {
		t.Fatal("watching a missing directory must fail")
	}
	if w.Path() != "" {
		t.Errorf("failed watch committed %q, want no path", w.Path())
	}

	// The retry fails honestly, and a valid path still works afterwards.
	if err := w.Watch(filepath.Join(dir, "ghost")); err == nil {
		t.Fatal("retrying a failed watch must fail again, not lie")
	}
	if err := w.Watch(dir); err != nil {
		t.Fatal(err)
	}
	if w.Path() != dir {
		t.Fatalf("path = %q, want %q", w.Path(), dir)
	}

	if err := os.WriteFile(filepath.Join(dir, "fresh"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := expectEvent(t, w); ev.Path != dir {
		t.Fatalf("event = %+v, want %q", ev, dir)
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

// TestOverflowBecomesChangeHint pins the event-overflow contract: dropped
// kernel events surface as a change notification so the application
// re-reads the directory instead of trusting a stale listing.
func TestOverflowBecomesChangeHint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notify, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	w := newWith(notify, 10*time.Millisecond)
	defer w.Close()
	if err := w.Watch(dir); err != nil {
		t.Fatal(err)
	}

	notify.Errors <- fsnotify.ErrEventOverflow

	if ev := expectEvent(t, w); ev.Err != nil {
		t.Fatalf("overflow should degrade to a change hint, got error %v", ev.Err)
	}
}
