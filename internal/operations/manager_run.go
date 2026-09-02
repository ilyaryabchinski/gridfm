package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// runJob executes one job to completion: serial items, conflict questions,
// per-item progress, and an accurate final result.
func (m *Manager) runJob(j *job) {
	ctx, cancel := context.WithCancel(context.Background())
	j.hooks = jobHooks{cancel: cancel}
	m.active.Store(j)

	defer func() {
		cancel()
		m.active.Store(nil)
		<-m.busy
	}()

	coordinator := &conflictCoordinator{publish: m.publish}

	result := Result{OpID: j.op.ID, Kind: j.op.Kind}
	accounts := 0

	for _, item := range j.op.Items {
		if ctx.Err() != nil {
			result.Cancelled = true

			break
		}

		err := runItem(ctx, j.op.Kind, item, coordinator)
		switch {
		case err == nil:
			result.Succeeded++
		case errors.Is(err, ErrSkipped):
			result.Skipped++
		case errors.Is(err, ErrAborted):
			result.Cancelled = true
			m.publishResult(FinishedEvent{ID: j.op.ID, Result: finish(result, j.op, accounts)})

			return
		case errors.Is(err, context.Canceled):
			result.Cancelled = true
			result.Failed++
			result.Failures = append(result.Failures, ItemError{Path: item.Target, Err: err})
		default:
			result.Failed++
			result.Failures = append(result.Failures, ItemError{Path: item.Target, Err: err})
		}
		accounts++

		// A cancelled job stops advertising progress; the finished result
		// is the authoritative closing event.
		_ = m.publish(ctx, ProgressEvent{
			ID:     j.op.ID,
			Kind:   j.op.Kind,
			Done:   accounts,
			Total:  len(j.op.Items),
			Target: item.Target,
		})

		if ctx.Err() != nil {
			result.Cancelled = true

			break
		}
	}

	m.publishResult(FinishedEvent{ID: j.op.ID, Result: finish(result, j.op, accounts)})
}

// finish closes out the counters: whatever was not accounted for did not
// run.
func finish(result Result, op Operation, accounts int) Result {
	result.NotRun = len(op.Items) - accounts

	return result
}

// runItem dispatches one item to its mutation primitive.
func runItem(ctx context.Context, kind Kind, item Item, resolve conflictResolver) error {
	switch kind {
	case OpCopy:
		return copyEntry(ctx, item.Source, item.Target, resolve)
	case OpMove:
		return moveEntry(ctx, item.Source, item.Target, resolve)
	case OpRename:
		return renameItem(ctx, item.Source, item.Target, resolve)
	case OpCreateFile:
		return createFile(item.Target)
	case OpCreateDir:
		return createDir(item.Target)
	case OpTrash:
		return trashItem(ctx, item.Source, resolve)
	case OpDelete:
		return deleteItem(item.Source)
	}

	return fmt.Errorf("%w: %d", ErrUnknownKind, kind)
}

func renameItem(ctx context.Context, src, dst string, resolve conflictResolver) error {
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

	renameErr := renameFn(src, dst)
	if renameErr != nil {
		return fmt.Errorf("rename %q: %w", src, renameErr)
	}

	return nil
}

func createFile(path string) error {
	//nolint:gosec // 0666 with the process umask is the expected mode for user files
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}

	closeErr := file.Close()
	if closeErr != nil {
		return fmt.Errorf("close %q: %w", path, closeErr)
	}

	return nil
}

func createDir(path string) error {
	//nolint:gosec // 0755 with the process umask is the expected mode for user directories
	mkdirErr := os.Mkdir(path, 0o755)
	if mkdirErr != nil {
		return fmt.Errorf("mkdir %q: %w", path, mkdirErr)
	}

	return nil
}

func deleteItem(src string) error {
	removeErr := os.RemoveAll(src)
	if removeErr != nil {
		return fmt.Errorf("delete %q: %w", src, removeErr)
	}

	return nil
}
