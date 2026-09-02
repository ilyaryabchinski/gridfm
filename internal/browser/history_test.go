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

	if _, ok := b.Back(); !ok {
		t.Fatal("Back should be available")
	}

	// Navigating somewhere new from the past discards the forward entry.
	b.SetEntries("/elsewhere", []browser.Entry{{Name: "e", Path: "/elsewhere/e"}})
	if b.CanForward() {
		t.Error("forward stack should be dropped after new navigation")
	}
	if got, ok := b.Back(); !ok || got != "/root" {
		t.Errorf("Back after new navigation = %q, %v; want /root, true", got, ok)
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
