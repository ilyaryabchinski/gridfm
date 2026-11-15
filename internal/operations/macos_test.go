//go:build darwin && cgo

package operations_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/operations"

	"golang.org/x/sys/unix"
)

func TestRejectsCaseAliasInsideSourceOnMac(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "Source")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "source")
	if _, err := os.Stat(alias); err != nil {
		t.Skip("test volume is case-sensitive")
	}

	err := operations.ValidateDestinations([]operations.Item{{
		Source: src, Target: filepath.Join(alias, "child"),
	}})
	if !errors.Is(err, operations.ErrDestinationInsideSource) {
		t.Fatalf("case alias was accepted: %v", err)
	}
}

func TestCopyPreservesMacExtendedAttributes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "source")
	dst := filepath.Join(root, "copy")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	const attribute = "com.gridfm.test"
	if err := unix.Setxattr(src, attribute, []byte("metadata"), 0); err != nil {
		t.Fatal(err)
	}

	mgr := operations.NewManager()
	if err := mgr.Enqueue(operations.Operation{
		ID: "copy-xattr", Kind: operations.OpCopy,
		Items: []operations.Item{{Source: src, Target: dst}},
	}); err != nil {
		t.Fatal(err)
	}
	if result := syncMgr(t, mgr, "copy-xattr"); result.Succeeded != 1 {
		t.Fatalf("copy failed: %+v", result)
	}
	data := make([]byte, 32)
	n, err := unix.Getxattr(dst, attribute, data)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:n]) != "metadata" {
		t.Fatalf("copied attribute = %q, want metadata", data[:n])
	}
}

func TestRenameDirectoryChangesCaseOnMac(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "Folder")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "folder")
	if _, err := os.Stat(dst); err != nil {
		t.Skip("test volume is case-sensitive")
	}
	mgr := operations.NewManager()
	if err := mgr.Enqueue(operations.Operation{
		ID: "case-rename", Kind: operations.OpRename,
		Items: []operations.Item{{Source: src, Target: dst}},
	}); err != nil {
		t.Fatal(err)
	}
	if result := syncMgr(t, mgr, "case-rename"); result.Succeeded != 1 {
		t.Fatalf("case-only rename failed: %+v", result)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "folder" {
		t.Fatalf("directory names after rename = %v", entries)
	}
}
