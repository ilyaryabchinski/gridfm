package app_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
)

// TestHelpOverlayOpensClosesAndBlocks pins the legend behavior: ? opens a
// blocking help overlay, esc/? close it, and grid shortcuts stay inert
// while it is up.
func TestHelpOverlayOpensClosesAndBlocks(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	// ? opens the legend.
	m = press(t, m, "?")
	view := m.View()
	for _, want := range []string{"keys", "space", "filter", "rename", "trash", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("help should list %q, got:\n%s", want, view)
		}
	}
	// The frame must not grow beyond the terminal: 24 rows means 23 newlines.
	if lines := strings.Count(view, "\n"); lines != 23 {
		t.Errorf("help frame renders %d rows, want exactly 24", lines+1)
	}

	// Grid shortcuts must not fire behind the overlay: d opens no trash
	// confirmation, s opens no sort menu.
	m = press(t, m, "d")
	if strings.Contains(m.View(), "MOVE TO TRASH") {
		t.Error("d leaked through the help overlay")
	}
	m = press(t, m, "s")
	if strings.Contains(m.View(), "enter apply") {
		t.Error("s leaked through the help overlay")
	}
	if !strings.Contains(m.View(), "keys") {
		t.Fatal("the help overlay should still be up")
	}

	// esc closes; the grid owns keys again.
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(m.View(), "keys") {
		t.Error("esc should close the help overlay")
	}
	m = press(t, m, "d")
	if !strings.Contains(m.View(), "MOVE TO TRASH") {
		t.Error("the grid should own keys again after help closes")
	}

	// The confirmation closes, then ? reopens the legend.
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = press(t, m, "?")
	if !strings.Contains(m.View(), "keys") {
		t.Error("? should reopen the help overlay")
	}
}
