package app

import (
	"io"
	"os"
	"sync"

	"gridfm/internal/graphics/kitty"
	"gridfm/internal/thumbs"
)

// ThumbJob asks the loader for one thumbnail. Cols and Rows are the
// coverage in terminal cells; the loader converts to pixels using the
// terminal's real cell size.
type ThumbJob struct {
	Path       string
	Key        string
	Size       int64
	MtimeNanos int64
	Cols       int
	Rows       int
}

// ThumbReadyMsg carries one finished thumbnail back to the model. PNG may
// be nil when generation failed; the card then keeps its icon.
type ThumbReadyMsg struct {
	Key string
	PNG []byte
}

// imgUpdate is the pending work state for the sync loop, guarded by mu.
// valid marks a slots snapshot as present (nil is meaningful: it clears
// placements). reset is sticky until consumed — a slots update must
// never erase it, or stale screen assumptions survive a resize or an
// external viewer. final ends the loop after writing the last delete.
type imgUpdate struct {
	valid bool
	slots []kitty.Slot
	reset bool
	final bool
}

// ImageSync owns everything that must not race the render loop: the
// kitty placement table (the truth about what is on screen), the writes
// of protocol bytes, and bounded thumbnail generation. The model only
// ships immutable slot lists and job requests.
type ImageSync struct {
	mu       sync.Mutex
	pend     imgUpdate
	kick     chan struct{}
	done     chan struct{}
	jobs     chan ThumbJob
	inFlight map[string]bool
	notify   func(ThumbReadyMsg)
	store    *thumbs.Store
	limits   thumbs.Limits
	quitting bool
	// cellW and cellH are the terminal cell size in pixels used to size
	// generated thumbnails.
	cellW, cellH int

	table *kitty.Table
	out   io.Writer
}

// NewImageSync starts the sync loop and two generation workers. notify is
// called from worker goroutines with each finished thumbnail — wire it to
// Program.Send. store may be nil for a memory-only setup (tests).
func NewImageSync(out io.Writer, store *thumbs.Store, cellW, cellH int, notify func(ThumbReadyMsg)) *ImageSync {
	if cellW <= 0 || cellH <= 0 {
		// Terminals that do not report a pixel size get a common default.
		cellW, cellH = 10, 20
	}

	s := &ImageSync{
		kick:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		jobs:     make(chan ThumbJob, 256),
		inFlight: map[string]bool{},
		notify:   notify,
		store:    store,
		limits:   thumbs.Default,
		table:    kitty.NewTable(),
		out:      out,
		cellW:    cellW,
		cellH:    cellH,
	}

	go s.loop()
	for range 2 {
		go s.worker()
	}

	return s
}

// ship queues an update, never blocking: the latest slots snapshot wins,
// but a pending reset is sticky — coalescing must not let a placement
// update discard it.
func (s *ImageSync) ship(u imgUpdate) {
	s.mu.Lock()
	if s.quitting {
		s.mu.Unlock()

		return
	}
	if u.reset {
		s.pend.reset = true
	}
	if u.valid {
		s.pend.valid = true
		s.pend.slots = u.slots
	}
	s.mu.Unlock()

	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Slots ships a desired placement set.
func (s *ImageSync) Slots(slots []kitty.Slot) { s.ship(imgUpdate{valid: true, slots: slots}) }

// Reset demands the terminal's graphics state be treated as garbage — an
// external viewer ran, or the terminal was resized — so the next frame
// re-uploads everything from scratch.
func (s *ImageSync) Reset() { s.ship(imgUpdate{reset: true}) }

// Load requests thumbnail generation for one entry; repeated requests for
// the same key while one is in flight are dropped. Results arrive via
// notify as ThumbReadyMsg.
func (s *ImageSync) Load(job ThumbJob) {
	s.mu.Lock()
	if s.quitting || s.inFlight[job.Key] {
		s.mu.Unlock()

		return
	}
	s.inFlight[job.Key] = true
	s.mu.Unlock()

	select {
	case s.jobs <- job:
	default:
		// Queue full: drop the request; the next scroll re-asks.
		s.mu.Lock()
		delete(s.inFlight, job.Key)
		s.mu.Unlock()
	}
}

// Stop drains any pending work, writes the final delete-all through the
// loop so it can never interleave with placement output, and shuts
// everything down. Safe to call once; called after the program loop has
// exited, so no more updates arrive.
func (s *ImageSync) Stop() {
	s.mu.Lock()
	if s.quitting {
		s.mu.Unlock()

		return
	}
	s.quitting = true
	s.pend.final = true
	s.mu.Unlock()

	close(s.jobs) // workers drain in-flight jobs and exit

	select {
	case s.kick <- struct{}{}:
	default:
	}
	<-s.done // the loop consumed the final update and wrote the delete
}

// loop serializes protocol output and keeps the table as screen truth.
// It owns the table exclusively: all other goroutines communicate
// through the pending update.
func (s *ImageSync) loop() {
	defer close(s.done)

	for range s.kick {
		for {
			s.mu.Lock()
			u := s.pend
			s.pend.valid = false
			s.pend.reset = false
			s.mu.Unlock()

			if u.reset {
				// The screen's images are garbage: delete everything and
				// forget every upload so the next sync re-transmits.
				s.write(kitty.EncodeDeleteAll())
				s.table = kitty.NewTable()
			}
			if u.valid {
				payload, err := s.table.Sync(u.slots)
				if err == nil && len(payload) > 0 {
					s.write(payload)
				}
			}
			if u.final {
				s.write(kitty.EncodeDeleteAll())

				return
			}
			if !u.valid && !u.reset {
				break // nothing left in this batch; wait for the next kick
			}
		}
	}
}

// write emits one wrapped batch: cursor saved and restored so placement
// never moves the real cursor away from where the TUI renderer expects
// it.
func (s *ImageSync) write(payload []byte) {
	buf := make([]byte, 0, len(kitty.CursorSave)+len(payload)+len(kitty.CursorRestore))
	buf = append(buf, kitty.CursorSave...)
	buf = append(buf, payload...)
	buf = append(buf, kitty.CursorRestore...)
	_, _ = s.out.Write(buf)
}

// worker generates thumbnails one at a time per goroutine: bounded
// parallelism, bounded memory.
func (s *ImageSync) worker() {
	for job := range s.jobs {
		png := s.generate(job)
		if s.notify != nil {
			s.notify(ThumbReadyMsg{Key: job.Key, PNG: png})
		}

		s.mu.Lock()
		delete(s.inFlight, job.Key)
		s.mu.Unlock()
	}
}

// generate resolves the thumbnail: memory via the disk cache when it has
// seen the file version, otherwise bounded decoding.
func (s *ImageSync) generate(job ThumbJob) []byte {
	entry := thumbs.Entry{Path: job.Path, Size: job.Size, MtimeNanos: job.MtimeNanos}
	if s.store != nil {
		if raw, ok := s.store.Get(entry); ok {
			return raw
		}
	}

	boxW := job.Cols * s.cellW
	boxH := job.Rows * s.cellH
	raw, err := thumbs.Generate(job.Path, boxW, boxH, s.limits)
	if err != nil {
		return nil
	}
	if s.store != nil {
		_ = s.store.Put(entry, raw)
	}

	return raw
}

// defaultThumbCache resolves the user cache location for thumbnails.
func defaultThumbCache() *thumbs.Store {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	store, err := thumbs.Open(base+"/gridfm/thumbs", 256<<20)
	if err != nil {
		return nil
	}

	return store
}

// DefaultThumbCache opens the shared thumbnail cache directory, nil when
// unavailable (thumbnails then simply never cache to disk).
func DefaultThumbCache() *thumbs.Store { return defaultThumbCache() }
