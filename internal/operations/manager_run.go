package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
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
		m.inFlight.Add(-1)
	}()

	coordinator := &conflictCoordinator{publish: m.publish}

	result := Result{OpID: j.op.ID, Kind: j.op.Kind}
	accounts := 0

	for _, item := range j.op.Items {
		if ctx.Err() != nil {
			result.Cancelled = true

			break
		}

		// Announce the item before it runs: the shelf names the current
		// target and offers the cancel control from the first moment of a
		// long item, not only after it completes.
		reporter := &itemReporter{
			m:      m,
			ctx:    ctx,
			opID:   j.op.ID,
			kind:   j.op.Kind,
			done:   accounts,
			total:  len(j.op.Items),
			target: progressTarget(item),
		}
		if err := m.publish(ctx, reporter.started()); err != nil {
			result.Cancelled = true

			break
		}

		err := runItem(ctx, j.op.Kind, item, coordinator, reporter.bytes)
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
			result.Failures = append(result.Failures, ItemError{Path: itemPath(item), Err: err})
		default:
			result.Failed++
			result.Failures = append(result.Failures, ItemError{Path: itemPath(item), Err: err})
		}
		accounts++

		// A cancelled job stops advertising progress; the finished result
		// is the authoritative closing event.
		_ = m.publish(ctx, ProgressEvent{
			ID:     j.op.ID,
			Kind:   j.op.Kind,
			Done:   accounts,
			Total:  len(j.op.Items),
			Target: progressTarget(item),
		})

		if ctx.Err() != nil {
			result.Cancelled = true

			break
		}
	}

	m.publishResult(FinishedEvent{ID: j.op.ID, Result: finish(result, j.op, accounts)})
}

// progressTarget names the path a display shows for the item: the target
// when the kind carries one, otherwise the affected source.
func progressTarget(item Item) string {
	return itemPath(item)
}

// itemPath returns the path a failure should name: the target for
// destination-carrying kinds, the source for trash and delete.
func itemPath(item Item) string {
	if item.Target != "" {
		return item.Target
	}

	return item.Source
}

// itemReporter publishes progress for the running item: the start
// announcement, its completion, and throttled byte-level progress fed from
// the copy primitives.
type itemReporter struct {
	m      *Manager
	ctx    context.Context
	opID   string
	kind   Kind
	done   int
	total  int
	target string
	last   time.Time
}

func (r *itemReporter) started() ProgressEvent {
	return ProgressEvent{ID: r.opID, Kind: r.kind, Done: r.done, Total: r.total, Target: r.target}
}

// bytes reports intra-item copy progress. Publications are rate-limited by
// the manager's interval so a multi-gigabyte file cannot flood the
// consumer; the per-item events before and after always get through.
func (r *itemReporter) bytes(done, total int64) {
	if total <= 0 || done <= 0 {
		return
	}
	if gap := r.m.progressGap(); gap > 0 && time.Since(r.last) < gap {
		return
	}

	r.last = time.Now()
	_ = r.m.publish(r.ctx, ProgressEvent{
		ID:             r.opID,
		Kind:           r.kind,
		Done:           r.done,
		Total:          r.total,
		Target:         r.target,
		ItemBytes:      done,
		ItemBytesTotal: total,
	})
}

// finish closes out the counters: whatever was not accounted for did not
// run.
func finish(result Result, op Operation, accounts int) Result {
	result.NotRun = len(op.Items) - accounts

	return result
}

// runItem dispatches one item to its mutation primitive. report receives
// cumulative byte progress for kinds that copy bytes.
func runItem(ctx context.Context, kind Kind, item Item, resolve conflictResolver, report func(done, total int64)) error {
	switch kind {
	case OpCopy:
		return copyEntry(ctx, item.Source, item.Target, resolve, report)
	case OpMove:
		return moveEntry(ctx, item.Source, item.Target, resolve, report)
	case OpRename:
		return renameItem(ctx, item.Source, item.Target, resolve)
	case OpCreateFile:
		return createFile(item.Target)
	case OpCreateDir:
		return createDir(item.Target)
	case OpTrash:
		return trashItem(ctx, item.Source, resolve, report)
	case OpDelete:
		return deleteItem(ctx, item.Source)
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

// deleteItem permanently removes the entry and everything beneath it. The
// traversal checks the context between entries, so a cancelled job stops
// walking instead of finishing a huge deletion unnoticed. A path that
// vanished before the job ran is a successful no-op.
func deleteItem(ctx context.Context, src string) error {
	return removeAll(ctx, src)
}
