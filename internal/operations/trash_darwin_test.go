//go:build darwin && cgo

package operations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTrashMovesFileAndReturnsDestination(t *testing.T) {
	t.Parallel()

	src := filepath.Join(t.TempDir(), "gridfm-trash-test")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	dst, err := trashNative(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dst) })
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source remains after trash: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "data" {
		t.Fatalf("trashed file = %q, %v", data, err)
	}
}
