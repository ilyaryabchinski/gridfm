// Package preview collects inspector metadata for one filesystem entry:
// identity, size, permissions, ownership, timestamps, symlink targets, and
// a bounded text preview. Inspect runs off the update loop and never
// renders.
package preview

import (
	"bytes"

	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Preview bounds: a window of lines from the head of small text files.
const (
	maxPreviewRead    = 8192
	maxPreviewLines   = 12
	maxPreviewColumns = 64
)

// Info is the collected metadata for one inspected entry.
type Info struct {
	Path     string
	Name     string
	IsDir    bool
	Symlink  bool
	Size     int64
	Mode     string
	Owner    string
	Group    string
	ModTime  time.Time
	ReadOnly bool

	// LinkTarget is a symlink's stored target, empty for other kinds.
	LinkTarget string
	// TargetMissing reports a symlink whose target does not exist.
	TargetMissing bool

	// Preview holds up to maxPreviewLines from the head of a small text
	// file; nil for directories, binaries, and anything unreadable.
	Preview []string
	// PreviewTruncated reports that the file had more lines or wider
	// lines than the preview window shows.
	PreviewTruncated bool
}

// Inspect collects the metadata for path. Symlinks are described as
// themselves: the target name is recorded, never followed.
func Inspect(path string) (*Info, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %q: %w", path, err)
	}

	out := &Info{
		Path:     path,
		Name:     filepath.Base(path),
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Mode:     info.Mode().String(),
		ModTime:  info.ModTime(),
		ReadOnly: info.Mode().Perm()&0o222 == 0,
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		out.Symlink = true
		out.LinkTarget, err = os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("readlink %q: %w", path, err)
		}
		// A link target stat failure usually means the link is broken;
		// other errors keep the flag off and the entry stays inspectable.
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			out.TargetMissing = true
		}

		return out, nil
	}

	out.Owner, out.Group = ownerGroup(path, info)
	if !out.IsDir {
		out.Preview, out.PreviewTruncated, err = readPreview(path)
		if err != nil {
			// An unreadable file is still inspectable; it just has no
			// preview.
			out.Preview, out.PreviewTruncated = nil, false
		}
	}

	return out, nil
}

// readPreview returns the head of a text file. A NUL byte in the first
// window marks binary content and returns nil without an error.
func readPreview(path string) ([]string, bool, error) {
	file, err := os.Open(path) //nolint:gosec // paths come from user-selected entries
	if err != nil {
		return nil, false, err
	}
	defer file.Close() //nolint:errcheck // read-only handle

	head := make([]byte, maxPreviewRead)
	n, readErr := io.ReadFull(file, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return nil, false, readErr
	}
	head = head[:n]
	if n == 0 {
		return nil, false, nil // empty file: nothing to preview
	}
	if bytes.ContainsRune(head, 0) {
		return nil, false, nil // binary
	}

	truncated := false
	text := string(head)
	if n == maxPreviewRead { // may have more content
		if _, err := file.Seek(0, io.SeekCurrent); err == nil {
			var extra [1]byte
			if _, ioErr := file.Read(extra[:]); ioErr == nil {
				truncated = true
			}
		}
	}

	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // trailing newline is not a line
	}
	if len(lines) > maxPreviewLines {
		lines = lines[:maxPreviewLines]
		truncated = true
	}
	for i, line := range lines {
		if len(line) > maxPreviewColumns {
			lines[i] = line[:maxPreviewColumns-1] + "…"
			truncated = true
		}
	}

	return lines, truncated, nil
}
