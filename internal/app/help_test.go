package app_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/operations"
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

// TestHelpClosesForOperationEvents pins the one-blocking-overlay rule for
// message-driven overlays: an arriving conflict question or finished job
// must close the legend, not stack beside it.
func TestHelpClosesForOperationEvents(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	// Help open when a conflict question arrives: the question wins.
	m = press(t, m, "?")
	answerCh := make(chan operations.Answer, 1)
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.QuestionEvent{Target: "/d/x", AnswerCh: answerCh},
	})
	if strings.Contains(m.View(), "keys") {
		t.Error("the help overlay stayed open beside the conflict question")
	}
	if !strings.Contains(m.View(), "TARGET EXISTS") {
		t.Error("the conflict overlay should render")
	}

	// Help open when a job finishes with failures: results win, and after
	// they close, help must not resurface.
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-1", Result: operations.Result{
			OpID: "op-1", Kind: operations.OpCopy, Failed: 1,
			Failures: []operations.ItemError{{Path: "/d/x", Err: errTestLoad}},
		}},
	})
	m.DrainEvents()
	m = press(t, m, "?") // open help again
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-2", Result: operations.Result{
			OpID: "op-2", Kind: operations.OpCopy, Failed: 1,
			Failures: []operations.ItemError{{Path: "/d/y", Err: errTestLoad}},
		}},
	})
	if strings.Contains(m.View(), "keys") {
		t.Error("the help overlay stayed open beside the results overlay")
	}
	m = press(t, m, "e") // close the results
	if strings.Contains(m.View(), "keys") {
		t.Error("the stale help overlay resurfaced after the results closed")
	}
}
