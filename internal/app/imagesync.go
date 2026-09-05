package app

import (
	"io"
	"os"
	"sync"

	"gridfm/internal/graphics/kitty"
	"gridfm/internal/thumbs"
)

// cellPixels is the assumed terminal cell size when the terminal does not
// report one, in pixels: a common 10x20 default close enough for
// thumbnail sharpness.
var cellPixels = [2]int{10, 20}

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

// ImageSync owns everything that must not race the render loop: the
// kitty placement table (the truth about what is on screen), the writes
// of protocol bytes, and bounded thumbnail generation. The model only
// ships immutable slot lists and job requests.
type ImageSync struct {
	mu       sync.Mutex
	ch       chan []kitty.Slot
	jobs     chan ThumbJob
	inFlight map[string]bool
	notify   func(ThumbReadyMsg)
	store    *thumbs.Store
	limits   thumbs.Limits
	quitting bool

	table *kitty.Table
	out   io.Writer
}

// NewImageSync starts the sync loop and two generation workers. notify is
// called from worker goroutines with each finished thumbnail — wire it to
// Program.Send. store may be nil for a memory-only setup (tests).
func NewImageSync(out io.Writer, store *thumbs.Store, cellW, cellH int, notify func(ThumbReadyMsg)) *ImageSync {
	if cellW > 0 && cellH > 0 {
		cellPixels = [2]int{cellW, cellH}
	}

	s := &ImageSync{
		ch:       make(chan []kitty.Slot, 1),
		jobs:     make(chan ThumbJob, 256),
		inFlight: map[string]bool{},
		notify:   notify,
		store:    store,
		limits:   thumbs.Default,
		table:    kitty.NewTable(),
		out:      out,
	}

	go s.loop()
	for range 2 {
		go s.worker()
	}

	return s
}

// Slots ships a desired placement set. Never blocks: the latest snapshot
// wins.
func (s *ImageSync) Slots(slots []kitty.Slot) {
	select {
	case s.ch <- slots:
	default:
		// The single buffer is full: replace its stale snapshot.
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- slots:
		default:
		}
	}
}

// SlotChan returns the channel the model ships desired placement sets to.
func (s *ImageSync) SlotChan() chan<- []kitty.Slot { return s.ch }

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

// Stop clears every on-screen image and shuts the loops down. Safe to
// call once.
func (s *ImageSync) Stop() {
	s.mu.Lock()
	if s.quitting {
		s.mu.Unlock()

		return
	}
	s.quitting = true
	s.mu.Unlock()

	close(s.jobs)
	s.Slots(nil)
	s.write(kitty.EncodeDeleteAll())
}

// loop serializes protocol output and keeps the table as screen truth.
func (s *ImageSync) loop() {
	for slots := range s.ch {
		payload, err := s.table.Sync(slots)
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

	boxW := job.Cols * cellPixels[0]
	boxH := job.Rows * cellPixels[1]
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
