package operations_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/operations"
)

// TestValidateRejectsSymlinkAliasIntoSource covers the alias bypass: the
// destination is spelled outside the source tree but a symlinked ancestor
// resolves into it, which used to recurse the copy into itself.
func TestValidateRejectsSymlinkAliasIntoSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "src")
	mkdirErr := os.Mkdir(src, 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	alias := filepath.Join(root, "alias")
	if linkErr := os.Symlink(src, alias); linkErr != nil {
		t.Fatal(linkErr)
	}

	mgr := operations.NewManager()
	err := mgr.Enqueue(operations.Operation{
		ID:   "copy-alias",
		Kind: operations.OpCopy,
		Items: []operations.Item{
			{Source: src, Target: filepath.Join(alias, "inside")},
		},
	})
	if !errors.Is(err, operations.ErrDestinationInsideSource) {
		t.Errorf("symlinked alias into source must be rejected, got %v", err)
	}

	// The source must be untouched: no partial self-copy may have started.
	entries, readErr := os.ReadDir(src)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Errorf("rejected copy must not touch the source, found %v", entries)
	}
}

// TestReplaceFailureKeepsExistingDestination pins the ordering contract: an
// unreadable source must fail the item without deleting the entry the user
// agreed to replace.
func TestReplaceFailureKeepsExistingDestination(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions, so the unreadable source cannot be simulated")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	unreadable := filepath.Join(srcDir, "locked.txt")
	mustWrite(t, unreadable, "later")
	if chmodErr := os.Chmod(unreadable, 0o000); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadable, 0o644)
	})

	dst := filepath.Join(dstDir, "a.txt")
	mustWrite(t, dst, "precious old")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-replace-fail",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: unreadable, Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	questions := 0
	for {
		ev := <-mgr.Events()
		if q, ok := ev.(operations.QuestionEvent); ok {
			questions++
			q.AnswerCh <- operations.Answer{Action: operations.ConflictReplace}

			continue
		}
		if fin, ok := ev.(operations.FinishedEvent); ok {
			if fin.Result.Failed != 1 {
				t.Fatalf("result = %+v, want 1 failed", fin.Result)
			}

			break
		}
	}
	if questions != 1 {
		t.Fatalf("questions = %d, want 1", questions)
	}
	if got := mustRead(t, dst); got != "precious old" {
		t.Errorf("failed replace destroyed the destination, content = %q", got)
	}
}

// TestTrashFailureKeepsSourceAndWritesNoRecord pins the metadata ordering:
// the .trashinfo is written before the move, and a metadata failure must
// leave the source exactly where it was.
func TestTrashFailureKeepsSourceAndWritesNoRecord(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))

	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions, so the unwritable info dir cannot be simulated")
	}

	trash := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash")
	infoDir := filepath.Join(trash, "info")
	filesDir := filepath.Join(trash, "files")
	for _, dir := range []string{infoDir, filesDir} {
		if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
	}
	// A read-only info directory makes the exclusive record creation fail
	// after the trash layout exists.
	if chmodErr := os.Chmod(infoDir, 0o500); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	t.Cleanup(func() {
		_ = os.Chmod(infoDir, 0o700)
	})

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "kept.txt")
	mustWrite(t, src, "still here")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "trash-fail",
		Kind:  operations.OpTrash,
		Items: []operations.Item{{Source: src}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "trash-fail")
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want 1 failed", result)
	}
	if got := mustRead(t, src); got != "still here" {
		t.Errorf("failed trash lost the source, content = %q", got)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("source must survive a failed trash: %v", statErr)
	}
	records, readErr := os.ReadDir(infoDir)
	if readErr != nil || len(records) != 0 {
		t.Errorf("failed trash must not leave a record: %v, %v", records, readErr)
	}
}

// TestTrashDoesNotOverwriteExistingRecord pins the exclusive record
// creation: a pre-existing .trashinfo with a reserved name must survive.
func TestTrashDoesNotOverwriteExistingRecord(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))

	trash := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash")
	filesDir := filepath.Join(trash, "files")
	infoDir := filepath.Join(trash, "info")
	for _, dir := range []string{infoDir, filesDir} {
		if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
	}
	// Reserve the first name with a decoy entry, as another trasher would.
	occupied := filepath.Join(filesDir, "doomed.txt")
	mustWrite(t, occupied, "not mine")
	record := filepath.Join(infoDir, "doomed.txt.trashinfo")
	mustWrite(t, record, "[Trash Info]\nPath=/somewhere/else\nDeletionDate=2000-01-01T00:00:00\n")

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "doomed.txt")
	mustWrite(t, src, "mine")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "trash-excl",
		Kind:  operations.OpTrash,
		Items: []operations.Item{{Source: src}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "trash-excl")
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	if got := mustRead(t, record); got != "[Trash Info]\nPath=/somewhere/else\nDeletionDate=2000-01-01T00:00:00\n" {
		t.Errorf("existing record was overwritten: %q", got)
	}
	if got := mustRead(t, occupied); got != "not mine" {
		t.Errorf("existing trash entry was disturbed: %q", got)
	}
}

// TestEnqueuePreservesOrder pins FIFO behavior: jobs run in the order they
// were enqueued, not in goroutine-scheduling order.
func TestEnqueuePreservesOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	const count = 20
	mgr := operations.NewManager()
	for i := range count {
		enqueued := mgr.Enqueue(operations.Operation{
			ID:    "create-" + string(rune('a'+i)),
			Kind:  operations.OpCreateFile,
			Items: []operations.Item{{Target: filepath.Join(dir, "f"+string(rune('a'+i)))}},
		})
		if enqueued != nil {
			t.Fatal(enqueued)
		}
	}

	var order []string
	// Every job publishes one progress and one finished event; the loop
	// must count finished events, not channel receives, or it can return
	// while the worker is still creating files.
	for len(order) < count {
		ev := <-mgr.Events()
		fin, ok := ev.(operations.FinishedEvent)
		if !ok {
			continue // progress events are not interesting here
		}
		order = append(order, fin.Result.OpID)
	}
	for i, id := range order {
		if want := "create-" + string(rune('a'+i)); id != want {
			t.Fatalf("jobs ran out of order: got %v", order)
		}
	}
}

// TestCopyReadOnlyDirectory pins the destination-mode contract: a readable
// but non-writable source directory must copy completely and end up with
// the source's mode.
func TestCopyReadOnlyDirectory(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "ro")
	if mkdirErr := os.Mkdir(src, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(src, "inner.txt"), "inner")
	if chmodErr := os.Chmod(src, 0o555); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	t.Cleanup(func() {
		_ = os.Chmod(src, 0o755)
	})

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-ro",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: src, Target: filepath.Join(dstDir, "ro")}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}
	t.Cleanup(func() {
		// The copy deliberately ends read-only; restore it for cleanup.
		_ = os.Chmod(filepath.Join(dstDir, "ro"), 0o755)
	})

	result := syncMgr(t, mgr, "copy-ro")
	if result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	if got := mustRead(t, filepath.Join(dstDir, "ro", "inner.txt")); got != "inner" {
		t.Errorf("read-only directory copy = %q, want inner", got)
	}
	info, statErr := os.Stat(filepath.Join(dstDir, "ro"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0o555 {
		t.Errorf("copied mode = %v, want 555", info.Mode().Perm())
	}
}

// TestCopyMergeKeepsExistingDirectoryMode pins the merge case: merging into
// an existing directory must not rewrite its mode.
func TestCopyMergeKeepsExistingDirectoryMode(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "new.txt"), "new")

	dst := filepath.Join(dstDir, "sub")
	if mkdirErr := os.Mkdir(dst, 0o700); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "copy-merge",
		Kind:  operations.OpCopy,
		Items: []operations.Item{{Source: srcDir, Target: dst}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	// Merging into the existing directory requires answering its conflict
	// question with replace; the harness otherwise skips.
	result := func() operations.Result {
		for {
			ev := <-mgr.Events()
			if q, ok := ev.(operations.QuestionEvent); ok {
				q.AnswerCh <- operations.Answer{Action: operations.ConflictReplace}

				continue
			}
			if fin, ok := ev.(operations.FinishedEvent); ok {
				return fin.Result
			}
		}
	}()
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	if got := mustRead(t, filepath.Join(dst, "new.txt")); got != "new" {
		t.Errorf("merged content = %q, want new", got)
	}
	info, statErr := os.Stat(dst)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("merged directory mode = %v, want 700", info.Mode().Perm())
	}
}
