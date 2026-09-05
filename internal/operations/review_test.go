package operations_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"gridfm/internal/operations"
)

// syncMergeSkipRest drains events until opID finishes: the first conflict
// (the merging directory itself) is answered with replace, every later one
// (the conflicting child) with skip.
func syncMergeSkipRest(t *testing.T, mgr *operations.Manager, opID string) operations.Result {
	t.Helper()

	first := true
	for {
		ev := <-mgr.Events()
		switch e := ev.(type) {
		case operations.QuestionEvent:
			if first {
				first = false
				e.AnswerCh <- operations.Answer{Action: operations.ConflictReplace}

				continue
			}
			e.AnswerCh <- operations.Answer{Action: operations.ConflictSkip}
		case operations.FinishedEvent:
			if e.Result.OpID == opID {
				return e.Result
			}
		}
	}
}

// awaitIdle waits for the worker's deferred in-flight decrement.
func awaitIdle(t *testing.T, mgr *operations.Manager) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for mgr.Busy() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

// TestCopyRejectsSpecialFiles pins that special entries are rejected
// before any open: copying a named pipe with no writer used to block
// forever in os.Open, unreachable by cancellation, stalling the serial
// queue behind it.
func TestCopyRejectsSpecialFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	fifo := filepath.Join(srcDir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-fifo",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: fifo, Target: filepath.Join(dstDir, "pipe")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "copy-fifo")
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want 1 failed", result)
	}
	if _, err := os.Lstat(filepath.Join(dstDir, "pipe")); !os.IsNotExist(err) {
		t.Errorf("no destination should exist for a rejected special file: %v", err)
	}
}

// TestSkipInsideMergeCopiesLaterSiblings pins that skipping one conflicting
// child does not strand its siblings: a conflicting "a" is skipped while a
// later non-conflicting "z" is still copied.
func TestSkipInsideMergeCopiesLaterSiblings(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "dir")
	if mkdirErr := os.Mkdir(src, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(src, "a"), "new a")
	mustWrite(t, filepath.Join(src, "z"), "new z")

	dst := filepath.Join(dstDir, "dir")
	if mkdirErr := os.Mkdir(dst, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(dst, "a"), "old a")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "merge-skip",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: src, Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMergeSkipRest(t, mgr, "merge-skip")
	if result.Skipped != 1 {
		t.Fatalf("result = %+v, want 1 skipped", result)
	}
	awaitIdle(t, mgr)
	if got := mustRead(t, filepath.Join(dst, "a")); got != "old a" {
		t.Errorf("conflicting child = %q, want untouched old a", got)
	}
	if got := mustRead(t, filepath.Join(dst, "z")); got != "new z" {
		t.Errorf("later sibling lost after a skip: %q", got)
	}
}

// TestCrossDeviceMoveKeepsSourceWhenChildSkipped pins the safety rule the
// skip-continuation depends on: a partial copy (a skipped child) must never
// trigger source removal.
func TestCrossDeviceMoveKeepsSourceWhenChildSkipped(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "dir")
	if mkdirErr := os.Mkdir(src, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(src, "a"), "new a")
	mustWrite(t, filepath.Join(src, "z"), "new z")

	dst := filepath.Join(dstDir, "dir")
	if mkdirErr := os.Mkdir(dst, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(dst, "a"), "old a")

	restore := operations.SwapRenameWithEXDEVForTest()
	defer restore()

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "move-partial",
		Kind:  operations.OpMove,
		Items: []operations.Item{{Source: src, Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMergeSkipRest(t, mgr, "move-partial")
	if result.Skipped != 1 || result.Succeeded != 0 {
		t.Fatalf("result = %+v, want the partial move skipped", result)
	}
	awaitIdle(t, mgr)
	if _, err := os.Stat(filepath.Join(src, "z")); err != nil {
		t.Errorf("partially copied source must survive: %v", err)
	}
	if mgr.Busy() {
		t.Error("the manager should be idle")
	}
}
