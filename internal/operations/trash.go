package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// trashItem moves src into the freedesktop.org home trash: the entry lands
// in Trash/files under a non-colliding name and a matching .trashinfo
// record in Trash/info records the original path and deletion time. The
// trash never overwrites anything. Cross-filesystem sources fall back to
// copy-then-remove, and the source is only removed after the copy fully
// succeeds.
func trashItem(ctx context.Context, src string, resolve conflictResolver) error {
	root, err := trashRoot()
	if err != nil {
		return err
	}

	filesDir := filepath.Join(root, "files")
	infoDir := filepath.Join(root, "info")
	for _, dir := range []string{filesDir, infoDir} {
		mkdirErr := os.MkdirAll(dir, 0o700)
		if mkdirErr != nil {
			return fmt.Errorf("prepare trash %q: %w", dir, mkdirErr)
		}
	}

	dst := UniqueName(filepath.Join(filesDir, filepath.Base(src)))

	err = renameFn(src, dst)
	if errors.Is(err, errCrossDevice) {
		copyErr := copyEntry(ctx, src, dst, resolve)
		if copyErr != nil {
			return copyErr
		}

		rmErr := os.RemoveAll(src)
		if rmErr != nil {
			return fmt.Errorf("remove trashed source %q: %w", src, rmErr)
		}
	} else if err != nil {
		return fmt.Errorf("trash %q: %w", src, err)
	}

	return writeTrashInfo(infoDir, filepath.Base(dst), src)
}

// trashRoot resolves the home trash location per the basedir
// specification.
func trashRoot() (string, error) {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "Trash"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home for trash: %w", err)
	}

	return filepath.Join(home, ".local", "share", "Trash"), nil
}

// writeTrashInfo records the original location and deletion time beside the
// trashed entry, as the specification requires for a valid entry.
func writeTrashInfo(infoDir, name, origPath string) error {
	abs, err := filepath.Abs(origPath)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", origPath, err)
	}

	content := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escapeTrashPath(abs),
		time.Now().Format("2006-01-02T15:04:05"),
	)

	infoPath := filepath.Join(infoDir, name+".trashinfo")
	writeErr := os.WriteFile(infoPath, []byte(content), 0o600)
	if writeErr != nil {
		return fmt.Errorf("write %q: %w", infoPath, writeErr)
	}

	return nil
}

// escapeTrashPath percent-encodes everything outside the unreserved set
// while keeping slashes, matching the Path field rules of the
// freedesktop.org trash specification.
func escapeTrashPath(path string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~/"

	out := make([]byte, 0, len(path)+8)
	for i := range len(path) {
		b := path[i]
		if strings.IndexByte(unreserved, b) >= 0 {
			out = append(out, b)

			continue
		}
		out = append(out, []byte(fmt.Sprintf("%%%02X", b))...)
	}

	return string(out)
}
