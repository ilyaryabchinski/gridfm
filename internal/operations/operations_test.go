package operations_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gridfm/internal/operations"
)

// syncMgr drains the event stream until the job with opID finishes and
// returns its result along with everything published before it. Conflict
// questions are answered with skip unless the test intercepts them first.
func syncMgr(t *testing.T, mgr *operations.Manager, opID string) operations.Result {
	t.Helper()

	for {
		ev := <-mgr.Events()

		switch e := ev.(type) {
		case operations.FinishedEvent:
			if e.Result.OpID == opID {
				return e.Result
			}
		case operations.QuestionEvent:
			// Tests answer conflicts inline by default: skip.
			e.AnswerCh <- operations.Answer{Action: operations.ConflictSkip}
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	writeErr := os.WriteFile(path, []byte(content), 0o644)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestCopyFileAndDirectoryRecursively(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	mustWrite(t, filepath.Join(srcDir, "a.txt"), "alpha")
	mkdirErr := os.Mkdir(filepath.Join(srcDir, "sub"), 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(srcDir, "sub", "b.txt"), "beta")

	mgr := operations.NewManager()
	err := mgr.Enqueue(operations.Operation{
		ID:   "copy-1",
		Kind: operations.OpCopy,
		Items: []operations.Item{
			{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")},
			{Source: filepath.Join(srcDir, "sub"), Target: filepath.Join(dstDir, "sub")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result := syncMgr(t, mgr, "copy-1")
	if result.Cancelled || result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("result = %+v, want 2 succeeded", result)
	}
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "alpha" {
		t.Errorf("copied content = %q, want alpha", got)
	}
	if got := mustRead(t, filepath.Join(dstDir, "sub", "b.txt")); got != "beta" {
		t.Errorf("nested content = %q, want beta", got)
	}
}

func TestCopyRejectsDestinationInsideSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	mgr := operations.NewManager()
	err := mgr.Enqueue(operations.Operation{
		ID:   "bad",
		Kind: operations.OpCopy,
		Items: []operations.Item{
			{Source: dir, Target: filepath.Join(dir, "inside")},
		},
	})
	if err == nil {
		t.Fatal("copy into own descendant must be rejected")
	}
}

func TestCopyPreservesModeAndModTime(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "script.sh")
	mustWrite(t, src, "#!/bin/sh\n")
	chmodErr := os.Chmod(src, 0o750)
	if chmodErr != nil {
		t.Fatal(chmodErr)
	}
	wantTime := time.Unix(1700000000, 0)
	err100 := os.Chtimes(src, wantTime, wantTime)
	if err100 != nil {
		t.Fatal(err100)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-mode",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: src, Target: filepath.Join(dstDir, "script.sh")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	syncMgr(t, mgr, "copy-mode")
	dst := filepath.Join(dstDir, "script.sh")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %v, want 750", info.Mode().Perm())
	}
	if !info.ModTime().Equal(wantTime) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), wantTime)
	}
}

func TestCopySymlinkAsSymlink(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	target := filepath.Join(srcDir, "target.txt")
	mustWrite(t, target, "linked")
	link := filepath.Join(srcDir, "link")
	err100 := os.Symlink(target, link)
	if err100 != nil {
		t.Fatal(err100)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-link",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: link, Target: filepath.Join(dstDir, "link")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	syncMgr(t, mgr, "copy-link")
	dst := filepath.Join(dstDir, "link")
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("copied link should stay a symlink, got mode %v", info.Mode())
	}
	gotLink, linkErr := os.Readlink(dst)
	if linkErr != nil || gotLink != target {
		t.Errorf("link target = %q, %v; want %q", gotLink, linkErr, target)
	}
}

func TestConflictSkipKeepsExistingTarget(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "new")
	mustWrite(t, filepath.Join(dstDir, "a.txt"), "old")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-skip",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	// The conflict question is answered with skip by syncMgr.
	result := syncMgr(t, mgr, "copy-skip")
	if result.Skipped != 1 || result.Succeeded != 0 {
		t.Fatalf("result = %+v, want 1 skipped", result)
	}
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "old" {
		t.Errorf("existing target changed: %q", got)
	}
}

func TestConflictReplaceOverwrites(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "new")
	mustWrite(t, filepath.Join(dstDir, "a.txt"), "old")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-replace",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	for {
		ev := <-mgr.Events()
		if q, ok := ev.(operations.QuestionEvent); ok {
			q.AnswerCh <- operations.Answer{Action: operations.ConflictReplace}

			continue
		}
		if fin, ok := ev.(operations.FinishedEvent); ok {
			if fin.Result.Succeeded != 1 {
				t.Fatalf("result = %+v, want 1 succeeded", fin.Result)
			}

			break
		}
	}
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "new" {
		t.Errorf("target content = %q, want new", got)
	}
}

func TestCopyReplaceUnlinksDestinationSymlink(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "new")

	// The destination symlink points at an unrelated file; replacing it
	// must rewrite the link itself, never the file it points at.
	innocent := filepath.Join(dstDir, "innocent.txt")
	mustWrite(t, innocent, "keep me")
	dst := filepath.Join(dstDir, "a.txt")
	symlinkErr := os.Symlink(innocent, dst)
	if symlinkErr != nil {
		t.Fatal(symlinkErr)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-replace-link",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: filepath.Join(srcDir, "a.txt"), Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	for {
		ev := <-mgr.Events()
		if q, ok := ev.(operations.QuestionEvent); ok {
			q.AnswerCh <- operations.Answer{Action: operations.ConflictReplace}

			continue
		}
		if fin, ok := ev.(operations.FinishedEvent); ok {
			if fin.Result.Succeeded != 1 {
				t.Fatalf("result = %+v, want 1 succeeded", fin.Result)
			}

			break
		}
	}

	if got := mustRead(t, innocent); got != "keep me" {
		t.Errorf("symlink target clobbered: %q", got)
	}
	info, statErr := os.Lstat(dst)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("replaced entry should be a regular file, not the old symlink")
	}
	if got := mustRead(t, dst); got != "new" {
		t.Errorf("replacement content = %q, want new", got)
	}
}

func TestCopyOntoHardLinkOfSourceIsRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	mustWrite(t, src, "precious")

	// dst is a hard link to src: copying onto it used to truncate the
	// shared inode before the source was read, emptying both names.
	dst := filepath.Join(dir, "b.txt")
	linkErr := os.Link(src, dst)
	if linkErr != nil {
		t.Fatal(linkErr)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-same",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: src, Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "copy-same")
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want 1 failed", result)
	}
	if got := mustRead(t, src); got != "precious" {
		t.Errorf("source content destroyed: %q", got)
	}
	if got := mustRead(t, dst); got != "precious" {
		t.Errorf("hard link content destroyed: %q", got)
	}
}

func TestConflictRenameWritesUniqueName(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "new")
	mustWrite(t, filepath.Join(dstDir, "a.txt"), "old")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-rename",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	for {
		ev := <-mgr.Events()
		if q, ok := ev.(operations.QuestionEvent); ok {
			q.AnswerCh <- operations.Answer{Action: operations.ConflictRename}

			continue
		}
		if fin, ok := ev.(operations.FinishedEvent); ok && fin.Result.Succeeded == 1 {
			break
		}
	}

	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "old" {
		t.Errorf("existing target changed: %q", got)
	}
	renamed := filepath.Join(dstDir, "a (copy).txt")
	if got := mustRead(t, renamed); got != "new" {
		t.Errorf("renamed copy = %q at %q, want new", got, renamed)
	}
}

func TestApplyAllAnswersLaterConflicts(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		mustWrite(t, filepath.Join(srcDir, name), "new")
		mustWrite(t, filepath.Join(dstDir, name), "old")
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:   "copy-all",
		Kind: operations.OpCopy,
		Items: []operations.Item{
			{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")},
			{Source: filepath.Join(srcDir, "b.txt"), Target: filepath.Join(dstDir, "b.txt")},
		},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	questions := 0
	for {
		ev := <-mgr.Events()
		if q, ok := ev.(operations.QuestionEvent); ok {
			questions++
			q.AnswerCh <- operations.Answer{Action: operations.ConflictReplace, ApplyAll: true}

			continue
		}
		if fin, ok := ev.(operations.FinishedEvent); ok && fin.Result.Succeeded == 2 {
			break
		}
	}
	if questions != 1 {
		t.Errorf("apply-to-all should answer later conflicts silently, got %d questions", questions)
	}
}

func TestCancelMarksResultAndStopsWork(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	for i := range 5 {
		mustWrite(t, filepath.Join(srcDir, string(rune('a'+i))+".txt"), "x")
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:   testCancelOpID,
		Kind: operations.OpCopy,
		Items: []operations.Item{
			{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")},
			{Source: filepath.Join(srcDir, "b.txt"), Target: filepath.Join(dstDir, "b.txt")},
			{Source: filepath.Join(srcDir, "c.txt"), Target: filepath.Join(dstDir, "c.txt")},
			{Source: filepath.Join(srcDir, "d.txt"), Target: filepath.Join(dstDir, "d.txt")},
			{Source: filepath.Join(srcDir, "e.txt"), Target: filepath.Join(dstDir, "e.txt")},
		},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := runCancelAfterFirstProgress(t, mgr)
	if !result.Cancelled {
		t.Fatalf("result should be cancelled: %+v", result)
	}
	if got := result.Succeeded + result.Skipped + result.Failed + result.NotRun; got != 5 {
		t.Fatalf("accounting does not add up (%d): %+v", got, result)
	}
	if result.Succeeded < 1 {
		t.Fatalf("the first item should have completed: %+v", result)
	}
	for _, failure := range result.Failures {
		if failure.Path == "" {
			t.Errorf("every failure should name its item: %+v", result.Failures)
		}
	}
}

// testCancelOpID is the job identifier used by runCancelAfterFirstProgress.
const testCancelOpID = "copy-cancel"

// runCancelAfterFirstProgress cancels the job once its first item is
// reported and drains events until the finished result arrives. Exactly
// where the cancel lands beyond that point is timing-dependent.
func runCancelAfterFirstProgress(t *testing.T, mgr *operations.Manager) operations.Result {
	t.Helper()

	cancelled := false

	for {
		ev := <-mgr.Events()

		if _, ok := ev.(operations.ProgressEvent); ok && !cancelled {
			cancelled = true
			mgr.CancelActive()

			continue
		}

		if fin, ok := ev.(operations.FinishedEvent); ok && fin.Result.OpID == testCancelOpID {
			return fin.Result
		}
	}
}

func TestFailedItemDoesNotLoseSource(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	// The source of the failed item must survive any failure mode.
	src := filepath.Join(srcDir, "precious.txt")
	mustWrite(t, src, "keep me")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:   "move-fail",
		Kind: operations.OpMove,
		Items: []operations.Item{
			// A move whose source vanishes mid-flight cannot be arranged
			// portably; instead fail the target side: dst is read-only dir.
			{Source: src, Target: filepath.Join(dstDir, "missing", "deep", "precious.txt")},
		},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "move-fail")
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want 1 failed", result)
	}
	if got := mustRead(t, src); got != "keep me" {
		t.Errorf("source lost on failure: %q", got)
	}
}

func TestMoveSameFilesystemUsesRename(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "a.txt"), "data")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "move",
		Kind:  operations.OpMove,
		Items: []operations.Item{{Source: filepath.Join(srcDir, "a.txt"), Target: filepath.Join(dstDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "move")
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	_, statErr := os.Stat(filepath.Join(srcDir, "a.txt"))
	if !os.IsNotExist(statErr) {
		t.Error("source should be gone after a move")
	}
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "data" {
		t.Errorf("moved content = %q", got)
	}
}

//nolint:paralleltest // swaps the package-level rename seam
func TestMoveCrossFilesystemCopiesThenRemoves(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "a.txt")
	mustWrite(t, src, "cross")

	// Force the cross-filesystem path: rename fails with EXDEV, so the
	// move must copy first and remove the source only after the copy
	// fully succeeds.
	restore := operations.SwapRenameWithEXDEVForTest()
	defer restore()

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "move-cross",
		Kind:  operations.OpMove,
		Items: []operations.Item{{Source: src, Target: filepath.Join(dstDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "move-cross")
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	if got := mustRead(t, filepath.Join(dstDir, "a.txt")); got != "cross" {
		t.Errorf("copied content = %q", got)
	}
	_, srcStat := os.Stat(src)
	if !os.IsNotExist(srcStat) {
		t.Error("source should be removed only after a successful copy")
	}
}

//nolint:paralleltest // swaps the package-level rename seam
func TestMoveCrossFilesystemKeepsSourceWhenCopyFails(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "a.txt")
	mustWrite(t, src, "precious")

	// EXDEV on rename, then a failing copy: the source must survive.
	restore := operations.SwapRenameWithEXDEVForTest()
	defer restore()
	failDir := filepath.Join(dstDir, "sub")
	err100 := os.Mkdir(failDir, 0o000)
	if err100 != nil {
		t.Fatal(err100)
	}
	t.Cleanup(func() {
		_ = os.Chmod(failDir, 0o755)
	})

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "move-cross-fail",
		Kind:  operations.OpMove,
		Items: []operations.Item{{Source: src, Target: filepath.Join(failDir, "a.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "move-cross-fail")
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want 1 failed", result)
	}
	if got := mustRead(t, src); got != "precious" {
		t.Errorf("source lost on failed cross-device move: %q", got)
	}
}

func TestCreateFileAndDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "create",
		Kind:  operations.OpCreateFile,
		Items: []operations.Item{{Target: filepath.Join(dir, "new.txt")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}
	syncMgr(t, mgr, "create")
	if got := mustRead(t, filepath.Join(dir, "new.txt")); got != "" {
		t.Errorf("created file should be empty, got %q", got)
	}

	mgr2 := operations.NewManager()
	mkdirEnqueueErr := mgr2.Enqueue(operations.Operation{
		ID:    "mkdir",
		Kind:  operations.OpCreateDir,
		Items: []operations.Item{{Target: filepath.Join(dir, "sub")}},
	})
	if mkdirEnqueueErr != nil {
		t.Fatal(mkdirEnqueueErr)
	}
	syncMgr(t, mgr2, "mkdir")
	info, err := os.Stat(filepath.Join(dir, "sub"))
	if err != nil || !info.IsDir() {
		t.Errorf("created dir missing: %v", err)
	}
}
