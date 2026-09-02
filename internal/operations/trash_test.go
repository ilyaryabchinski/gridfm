package operations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gridfm/internal/operations"
)

func TestTrashMovesFileAndWritesTrashInfo(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "doomed.txt")
	mustWrite(t, src, "gone soon")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "trash-1",
		Kind:  operations.OpTrash,
		Items: []operations.Item{{Source: src}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "trash-1")
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}
	_, statErr := os.Stat(src)
	if !os.IsNotExist(statErr) {
		t.Fatal("trashed source should be gone")
	}

	dataHome := filepath.Join(t.TempDir(), "data")
	_ = dataHome
	trashFiles := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "files")
	trashInfo := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "info")

	entries, err := os.ReadDir(trashFiles)
	if err != nil || len(entries) != 1 {
		t.Fatalf("trash files entries = %v, %v; want one", entries, err)
	}
	name := entries[0].Name()
	if name != "doomed.txt" {
		t.Errorf("trashed name = %q, want doomed.txt", name)
	}

	info := mustRead(t, filepath.Join(trashInfo, name+".trashinfo"))
	if !strings.Contains(info, "[Trash Info]") {
		t.Errorf("trashinfo missing header: %q", info)
	}
	// Temp paths contain only unreserved characters, so the recorded path
	// matches the source verbatim.
	if !strings.Contains(info, "Path="+src) {
		t.Errorf("trashinfo should record the original path %q, got %q", src, info)
	}
	if !strings.Contains(info, "DeletionDate=2") {
		t.Errorf("trashinfo should record a deletion date, got %q", info)
	}
}

func TestTrashDirectoryAndCollisionHandling(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))

	srcDir := t.TempDir()
	sub := filepath.Join(srcDir, "folder")
	mkdirErr := os.Mkdir(sub, 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(sub, "inner.txt"), "inner")

	// A pre-existing trash entry with the same name must not be touched.
	trashFiles := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "files")
	mkdirAllErr := os.MkdirAll(trashFiles, 0o700)
	if mkdirAllErr != nil {
		t.Fatal(mkdirAllErr)
	}
	existing := filepath.Join(trashFiles, "folder")
	mkdirErr = os.Mkdir(existing, 0o755)
	if mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	mustWrite(t, filepath.Join(existing, "old.txt"), "old")

	mgr := operations.NewManager()
	enqueueErr := mgr.Enqueue(operations.Operation{
		ID:    "trash-2",
		Kind:  operations.OpTrash,
		Items: []operations.Item{{Source: sub}},
	})
	if enqueueErr != nil {
		t.Fatal(enqueueErr)
	}

	result := syncMgr(t, mgr, "trash-2")
	if result.Succeeded != 1 {
		t.Fatalf("result = %+v, want 1 succeeded", result)
	}

	if got := mustRead(t, filepath.Join(existing, "old.txt")); got != "old" {
		t.Errorf("existing trash entry was disturbed: %q", got)
	}
	// The new entry got a unique name with its contents intact.
	newEntry := filepath.Join(trashFiles, "folder (copy)", "inner.txt")
	if got := mustRead(t, newEntry); got != "inner" {
		t.Errorf("nested content = %q at %q", got, newEntry)
	}
}
