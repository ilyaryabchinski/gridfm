package operations

import (
	"os"
	"sync"
	"syscall"
)

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
