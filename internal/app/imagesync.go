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

// imgUpdate is one instruction for the sync loop: either a desired
// placement set or a reset demand. Everything is serialized through one
// channel so the table is only ever touched by the loop.
type imgUpdate struct {
	slots []kitty.Slot
	reset bool
}

// ImageSync owns everything that must not race the render loop: the
// kitty placement table (the truth about what is on screen), the writes
// of protocol bytes, and bounded thumbnail generation. The model only
// ships immutable slot lists and job requests.
type ImageSync struct {
	mu       sync.Mutex
	ch       chan imgUpdate
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
		ch:       make(chan imgUpdate, 1),
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

// ship queues an update, never blocking: the latest one wins.
func (s *ImageSync) ship(u imgUpdate) {
	s.mu.Lock()
	quitting := s.quitting
	s.mu.Unlock()
	if quitting {
		return
	}

	select {
	case s.ch <- u:
	default:
		// The single buffer is full: replace its stale entry.
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- u:
		default:
		}
	}
}

// Slots ships a desired placement set.
func (s *ImageSync) Slots(slots []kitty.Slot) { s.ship(imgUpdate{slots: slots}) }

// Reset demands the terminal's graphics state be treated as garbage.
func (s *ImageSync) reset() { s.ship(imgUpdate{reset: true}) }

// Load requests thumbnail generation for one entry; repeated requests for// the same key while one is in flight are dropped. Results arrive via
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

// Reset tells the sync loop the terminal's graphics state is no longer
// trustworthy — an external viewer ran, or the terminal was resized — so
// it drops every table assumption and re-uploads from scratch on the
// next frame.
func (s *ImageSync) Reset() { s.ship(imgUpdate{reset: true}) }

// Stop clears every on-screen image and shuts the loops down. Safe to
// call once; called after the program loop has exited, so no more Ships
// arrive.
func (s *ImageSync) Stop() {
	s.mu.Lock()
	if s.quitting {
		s.mu.Unlock()

		return
	}
	s.quitting = true
	s.mu.Unlock()

	close(s.jobs) // workers drain and exit
	close(s.ch)   // loop drains pending updates and exits
	s.write(kitty.EncodeDeleteAll())
}

// loop serializes protocol output and keeps the table as screen truth.
func (s *ImageSync) loop() {
	for u := range s.ch {
		if u.reset {
			// The screen's images are garbage: delete everything and
			// forget every upload so the next sync re-transmits.
			s.write(kitty.EncodeDeleteAll())
			s.table = kitty.NewTable()

			continue
		}

		payload, err := s.table.Sync(u.slots)
		if err != nil || len(payload) == 0 {
			continue
		}
		s.write(payload)
	}
}

// write emits one wrapped batch: cursor saved and restored so placement
// never moves the real cursor away from where the TUI renderer expects
// it.
func (s *ImageSync) write(payload []byte) {
	var buf []byte
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
