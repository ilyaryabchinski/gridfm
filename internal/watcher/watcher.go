// Package watcher delivers debounced filesystem change notifications for
// the browsed directory. Events within the debounce window collapse into a
// single notification, so a burst of writes refreshes the view once. The
// watcher never assumes anything about event ordering; the application
// treats each notification as a hint to re-read.
package watcher

import (
	"errors"
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

	mu     sync.Mutex
	path   string
	notify *fsnotify.Watcher

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
// the previous path watching and a retry honestly fails again.
func (w *Watcher) Watch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.path == path {
		return nil
	}

	if err := w.notify.Add(path); err != nil {
		return err
	}

	if w.path != "" {
		_ = w.notify.Remove(w.path)
	}
	w.path = path

	return nil
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
			if w.current() == "" || filepath.Dir(ev.Name) != w.current() && ev.Name != w.current() {
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
