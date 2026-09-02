package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/operations"
)

// mustWrite writes a file, failing the test on error.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	writeErr := os.WriteFile(path, []byte(content), 0o644)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition not met within the deadline")
}

func TestSpaceTogglesSelection(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "a.txt", Path: "/d/a.txt"},
		{Name: "b.txt", Path: "/d/b.txt"},
	}, nil)

	m = press(t, m, " ")
	if m.SelectedCount() != 1 {
		t.Fatalf("selected = %d, want 1", m.SelectedCount())
	}
	m = press(t, m, " ")
	if m.SelectedCount() != 0 {
		t.Fatalf("selected after untoggle = %d, want 0", m.SelectedCount())
	}
}

func TestSelectModeExtendsRange(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 9), nil)

	m = press(t, m, "v") // enter select mode at entry-00
	m = press(t, m, "j") // down one row of 5 columns: covers entries 0-5

	if m.Mode() != app.ModeSelect {
		t.Fatal("v should enter select mode")
	}
	if m.SelectedCount() != 6 {
		t.Fatalf("range selection = %d, want 6 (two rows of 5, 9 entries)", m.SelectedCount())
	}

	m = press(t, m, "v") // exit, selection kept
	if m.Mode() != app.ModeBrowse {
		t.Error("second v should return to browse mode")
	}
	if m.SelectedCount() != 6 {
		t.Errorf("selection should persist after leaving select mode, got %d", m.SelectedCount())
	}

	// esc clears transient state first, then the selection.
	m = press(t, m, "esc")
	m = press(t, m, "esc")
	if m.SelectedCount() != 0 {
		t.Errorf("selection should clear after two esc presses, got %d", m.SelectedCount())
	}
}

func TestSelectAllVisible(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	}, nil)

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA}); cmd == nil && m.SelectedCount() != 2 {
		t.Fatal("ctrl+a should select every visible entry")
	}
	if m.SelectedCount() != 2 {
		t.Fatalf("selected = %d, want 2", m.SelectedCount())
	}
}

func TestStageAndPasteCopyAcrossDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	mkdirErr := os.Mkdir(srcDir, 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mkdirErr = os.Mkdir(dstDir, 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "content")

	m := gridOnly(t, resize(t, app.New(srcDir, app.Options{}), 80, 24))
	m = loaded(t, m, 1, srcDir, []browser.Entry{
		{Name: "a.txt", Path: filepath.Join(srcDir, "a.txt")},
	}, nil)

	m = press(t, m, "y") // stage copy
	if m.ClipboardKind() != app.ClipboardCopy || len(m.ClipboardPaths()) != 1 {
		t.Fatal("y should stage the focused entry for copy")
	}

	// Navigate to the parent and into dst: dirs sort first, so dst takes
	// focus on arrival.
	m = pressBackspace(t, m)
	rootEntries, err := browser.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	m = loaded(t, m, 2, root, rootEntries, nil)
	m = press(t, m, "enter") // enter the focused dir (dst)
	m = loaded(t, m, 3, dstDir, nil, nil)
	waitFor(t, func() bool { return m.Path() == dstDir && !m.IsLoading() })

	m = press(t, m, "p") // paste
	waitFor(t, func() bool {
		m.DrainEvents()
		_, statErr := os.Stat(filepath.Join(dstDir, "a.txt"))

		return statErr == nil
	})
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "content" {
		t.Errorf("copied content = %q", got)
	}
	// Copies stay staged for repeated pastes.
	if m.ClipboardKind() != app.ClipboardCopy {
		t.Error("copy clipboard should persist after pasting")
	}
}

func TestStageAndPasteMoveClearsClipboard(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	subMkdirErr := os.Mkdir(sub, 0o755)
	if subMkdirErr != nil {
		t.Fatal(subMkdirErr)
	}
	src := filepath.Join(root, "gone.txt")
	mustWrite(t, src, "moving")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	rootEntries, err := browser.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	m = loaded(t, m, 1, root, rootEntries, nil)

	// Dirs sort first: focus starts on sub; step right to the file.
	m = press(t, m, "l")
	m = press(t, m, "x") // stage move
	if m.ClipboardKind() != app.ClipboardMove {
		t.Fatal("x should stage the focused entry for move")
	}
	if len(m.ClipboardPaths()) != 1 || m.ClipboardPaths()[0] != src {
		t.Fatalf("staged = %v, want %q", m.ClipboardPaths(), src)
	}

	m = press(t, m, "h") // back to sub
	m = press(t, m, "enter")
	m = loaded(t, m, 2, sub, nil, nil)
	waitFor(t, func() bool { return m.Path() == sub && !m.IsLoading() })

	m = press(t, m, "p") // paste into sub

	waitFor(t, func() bool {
		_, err := os.Stat(src)

		return os.IsNotExist(err)
	})
	waitFor(t, func() bool {
		_, err := os.Stat(filepath.Join(sub, "gone.txt"))

		return err == nil
	})
	if m.ClipboardKind() != app.ClipboardNone {
		t.Error("move clipboard should clear after pasting")
	}
}

func TestCreateFileAndDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, nil, nil)

	// Create a file.
	m = press(t, m, "n")
	m = press(t, m, "n")
	m = press(t, m, "e")
	m = press(t, m, "w")
	_ = press(t, m, "enter")
	waitFor(t, func() bool {
		m.DrainEvents()
		_, err := os.Stat(filepath.Join(root, "new"))

		return err == nil
	})

	// Switch to directory mode with ctrl+d.
	m = press(t, m, "n")
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	m = press(t, m, "s")
	m = press(t, m, "u")
	m = press(t, m, "b")
	_ = press(t, m, "enter")
	waitFor(t, func() bool {
		m.DrainEvents()
		info, err := os.Stat(filepath.Join(root, "sub"))

		return err == nil && info.IsDir()
	})

	// Invalid names never reach the filesystem.
	m = press(t, m, "n")
	m = press(t, m, "b")
	m = press(t, m, "a")
	m = feed(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = press(t, m, "d")
	_ = press(t, m, "enter")
	time.Sleep(50 * time.Millisecond)
	_, badStat := os.Stat(filepath.Join(root, "bad"))
	if badStat == nil {
		t.Error("a name with separators must be rejected")
	}
}

func TestRenameFlow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	old := filepath.Join(root, "before.txt")
	mustWrite(t, old, "x")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{{Name: "before.txt", Path: old}}, nil)

	m = press(t, m, "R") // prefilled with before.txt
	if !strings.Contains(m.View(), "RENAME") {
		t.Fatal("R should open the rename overlay")
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU}) // clear the prefilled name
	for _, r := range "after.txt" {
		m = press(t, m, string(r))
	}
	_ = press(t, m, "enter")

	waitFor(t, func() bool {
		m.DrainEvents()
		_, err := os.Stat(filepath.Join(root, "after.txt"))

		return err == nil
	})
	_, statErr := os.Stat(old)
	if !os.IsNotExist(statErr) {
		t.Error("the old name should be gone after rename")
	}
}

func TestRenameUnchangedIsSilentNoOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "same.txt")
	mustWrite(t, src, "x")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{{Name: "same.txt", Path: src}}, nil)

	m = press(t, m, "R")
	m = press(t, m, "enter") // unchanged prefilled name

	if strings.Contains(m.View(), "rename started") {
		t.Error("an unchanged rename should not start an operation")
	}
	_, err100 := os.Stat(src)
	if err100 != nil {
		t.Errorf("source should be untouched: %v", err100)
	}
}

func TestTrashRequiresConfirmationAndRuns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	victim := filepath.Join(root, "gone.txt")
	mustWrite(t, victim, "x")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{{Name: "gone.txt", Path: victim}}, nil)

	m = press(t, m, "d")
	if !strings.Contains(m.View(), "MOVE TO TRASH") {
		t.Fatal("d should open the trash confirmation")
	}

	_ = press(t, m, "y")
	waitFor(t, func() bool {
		_, err := os.Stat(victim)

		return os.IsNotExist(err)
	})
}

func TestDeleteMultipleRequiresTypedYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	first := filepath.Join(root, "one.txt")
	second := filepath.Join(root, "two.txt")
	mustWrite(t, first, "1")
	mustWrite(t, second, "2")

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "one.txt", Path: first},
		{Name: "two.txt", Path: second},
	}, nil)

	// Select both, then request delete.
	m = press(t, m, " ")
	m = press(t, m, "l")
	m = press(t, m, " ")
	m = press(t, m, "D")
	if !strings.Contains(m.View(), "DELETE PERMANENTLY") {
		t.Fatal("D should open the delete confirmation")
	}
	if !strings.Contains(m.View(), "type yes") {
		t.Fatal("multi-item delete should require typing yes")
	}

	// A bare y is treated as part of the typed answer, not a confirmation.
	_ = press(t, m, "y")
	_, firstStat := os.Stat(first)
	if firstStat != nil {
		t.Fatal("delete ran without the typed confirmation")
	}

	// Clear the stray y, type the word, and confirm.
	m = press(t, m, "backspace")
	for _, r := range "yes" {
		m = press(t, m, string(r))
	}
	_ = press(t, m, "enter")
	waitFor(t, func() bool {
		m.DrainEvents()
		_, err1 := os.Stat(first)
		_, err2 := os.Stat(second)

		return os.IsNotExist(err1) && os.IsNotExist(err2)
	})
}

func TestConflictQuestionOverlayAnswers(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 1), nil)

	// A synthetic conflict question flows through the normal path.
	answerCh := make(chan operations.Answer, 1)
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.QuestionEvent{Target: "/d/entry-00.txt", AnswerCh: answerCh},
	})

	if !strings.Contains(m.View(), "TARGET EXISTS") {
		t.Fatal("the conflict overlay should render")
	}

	m = press(t, m, "r") // replace
	select {
	case answer := <-answerCh:
		if answer.Action != operations.ConflictReplace {
			t.Errorf("answer action = %v, want replace", answer.Action)
		}
		if answer.ApplyAll {
			t.Error("apply-all should default to off")
		}
	case <-time.After(time.Second):
		t.Fatal("no answer delivered")
	}

	if strings.Contains(m.View(), "TARGET EXISTS") {
		t.Error("answering should close the overlay")
	}
}

func TestConflictApplyAllToggles(t *testing.T) {
	t.Parallel()

	answerCh := make(chan operations.Answer, 1)
	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = feed(t, m, app.OperationEventMsg{
		Event: operations.QuestionEvent{Target: "/d/x", AnswerCh: answerCh},
	})

	m = press(t, m, "a") // apply-all on
	if !strings.Contains(m.View(), "a apply to all: ON") {
		t.Fatal("the overlay should reflect apply-all before answering")
	}
	_ = press(t, m, "s") // skip

	select {
	case answer := <-answerCh:
		if !answer.ApplyAll || answer.Action != operations.ConflictSkip {
			t.Fatalf("answer = %+v, want skip with apply-all", answer)
		}
	case <-time.After(time.Second):
		t.Fatal("no answer delivered")
	}
}

func TestOperationFinishedUpdatesResultsAndNote(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	m = feed(t, m, app.OperationEventMsg{
		Event: operations.FinishedEvent{ID: "op-1", Result: operations.Result{
			OpID: "op-1", Kind: operations.OpCopy,
			Succeeded: 2, Skipped: 1, Failed: 1, Cancelled: false,
			Failures: []operations.ItemError{{Path: "/d/x", Err: errTestLoad}},
		}},
	})

	if view := m.View(); !strings.Contains(view, "copy: 2 oks, 1 skipped, 1 failed") {
		t.Errorf("status should summarize the result, got %q", view)
	}
	if !strings.Contains(m.View(), "permission denied") {
		t.Error("failures should be visible in the results overlay")
	}

	// e toggles the results overlay closed and open.
	m = press(t, m, "e")
	if strings.Contains(m.View(), "permission denied") {
		t.Error("e should close the results overlay")
	}
}

func TestQuitWhenIdleNeedsNoConfirmation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))

	// Idle quit needs no confirmation.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Fatal("idle q should produce a quit command")
	}
}

// mustRead reads a file, failing the test on error.
func mustRead(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
