package browser_test

import (
	"testing"

	"gridfm/internal/browser"
)

func TestHistoryRecordsConfirmedNavigation(t *testing.T) {
	t.Parallel()

	b := browser.New("/root")
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})

	b.SetEntries("/root/sub", []browser.Entry{{Name: "y", Path: "/root/sub/y"}})
	b.SetEntries("/root/other", []browser.Entry{{Name: "z", Path: "/root/other/z"}})

	if !b.CanBack() {
		t.Fatal("two navigations deep should allow going back")
	}
	if b.CanForward() {
		t.Error("nothing to forward to at the newest location")
	}

	got, ok := b.Back()
	if !ok || got != "/root/sub" {
		t.Fatalf("Back() = %q, %v; want /root/sub, true", got, ok)
	}

	// Confirming the back move loads the same path: no duplicate push.
	b.SetEntries("/root/sub", []browser.Entry{{Name: "y", Path: "/root/sub/y"}})
	if !b.CanForward() {
		t.Error("forward should be available after a confirmed back")
	}

	got, ok = b.Forward()
	if !ok || got != "/root/other" {
		t.Fatalf("Forward() = %q, %v; want /root/other, true", got, ok)
	}
}

func TestHistoryNewNavigationDropsForwardEntries(t *testing.T) {
	t.Parallel()

	b := browser.New("/root")
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})
	b.SetEntries("/root/sub", []browser.Entry{{Name: "y", Path: "/root/sub/y"}})

	// A confirmed back move, then fresh navigation: the forward entry that
	// sat ahead of /root is discarded.
	got, ok := b.Back()
	if !ok || got != "/root" {
		t.Fatalf("Back() = %q, %v; want /root, true", got, ok)
	}
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})

	b.SetEntries("/elsewhere", []browser.Entry{{Name: "e", Path: "/elsewhere/e"}})
	if b.CanForward() {
		t.Error("forward stack should be dropped after new navigation")
	}
	if got, ok := b.Back(); !ok || got != "/root" {
		t.Errorf("Back after new navigation = %q, %v; want /root, true", got, ok)
	}
}

func TestHistoryAbandonedBackKeepsCursorConsistent(t *testing.T) {
	t.Parallel()

	b := browser.New("/root")
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})
	b.SetEntries("/root/sub", []browser.Entry{{Name: "y", Path: "/root/sub/y"}})

	// Back starts a traversal, but the load fails and the step is
	// cancelled: the cursor stays on /root/sub and the traversal can be
	// retried.
	got, ok := b.Back()
	if !ok || got != "/root" {
		t.Fatalf("Back() = %q, %v; want /root, true", got, ok)
	}
	b.CancelHistoryStep()

	if !b.CanBack() {
		t.Error("back should still be available after a cancelled step")
	}
	got, ok = b.Back()
	if !ok || got != "/root" {
		t.Errorf("retry Back() = %q, %v; want /root, true", got, ok)
	}

	// A mismatched load while a step is in flight treats it as abandoned:
	// navigating elsewhere from /root/sub makes /elsewhere the newest.
	b.SetEntries("/elsewhere", []browser.Entry{{Name: "e", Path: "/elsewhere/e"}})
	if b.CanForward() {
		t.Error("abandoned step must not preserve a forward entry")
	}
}

func TestHistoryRefreshDoesNotPush(t *testing.T) {
	t.Parallel()

	b := browser.New("/root")
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})

	// A refresh (same path) must not grow the history.
	b.SetEntries("/root", []browser.Entry{{Name: "x2", Path: "/root/x2"}})

	if b.CanBack() {
		t.Error("refresh should not create history entries")
	}
}

func TestHistoryBoundsAtOldestEntry(t *testing.T) {
	t.Parallel()

	b := browser.New("/root")
	b.SetEntries("/root", []browser.Entry{{Name: "x", Path: "/root/x"}})
	b.SetEntries("/root/sub", []browser.Entry{{Name: "y", Path: "/root/sub/y"}})

	if _, ok := b.Back(); !ok {
		t.Fatal("Back should be available")
	}
	if _, ok := b.Back(); ok {
		t.Error("Back past the oldest location should be unavailable")
	}
}
