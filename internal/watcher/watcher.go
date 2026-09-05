// Package watcher delivers debounced filesystem change notifications for
// the browsed directory. Events within the debounce window collapse into a
// single notification, so a burst of writes refreshes the view once. The
// watcher never assumes anything about event ordering; the application
// treats each notification as a hint to re-read.
package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDelay is the debounce window applied to raw filesystem events.
const DefaultDelay = 150 * time.Millisecond

// Event is one debounced change notification for the watched directory, or
// a watcher failure. Path is the watched directory, not the changed entry.
type Event struct {
	Path string
	Err  error
}

// Watcher watches one directory at a time. Watch switches directories; all
// methods are safe from any goroutine.
type Watcher struct {
	delay time.Duration

	mu sync.Mutex
	// path is the browsed directory the application sees; registered is
	// the spelling the inotify watch was actually registered under. They
	// diverge when the watch is reached through directory aliases, because
	// fsnotify keeps labeling events with the registered path.
	path       string
	registered string
	notify     *fsnotify.Watcher

	events chan Event
	stop   chan struct{}
}

// New starts a watcher with the default debounce delay. Consume Events in a
// dedicated goroutine; the watcher stops when Close is called.
func New() (*Watcher, error) {
	notify, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return newWith(notify, DefaultDelay), nil
}

// newWith wraps an existing fsnotify watcher; the test seam.
func newWith(notify *fsnotify.Watcher, delay time.Duration) *Watcher {
	w := &Watcher{
		delay:  delay,
		notify: notify,
		events: make(chan Event),
		stop:   make(chan struct{}),
	}

	go w.loop()

	return w
}

// Events delivers debounced change notifications and watcher failures. The
// channel closes when the watcher stops.
func (w *Watcher) Events() <-chan Event { return w.events }

// Watch switches the watch to path. Events outside the current path are
// dropped, and the switch itself never produces a notification. The switch
// commits only after the underlying watch succeeds, so a failed Add leaves
// the previous path watching and a retry honestly fails again. Switching
// between aliases of the same directory keeps the existing registration:
// re-adding would share or duplicate the inotify watch, and removing the
// registered path would kill it, so only the browsed spelling moves.
func (w *Watcher) Watch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.path == path {
		return nil
	}

	if w.registered != "" && sameDirectory(w.registered, path) {
		w.path = path

		return nil
	}

	if err := w.notify.Add(path); err != nil {
		return err
	}

	if w.registered != "" {
		_ = w.notify.Remove(w.registered)
	}
	w.registered = path
	w.path = path

	return nil
}

// sameDirectory reports whether two paths name the same directory,
// following symlinks on both sides.
func sameDirectory(a, b string) bool {
	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(b)
	if err != nil {
		return false
	}

	return os.SameFile(infoA, infoB)
}

// Path returns the currently watched directory, empty when none.
func (w *Watcher) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.path
}

// Close stops the watcher and releases its resources.
func (w *Watcher) Close() error {
	close(w.stop)

	return w.notify.Close()
}

// loop fans raw fsnotify events through the debounce window. The timer is
// restarted by every event, so a burst collapses into one notification.
func (w *Watcher) loop() {
	defer close(w.events)

	var debounce <-chan time.Time
	for {
		select {
		case <-w.stop:
			return

		case ev, ok := <-w.notify.Events:
			if !ok {
				return
			}
			if !w.watches(ev.Name) {
				continue
			}
			// Restart the window: only silence publishes.
			debounce = time.After(w.delay)

		case err, ok := <-w.notify.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				// The kernel dropped events: silence would strand the
				// consumer with a stale listing. Surface it as a change
				// hint so the application re-reads the directory.
				debounce = time.After(w.delay)

				continue
			}
			if !w.send(Event{Path: w.current(), Err: err}) {
				return
			}

		case <-debounce:
			debounce = nil
			if !w.send(Event{Path: w.current()}) {
				return
			}
		}
	}
}

// current snapshots the watched path.
func (w *Watcher) current() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.path
}

// watches reports whether a raw event's path belongs to the watched
// directory. fsnotify labels events with the path the watch was registered
// under, which diverges from the browsed path when the directory is
// reached through aliases, so both spellings are accepted.
func (w *Watcher) watches(name string) bool {
	cur := w.current()
	if cur == "" {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	dir := filepath.Dir(name)
	for _, p := range []string{w.registered, w.path} {
		if p == "" {
			continue
		}
		if dir == p || name == p {
			return true
		}
	}

	return false
}

// send delivers one event unless the watcher stopped or the consumer went
// away.
func (w *Watcher) send(ev Event) bool {
	select {
	case w.events <- ev:
		return true
	case <-w.stop:
		return false
	}
}
