//go:build darwin && !cgo

package operations

import "errors"

func copyMetadata(_, _ string) error {
	return errors.New("macOS metadata copy requires a cgo-enabled build")
}
