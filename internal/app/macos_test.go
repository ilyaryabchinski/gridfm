//go:build darwin

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppBundleOpensAsDesktopTarget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "Example.app")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	msg := openEntryCmd(1, path)()
	if _, ok := msg.(EntryResolvedMsg); !ok {
		t.Fatalf("app bundle resolved as %T, want EntryResolvedMsg", msg)
	}
}
