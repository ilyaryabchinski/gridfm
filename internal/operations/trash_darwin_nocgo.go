//go:build darwin && !cgo

package operations

import (
	"context"
	"errors"
)

func trashItem(_ context.Context, _ string, _ conflictResolver, _ func(done, total int64)) error {
	return errors.New("macOS Trash requires a cgo-enabled build")
}
