package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

// TestValidateOperationScopesDescendantCheck keeps the source-target
// descendant check on the kinds that carry both paths. Create, trash, and
// delete leave one side empty, and an empty path resolves to the process
// working directory, which used to reject every operation at or beneath
// the launch location.
func TestValidateOperationScopesDescendantCheck(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	create := Operation{ID: "create-here", Kind: OpCreateFile,
		Items: []Item{{Target: filepath.Join(cwd, "probe.txt")}}}
	if err := validateOperation(create); err != nil {
		t.Errorf("create beneath the working directory rejected: %v", err)
	}

	trashOp := Operation{ID: "trash-here", Kind: OpTrash, Items: []Item{{Source: cwd}}}
	if err := validateOperation(trashOp); err != nil {
		t.Errorf("trash of a working-directory path rejected: %v", err)
	}

	deleteOp := Operation{ID: "delete-here", Kind: OpDelete, Items: []Item{{Source: cwd}}}
	if err := validateOperation(deleteOp); err != nil {
		t.Errorf("delete of a working-directory path rejected: %v", err)
	}

	copyIntoSelf := Operation{ID: "copy-bad", Kind: OpCopy,
		Items: []Item{{Source: cwd, Target: filepath.Join(cwd, "inside")}}}
	if err := validateOperation(copyIntoSelf); !errors.Is(err, ErrDestinationInsideSource) {
		t.Errorf("copy into its own descendant must stay rejected, got %v", err)
	}

	empty := Operation{ID: "empty", Kind: OpCopy}
	if err := validateOperation(empty); !errors.Is(err, ErrEmptyOperation) {
		t.Errorf("empty operation must stay rejected, got %v", err)
	}
}

// SwapRenameWithEXDEVForTest forces renames to fail with the EXDEV errno
// rename returns across filesystems, wrapped exactly like os.Rename wraps
// it. Returns a restore function.
func SwapRenameWithEXDEVForTest() func() {
	return SwapRenameForTest(func(_, _ string) error {
		return &os.LinkError{Op: "rename", Err: syscall.EXDEV}
	})
}

var renameMu sync.Mutex

// SwapRenameForTest replaces the rename primitive (for example to force a
// cross-device move) and returns a restore function.
func SwapRenameForTest(f func(from, to string) error) func() {
	renameMu.Lock()
	previous := renameFn
	renameFn = f

	return func() {
		renameFn = previous
		renameMu.Unlock()
	}
}

// cancelAfter is a context.Context that reports cancellation once Err has
// been queried more than `after` times, making traversal cancellation
// deterministic: each directory entry checks Err exactly once.
type cancelAfter struct {
	context.Context

	after   int
	queries int
}

func (c *cancelAfter) Err() error {
	c.queries++
	if c.queries > c.after {
		return context.Canceled
	}

	return nil
}

// TestRemoveAllStopsWhenCancelled pins that a cancelled delete stops
// walking: entries after the cancellation point survive.
func TestRemoveAllStopsWhenCancelled(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "tree")
	if mkdirErr := os.Mkdir(tree, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	const files = 8
	for i := range files {
		if writeErr := os.WriteFile(filepath.Join(tree, fmt.Sprintf("f%d", i)), []byte("x"), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	// Cancel once the context has been consulted twice: once before the
	// first child removal and once at the recursion's own check, so the
	// walk stops deterministically after the first child.
	ctx := &cancelAfter{Context: context.Background(), after: 2}
	err := removeAll(ctx, tree)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("removeAll = %v, want context.Canceled", err)
	}

	entries, readErr := os.ReadDir(tree)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) == 0 {
		t.Error("cancellation must leave the remaining entries in place")
	}
	if len(entries) >= files {
		t.Errorf("no progress should have been made, found all %d files", files)
	}

	// An already-cancelled context removes nothing at all.
	full := &cancelAfter{Context: context.Background(), after: 0}
	if err := removeAll(full, tree); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled removeAll = %v, want context.Canceled", err)
	}
	entries, readErr = os.ReadDir(tree)
	if readErr != nil || len(entries) != files-1 {
		t.Errorf("pre-cancelled removeAll must not delete, found %v, %v", entries, readErr)
	}
}

// TestRemoveAllTreatsMissingAsDone pins RemoveAll parity: a missing path
// is a successful no-op.
func TestRemoveAllTreatsMissingAsDone(t *testing.T) {
	dir := t.TempDir()
	if err := removeAll(context.Background(), filepath.Join(dir, "ghost")); err != nil {
		t.Errorf("removing a missing path = %v, want nil", err)
	}
}
