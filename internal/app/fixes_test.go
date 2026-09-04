package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/operations"
)

// TestCrossDirectoryDeleteRequiresTypedYes pins the safety net for
// cross-directory selections: a directory chosen in one location stays a
// directory after navigation, so deleting it alone must still demand the
// typed confirmation instead of a single "y".
func TestCrossDirectoryDeleteRequiresTypedYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	kid := filepath.Join(root, "kid")
	other := filepath.Join(root, "other")
	for _, dir := range []string{kid, other} {
		if mkdirErr := os.Mkdir(dir, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "kid", Path: kid, IsDir: true},
		{Name: "other", Path: other, IsDir: true},
	}, nil)

	// Select the focused directory, then navigate away from it.
	m = press(t, m, " ")
	m = pressBackspace(t, m)
	parent := filepath.Dir(root)
	m = loaded(t, m, 2, parent, entriesAt(parent, 3), nil)

	m = press(t, m, "D")
	if !strings.Contains(m.View(), "DELETE PERMANENTLY") {
		t.Fatal("D should open the delete confirmation")
	}
	if !strings.Contains(m.View(), "type yes") {
		t.Error("a cross-directory directory delete must require typing yes")
	}
}

// TestCtrlDInRenameKeepsRenameOperation pins that the create-kind switch
// never hijacks the rename overlay.
func TestCtrlDInRenameKeepsRenameOperation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "before.txt")
	mustWrite(t, src, "x")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{{Name: "before.txt", Path: src}}, nil)

	m = press(t, m, "R")
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})

	if view := m.View(); strings.Contains(view, "NEW FILE") {
		t.Error("ctrl+d must not turn the rename overlay into file creation")
	}

	// The operation the overlay submits is still a rename.
	m = press(t, m, "2")
	_ = press(t, m, "enter")
	if strings.Contains(m.View(), "create file started") {
		t.Error("a create started instead of a rename")
	}
	waitFor(t, func() bool {
		m.DrainEvents()
		_, err := os.Stat(filepath.Join(root, "before.txt2"))

		return err == nil
	})
}

// TestResultsOverlayBlocksGridKeysAndEscCloses pins the blocking overlay
// contract: while results are shown, mutation shortcuts stay inert, and
// esc closes the overlay as the footer promises.
func TestResultsOverlayBlocksGridKeysAndEscCloses(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-1", Result: operations.Result{
			OpID: "op-1", Kind: operations.OpCopy, Failed: 1,
			Failures: []operations.ItemError{{Path: "/d/x", Err: errTestLoad}},
		}},
	})
	if !strings.Contains(m.View(), "permission denied") {
		t.Fatal("the results overlay should render")
	}

	// A mutation shortcut must not reach the grid behind the overlay.
	m = press(t, m, "D")
	if strings.Contains(m.View(), "DELETE PERMANENTLY") {
		t.Error("D leaked through the blocking results overlay")
	}

	// esc closes the overlay, as the footer hints.
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(m.View(), "permission denied") {
		t.Error("esc should close the results overlay")
	}
	// ...and the grid reacts to keys again.
	m = press(t, m, "D")
	if !strings.Contains(m.View(), "DELETE PERMANENTLY") {
		t.Error("the grid should own keys again after the overlay closes")
	}
}

// TestApplyAllResetsBetweenJobs pins that an apply-to-all decision cannot
// outlive the job it was made in.
func TestApplyAllResetsBetweenJobs(t *testing.T) {
	t.Parallel()

	first := make(chan operations.Answer, 1)
	second := make(chan operations.Answer, 1)
	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.QuestionEvent{Target: "/d/a", AnswerCh: first},
	})
	m = press(t, m, "a") // apply-all on
	m = press(t, m, "s") // skip with apply-all
	select {
	case answer := <-first:
		if !answer.ApplyAll {
			t.Fatal("the first answer should carry apply-all")
		}
	default:
		t.Fatal("no first answer delivered")
	}

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-1", Result: operations.Result{
			OpID: "op-1", Kind: operations.OpCopy, Succeeded: 1,
		}},
	})
	m.DrainEvents()

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.QuestionEvent{Target: "/d/b", AnswerCh: second},
	})
	if strings.Contains(m.View(), "apply to all: ON") {
		t.Error("apply-all must reset when the job finishes")
	}
	m = press(t, m, "s")
	select {
	case answer := <-second:
		if answer.ApplyAll {
			t.Error("the next job's answer must not inherit apply-all")
		}
	default:
		t.Fatal("no second answer delivered")
	}
}

// TestResultNoteSurvivesRefresh pins that a finished job's summary stays
// visible across the refresh its completion triggers.
func TestResultNoteSurvivesRefresh(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-1", Result: operations.Result{
			OpID: "op-1", Kind: operations.OpCopy, Succeeded: 2,
		}},
	})
	if view := m.View(); !strings.Contains(view, "copy: 2 oks") {
		t.Fatalf("status should summarize the finished job, got %q", view)
	}

	// The refresh started by the finish event resolves; the summary must
	// not be wiped by it.
	m = loaded(t, m, 2, "/d", entriesAt("/d", 2), nil)
	if view := m.View(); !strings.Contains(view, "copy: 2 oks") {
		t.Errorf("the refresh wiped the result summary: %q", view)
	}
}

// TestSelectAnchorFollowsEntryAfterSort pins identity-anchored ranges: a
// select-mode anchor survives a reordering of the visible entries.
func TestSelectAnchorFollowsEntryAfterSort(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 10), nil)

	m = press(t, m, "v") // anchor and select entry-00

	// Reverse the ordering through the sort menu; focus follows entry-00
	// by identity to the end of the grid.
	m = press(t, m, "s")
	m = press(t, m, "o") // name descending
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	m = press(t, m, "h") // extend the range left of the anchored entry

	// The range is the anchored entry-00 plus the entry to its left: two.
	// A stale index anchor would have swept nine entries instead.
	if got := m.SelectedCount(); got != 2 {
		t.Fatalf("identity-anchored range = %d selected, want 2", got)
	}
}

// TestBusyCtrlCRequiresQuitConfirmation pins quit parity: ctrl+c with an
// operation in flight opens the confirmation instead of quitting outright.
func TestBusyCtrlCRequiresQuitConfirmation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, entriesAt(root, 2), nil)

	// Enqueue a job without draining events: the worker announces the item
	// and then blocks on the unbuffered event stream, so the job is
	// reliably in flight.
	if err := m.EnqueueOperation(operations.Operation{
		ID:   "busy-op",
		Kind: operations.OpCreateFile,
		Items: []operations.Item{
			{Target: filepath.Join(root, "pending.txt")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if !m.Busy() {
		t.Fatal("the enqueued job should be in flight")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd != nil {
		t.Fatal("ctrl+c during a running operation must not quit outright")
	}
	if view := updated.View(); !strings.Contains(view, "QUIT?") {
		t.Error("ctrl+c during a running operation should ask for confirmation")
	}

	// Confirming proceeds with the quit.
	_, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Error("confirming the quit dialog should produce a quit command")
	}
}

// TestIdleCtrlCQuitsWithoutConfirmation pins the idle path: with nothing
// running, ctrl+c quits immediately.
func TestIdleCtrlCQuitsWithoutConfirmation(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("idle ctrl+c should quit immediately")
	}
}

// TestShelfShowsByteProgress pins the shelf rendering for a copy with byte
// counts: the totals render in human units next to the item counters.
func TestShelfShowsByteProgress(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.ProgressEvent{
			ID: "op-1", Kind: operations.OpCopy, Done: 0, Total: 1,
			Target: "/d/big.bin", ItemBytes: 1_600_000, ItemBytesTotal: 4_000_000,
		},
	})

	view := m.View()
	for _, want := range []string{"0/1", "1.5 MB", "3.8 MB", "c cancels"} {
		if !strings.Contains(view, want) {
			t.Errorf("shelf should show %q, got %q", want, view)
		}
	}

	// An item without measurable size renders no byte segment.
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.ProgressEvent{ID: "op-1", Kind: operations.OpDelete, Done: 0, Total: 1, Target: "/d/x"},
	})
	if view := m.View(); strings.Contains(view, "MB") {
		t.Errorf("delete progress should not show byte counts: %q", view)
	}
}

// TestLoadErrorNoteClearsOnSuccessfulRetry pins that a load error note is
// cleared exactly when a retry succeeds, not on every unrelated load.
func TestLoadErrorNoteClearsOnSuccessfulRetry(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", nil, errTestLoad)
	if view := m.View(); !strings.Contains(view, "error: permission denied") {
		t.Fatalf("the load error should be surfaced, got %q", view)
	}

	// Retry via a real navigation, which bumps the request identity.
	m = pressBackspace(t, m)
	m = loaded(t, m, 2, "/", entriesAt("/", 2), nil)
	if view := m.View(); strings.Contains(view, "error: permission denied") {
		t.Error("a successful retry should clear the load error note")
	}
}

// TestFilterClampsScrollToResults drives a scrolled viewport into a filter
// that keeps one entry: the grid must not render an empty page.
func TestFilterClampsScrollToResults(t *testing.T) {
	t.Parallel()

	const count = 40 // several pages at 5 columns
	base := "/d"
	entries := make([]browser.Entry, count)
	for i := range entries {
		name := fmt.Sprintf("entry-%03d.txt", i)
		entries[i] = browser.Entry{Name: name, Path: filepath.Join(base, name)}
	}

	m := gridOnly(t, resize(t, app.New(base, app.Options{}), 80, 24))
	m = loaded(t, m, 1, base, entries, nil)

	// Page down to the last row, then open the filter and match one entry
	// far above the viewport.
	for range 5 {
		m = feed(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	m = press(t, m, "/")
	m = press(t, m, "0") // entry-000 is the only match... entry-030 also has 0
	m = press(t, m, "0")
	m = press(t, m, "0")
	if m.Filter() != "000" {
		t.Fatalf("filter = %q, want 000", m.Filter())
	}

	view := m.View()
	if strings.Contains(view, "entry-0") && !strings.Contains(view, "entry-000") {
		t.Error("the viewport shows other entries than the single match")
	}
	if !strings.Contains(view, "entry-000") {
		t.Error("the only matching entry must be visible after filtering")
	}
}
