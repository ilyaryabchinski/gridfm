package operations

import (
	"context"
	"fmt"
	"sync/atomic"
)

// Manager runs mutation jobs strictly one at a time. Jobs are enqueued from
// anywhere; progress, conflict questions, and results are published as
// events the application consumes. Question answering and cancellation are
// safe from any goroutine.
type Manager struct {
	events chan Event
	queue  chan *job
	active atomic.Pointer[job]

	// busy tracks jobs from enqueue until finish.
	busy chan struct{}
}

// Event is one manager publication: progress, a conflict question, or a
// finished result.
type Event interface {
	OpID() string
}

// ProgressEvent reports per-item advancement of the active job.
type ProgressEvent struct {
	ID     string
	Kind   Kind
	Done   int
	Total  int
	Target string
}

// OpID identifies the job the progress belongs to.
func (e ProgressEvent) OpID() string { return e.ID }

// QuestionEvent asks the user how to resolve an existing target. The
// answer must eventually be sent on AnswerCh, or the job stays blocked.
// The manager is serial, so the question belongs to the active job.
type QuestionEvent struct {
	Target   string
	AnswerCh chan<- Answer
}

// OpID is empty: the manager is serial, so the question belongs to the
// active job the application already tracks.
func (e QuestionEvent) OpID() string { return "" }

// FinishedEvent carries the final, accurate result of a job.
type FinishedEvent struct {
	ID     string
	Result Result
}

// OpID identifies the job the result belongs to.
func (e FinishedEvent) OpID() string { return e.ID }

type job struct {
	op    Operation
	hooks jobHooks
}

// jobHooks let the manager cancel a running job.
type jobHooks struct {
	cancel context.CancelFunc
}

// Operation is a requested mutation.
type Operation struct {
	ID    string
	Kind  Kind
	Items []Item
}

// NewManager starts the serial job worker.
func NewManager() *Manager {
	// The event stream is unbuffered: progress events synchronize the
	// worker with the consumer, which makes cancellation deterministic.
	m := &Manager{
		events: make(chan Event),
		queue:  make(chan *job),
		busy:   make(chan struct{}, 1024),
	}

	go m.worker()

	return m
}

// Events exposes the event stream for the application to consume.
func (m *Manager) Events() <-chan Event { return m.events }

// Enqueue adds a job to the serial queue after validating it. Destinations
// inside their own source tree are rejected before anything runs.
func (m *Manager) Enqueue(op Operation) error {
	validateErr := ValidateDestinations(op.Items)
	if validateErr != nil {
		return validateErr
	}
	if len(op.Items) == 0 {
		return fmt.Errorf("%w: %q", ErrEmptyOperation, op.ID)
	}

	select {
	case m.busy <- struct{}{}:
	default:
	}

	go func() {
		m.queue <- &job{op: op}
	}()

	return nil
}

// Busy reports whether any job is queued or running.
func (m *Manager) Busy() bool {
	return len(m.busy) > 0
}

// CancelActive cancels the running job, if any. Queued jobs are not
// affected; each is cancellable once it starts.
func (m *Manager) CancelActive() {
	for _, j := range m.current() {
		j.hooks.cancel()
	}
}

// current snapshots the single running job, if any. The serial worker
// guarantees at most one.
func (m *Manager) current() []*job {
	if j := m.active.Load(); j != nil {
		return []*job{j}
	}

	return nil
}

func (m *Manager) worker() {
	for j := range m.queue {
		m.runJob(j)
	}
}

// publish sends a progress or question event, synchronously waiting for
// the application to consume the previous one. When the job is cancelled
// the event is dropped; finished results are always delivered with
// publishResult.
func (m *Manager) publish(ctx context.Context, ev Event) error {
	select {
	case m.events <- ev:
		return nil
	case <-ctx.Done():
		//nolint:wrapcheck // callers match this with errors.Is(context.Canceled)
		return ctx.Err()
	}
}

// publishResult delivers the final result of a job unconditionally.
func (m *Manager) publishResult(ev Event) {
	m.events <- ev
}
