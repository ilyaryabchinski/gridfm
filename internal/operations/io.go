package operations

import (
	"context"
	"io"
	"os"
	"syscall"
)

// renameFn is the filesystem rename primitive, swappable for tests that
// simulate cross-device moves.
//
//nolint:gochecknoglobals // test seam
var renameFn = os.Rename

// errCrossDevice is the errno a rename returns when the target lives on a
// different filesystem than the source.
var errCrossDevice = syscall.EXDEV // fixed platform errno

// ioCopy copies while watching for cancellation: a cancelled context fails
// the very next read instead of finishing a potentially huge file.
func ioCopy(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	copyErr := ctx.Err()
	if copyErr != nil {
		return 0, copyErr //nolint:wrapcheck // cancellation is matched with errors.Is upstream
	}

	return io.Copy( //nolint:wrapcheck // copyFile wraps the error with the source path
		dst,
		readFunc(func(p []byte) (int, error) {
			readErr := ctx.Err()
			if readErr != nil {
				return 0, readErr //nolint:wrapcheck // cancellation is matched with errors.Is upstream
			}

			return src.Read(p)
		},
		))
}

// readFunc adapts a function to io.Reader.
type readFunc func(p []byte) (int, error)

func (f readFunc) Read(p []byte) (int, error) { return f(p) }
