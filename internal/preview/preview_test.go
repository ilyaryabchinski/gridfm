package preview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestInspectFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	content := "first line\nsecond line\nthird line\n"
	if writeErr := os.WriteFile(path, []byte(content), 0o640); writeErr != nil {
		t.Fatal(writeErr)
	}
	wantTime := time.Unix(1700000000, 0)
	if err := os.Chtimes(path, wantTime, wantTime); err != nil {
		t.Fatal(err)
	}

	info, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "notes.txt" || info.IsDir || info.Symlink {
		t.Errorf("identity = %q dir=%v link=%v", info.Name, info.IsDir, info.Symlink)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", info.Size, len(content))
	}
	if !strings.HasPrefix(info.Mode, "-rw-r----") {
		t.Errorf("mode = %q, want rw-r---- prefix", info.Mode)
	}
	if info.Owner == "" || info.Group == "" {
		t.Errorf("owner = %q, group = %q, want resolution", info.Owner, info.Group)
	}
	if !info.ModTime.Equal(wantTime) {
		t.Errorf("mtime = %v, want %v", info.ModTime, wantTime)
	}
	want := []string{"first line", "second line", "third line"}
	if len(info.Preview) != len(want) {
		t.Fatalf("preview = %q, want %q", info.Preview, want)
	}
	for i := range want {
		if info.Preview[i] != want[i] {
			t.Errorf("preview[%d] = %q, want %q", i, info.Preview[i], want[i])
		}
	}
	if info.PreviewTruncated {
		t.Error("a three-line file is not truncated")
	}
}

func TestInspectDirectoryHasNoPreview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir || info.Preview != nil {
		t.Errorf("dir inspect: IsDir=%v preview=%v", info.IsDir, info.Preview)
	}
}

func TestInspectBinaryFileHasNoPreview(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(path, []byte("text\x00then binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Preview != nil {
		t.Errorf("binary preview = %q, want nil", info.Preview)
	}
}

func TestInspectSymlinkAndBrokenLink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("linked"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken")
	if err := os.Symlink(filepath.Join(dir, "ghost"), broken); err != nil {
		t.Fatal(err)
	}

	info, err := Inspect(link)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Symlink || info.LinkTarget != target {
		t.Errorf("link = %+v, want target %q", info, target)
	}

	info, err = Inspect(broken)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Symlink || !info.TargetMissing {
		t.Errorf("broken link = %+v, want TargetMissing", info)
	}
}

func TestInspectMissingPath(t *testing.T) {
	t.Parallel()

	if _, err := Inspect(filepath.Join(t.TempDir(), "ghost")); err == nil {
		t.Error("inspecting a missing path must fail")
	}
}

func TestInspectTruncatesLongPreview(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "long.txt")
	var b strings.Builder
	for range 40 {
		b.WriteString(strings.Repeat("x", 120) + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Preview) > maxPreviewLines {
		t.Errorf("preview lines = %d, want at most %d", len(info.Preview), maxPreviewLines)
	}
	if !info.PreviewTruncated {
		t.Error("a 40-line file must be marked truncated")
	}
	for _, line := range info.Preview {
		if utf8.RuneCountInString(line) > maxPreviewColumns {
			t.Errorf("preview line wider than %d: %q", maxPreviewColumns, line)
		}
	}
}
