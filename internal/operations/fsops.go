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
// preserved where the filesystem allows. report, when non-nil, receives
// cumulative byte progress for the file being copied.
func copyEntry(ctx context.Context, src, dst string, resolve conflictResolver, report func(done, total int64)) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat %q: %w", src, err)
	}

	// Copying an entry onto itself (same path or a hard link to it) would
	// truncate the content being read; refuse instead of destroying it.
	if dstInfo, statErr := os.Lstat(dst); statErr == nil && os.SameFile(info, dstInfo) {
		return fmt.Errorf("%w: %q and %q", ErrSameFile, src, dst)
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
	}

	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		// Resolve the link before the destination is disturbed: a failing
		// readlink must never cost the existing entry.
		target, linkErr := os.Readlink(src)
		if linkErr != nil {
			return fmt.Errorf("readlink %q: %w", src, linkErr)
		}
		if replaceErr := replaceRemove(action, dst, info); replaceErr != nil {
			return replaceErr
		}

		return createSymlink(target, dst)
	case info.IsDir():
		if replaceErr := replaceRemove(action, dst, info); replaceErr != nil {
			return replaceErr
		}

		return copyDir(ctx, src, dst, info, resolve, report)
	default:
		// Named pipes block forever in Open with no writer, and devices
		// have nothing meaningful to copy: reject special entries before
		// anything is opened. Symlinks and directories are handled above.
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot copy %q: not a regular file", src)
		}
		// The source is opened before any replacement removal: an
		// unreadable or vanished source must not destroy the destination.
		return copyFile(ctx, src, dst, info, action, report)
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

// replaceRemove clears the way for an explicit replace by removing the
// destination entry itself instead of letting the copy follow it: a
// destination symlink is unlinked, never traversed into its target, and a
// destination hard link to the source is broken before anything is opened.
// A real destination directory survives: a source directory merges into it,
// and replacing a directory with a plain file is refused. Callers invoke
// it only after proving the replacement source is readable.
func replaceRemove(action ConflictAction, dst string, srcInfo fs.FileInfo) error {
	if action != ConflictReplace {
		return nil
	}

	dstInfo, err := os.Lstat(dst)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // vanished after the conflict was resolved
		}

		return fmt.Errorf("stat %q: %w", dst, err)
	}
	if dstInfo.IsDir() {
		if srcInfo.IsDir() {
			return nil // directory replaces directory by merging
		}

		return fmt.Errorf("cannot replace directory %q with a file", dst)
	}
	if err := os.Remove(dst); err != nil {
		return fmt.Errorf("remove %q: %w", dst, err)
	}

	return nil
}

func createSymlink(target, dst string) error {
	err := os.Symlink(target, dst)
	if err != nil {
		return fmt.Errorf("symlink %q: %w", dst, err)
	}

	return nil
}

func copyDir(ctx context.Context, src, dst string, info fs.FileInfo, resolve conflictResolver, report func(done, total int64)) error {
	// The destination is created owner-writable even when the source mode
	// is read-only, so its children can be populated; the source mode is
	// restored only after the contents are in place. An existing
	// destination survives as a merge target and keeps its own mode.
	created := true
	err := os.Mkdir(dst, info.Mode().Perm()|0o200)
	if err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("mkdir %q: %w", dst, err)
		}

		created = false
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", src, err)
	}

	// A skipped child must not strand its later siblings: keep copying and
	// report the partial outcome upward, so a directory merge can skip one
	// conflicting entry yet still copy everything after it.
	skipped := false
	for _, entry := range entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("copy %q interrupted: %w", src, ctxErr)
		}

		childSrc := filepath.Join(src, entry.Name())
		childDst := filepath.Join(dst, entry.Name())
		if copyErr := copyEntry(ctx, childSrc, childDst, resolve, report); copyErr != nil {
			if errors.Is(copyErr, ErrSkipped) {
				skipped = true

				continue
			}

			return copyErr
		}
	}

	if skipped {
		// The marker still matches ErrSkipped for result accounting, and a
		// cross-device move treats any error as "do not remove the source",
		// so a partially copied source always survives.
		return fmt.Errorf("%w: some children of %q were skipped", ErrSkipped, src)
	}

	if created {
		if chmodErr := os.Chmod(dst, info.Mode().Perm()); chmodErr != nil {
			return fmt.Errorf("chmod %q: %w", dst, chmodErr)
		}
	}

	// Preserve the directory mtime only after its contents are written.
	return preserveTimes(src, dst)
}

func copyFile(ctx context.Context, src, dst string, info fs.FileInfo, action ConflictAction, report func(done, total int64)) error {
	//nolint:gosec // paths come from user-selected entries; this is a file manager
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %q: %w", src, err)
	}
	defer srcFile.Close() //nolint:errcheck // read-only handle

	// The source is open and readable; only now is the existing entry removed.
	if replaceErr := replaceRemove(action, dst, info); replaceErr != nil {
		return replaceErr
	}

	//nolint:gosec // the copied mode comes from the source entry
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}

	var written int64
	total := info.Size()
	_, err = ioCopy(ctx, writeFunc(func(p []byte) (int, error) {
		n, writeErr := dstFile.Write(p)
		written += int64(n)
		if report != nil {
			report(written, total)
		}

		return n, writeErr
	}), srcFile)
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
func moveEntry(ctx context.Context, src, dst string, resolve conflictResolver, report func(done, total int64)) error {
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
	copyErr := copyEntry(ctx, src, dst, resolve, report)
	if copyErr != nil {
		return copyErr
	}

	removeErr := removeAll(ctx, src)
	if removeErr != nil {
		return fmt.Errorf("remove copied source %q: %w", src, removeErr)
	}

	return nil
}

// UniqueName generates a non-existing sibling name by inserting a suffix
// before the extension: report.txt becomes report (copy).txt.
func UniqueName(path string) string {
	if _, err := os.Lstat(path); err != nil {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 2; ; i++ {
		candidate := base + " (copy)" + ext
		if i > 2 {
			candidate = base + fmt.Sprintf(" (copy %d)", i) + ext
		}
		if _, err := os.Lstat(candidate); err != nil {
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

// removeAll removes path and everything beneath it, checking the context
// between entries so a cancelled job stops walking instead of finishing a
// huge deletion unnoticed. A missing path is a successful no-op, matching
// os.RemoveAll. Symlinks are removed as entries, never traversed.
func removeAll(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("lstat %q: %w", path, err)
	}

	if info.IsDir() {
		entries, readErr := os.ReadDir(path)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return fmt.Errorf("read dir %q: %w", path, readErr)
		}

		for _, entry := range entries {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr //nolint:wrapcheck // callers match errors.Is(context.Canceled)
			}

			child := filepath.Join(path, entry.Name())
			if rmErr := removeAll(ctx, child); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
				return rmErr
			}
		}
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr //nolint:wrapcheck // callers match errors.Is(context.Canceled)
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %q: %w", path, err)
	}

	return nil
}

// ValidateDestinations rejects any job whose target lies inside its own
// source tree, which would recursively copy the copy. Both sides are
// resolved through their existing symlinked ancestors first, so an alias
// pointing into the source cannot smuggle a self-copy past the lexical
// comparison.
func ValidateDestinations(items []Item) error {
	for _, item := range items {
		src, err := resolveReal(item.Source)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", item.Source, err)
		}
		dst, err := resolveReal(item.Target)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", item.Target, err)
		}
		if dst == src || strings.HasPrefix(dst, src+string(filepath.Separator)) {
			return fmt.Errorf("%w: %q into %q", ErrDestinationInsideSource, dst, src)
		}
	}

	return nil
}

// resolveReal returns the absolute path with every existing ancestor's
// symlinks resolved. Missing tail components cannot be resolved and are
// rejoined lexically onto the resolved prefix; if nothing along the path
// exists, the lexical absolute path is returned.
func resolveReal(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	tail := []string{}
	prefix := abs
	for {
		resolved, resolveErr := filepath.EvalSymlinks(prefix)
		if resolveErr == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}

		parent := filepath.Dir(prefix)
		if parent == prefix {
			return abs, nil // reached the root without resolving; compare lexically
		}
		tail = append([]string{filepath.Base(prefix)}, tail...)
		prefix = parent
	}
}
