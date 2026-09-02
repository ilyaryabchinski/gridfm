package operations

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// conflictResolver decides what to do when a target exists. Implementations
// may block awaiting the user; a resolved action of ConflictSkip makes the
// caller skip the item with ErrSkipped.
type conflictResolver interface {
	Resolve(ctx context.Context, target string) (Answer, error)
}

// copyEntry copies one filesystem entry recursively. Symlinks are copied
// as symlinks and never traversed; file modes and modification times are
// preserved where the filesystem allows.
func copyEntry(ctx context.Context, src, dst string, resolve conflictResolver) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat %q: %w", src, err)
	}

	action, err := ensureTarget(ctx, dst, resolve)
	if err != nil {
		return err
	}

	switch action {
	case ConflictSkip:
		return ErrSkipped
	case ConflictAbort:
		return ErrAborted
	case ConflictRename:
		dst = UniqueName(dst)
	case ConflictReplace:
	}

	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return copySymlink(src, dst)
	case info.IsDir():
		return copyDir(ctx, src, dst, info, resolve)
	default:
		return copyFile(ctx, src, dst, info)
	}
}

// ensureTarget decides how to treat an existing destination and creates
// nothing itself. It returns the (possibly rewritten) action to apply.
func ensureTarget(ctx context.Context, dst string, resolve conflictResolver) (ConflictAction, error) {
	_, statErr := os.Lstat(dst)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return ConflictReplace, nil // nothing to conflict with
		}

		return ConflictReplace, fmt.Errorf("stat %q: %w", dst, statErr)
	}

	answer, err := resolve.Resolve(ctx, dst)
	if err != nil {
		return ConflictReplace, fmt.Errorf("resolve conflict for %q: %w", dst, err)
	}

	return answer.Action, nil
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("readlink %q: %w", src, err)
	}

	err = os.Symlink(target, dst)
	if err != nil {
		return fmt.Errorf("symlink %q: %w", dst, err)
	}

	return nil
}

func copyDir(ctx context.Context, src, dst string, info fs.FileInfo, resolve conflictResolver) error {
	err := os.Mkdir(dst, info.Mode().Perm())
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("mkdir %q: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", src, err)
	}

	for _, entry := range entries {
		err100 := ctx.Err()
		if err100 != nil {
			return fmt.Errorf("copy %q interrupted: %w", src, err100)
		}

		childSrc := filepath.Join(src, entry.Name())
		childDst := filepath.Join(dst, entry.Name())
		copyErr := copyEntry(ctx, childSrc, childDst, resolve)
		if copyErr != nil {
			return copyErr
		}
	}

	// Preserve the directory mtime only after its contents are written.
	return preserveTimes(src, dst)
}

func copyFile(ctx context.Context, src, dst string, info fs.FileInfo) error {
	//nolint:gosec // paths come from user-selected entries; this is a file manager
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %q: %w", src, err)
	}
	defer srcFile.Close() //nolint:errcheck // read-only handle

	//nolint:gosec // the copied mode comes from the source entry
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}

	_, err = ioCopy(ctx, dstFile, srcFile)
	if err != nil {
		// The copy failed: drop the partial destination and report every
		// error encountered along the way.
		closeErr := dstFile.Close()
		removeErr := os.Remove(dst)
		err = errors.Join(err, closeErr, removeErr)
		if err != nil {
			return fmt.Errorf("copy %q: %w", src, err)
		}
	}

	err = dstFile.Close()
	if err != nil {
		return fmt.Errorf("close %q: %w", dst, err)
	}

	return preserveTimes(src, dst)
}

// moveEntry moves one entry. Same-filesystem moves use rename; a rename
// rejected with EXDEV falls back to copy followed by removal, and the
// source is only removed after the copy fully succeeds.
func moveEntry(ctx context.Context, src, dst string, resolve conflictResolver) error {
	action, err := ensureTarget(ctx, dst, resolve)
	if err != nil {
		return err
	}

	switch action {
	case ConflictSkip:
		return ErrSkipped
	case ConflictAbort:
		return ErrAborted
	case ConflictRename:
		dst = UniqueName(dst)
	case ConflictReplace:
	}

	err = renameFn(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errCrossDevice) {
		return fmt.Errorf("rename %q: %w", src, err)
	}

	// Cross-filesystem: copy first; only a fully successful copy removes
	// the source, so no failure mode loses data.
	copyErr := copyEntry(ctx, src, dst, resolve)
	if copyErr != nil {
		return copyErr
	}

	err100 := os.RemoveAll(src)
	if err100 != nil {
		return fmt.Errorf("remove copied source %q: %w", src, err)
	}

	return nil
}

// UniqueName generates a non-existing sibling name by inserting a suffix
// before the extension: report.txt becomes report (copy).txt.
func UniqueName(path string) string {
	_, err100 := os.Lstat(path)
	if err100 != nil {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 2; ; i++ {
		candidate := base + " (copy)" + ext
		if i > 2 {
			candidate = base + fmt.Sprintf(" (copy %d)", i) + ext
		}
		_, err100 := os.Lstat(candidate)
		if err100 != nil {
			return candidate
		}
	}
}

func preserveTimes(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat %q: %w", src, err)
	}

	err = os.Chtimes(dst, info.ModTime(), info.ModTime())
	if err != nil {
		return fmt.Errorf("chtimes %q: %w", dst, err)
	}

	return nil
}

// ValidateDestinations rejects any job whose target lies inside its own
// source tree, which would recursively copy the copy.
func ValidateDestinations(items []Item) error {
	for _, item := range items {
		src, err := filepath.Abs(item.Source)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", item.Source, err)
		}
		dst, err := filepath.Abs(item.Target)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", item.Target, err)
		}
		if dst == src || strings.HasPrefix(dst, src+string(filepath.Separator)) {
			return fmt.Errorf("%w: %q into %q", ErrDestinationInsideSource, dst, src)
		}
	}

	return nil
}
