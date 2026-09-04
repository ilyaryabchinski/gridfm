package operations

import (
	"errors"
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
