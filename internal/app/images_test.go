package app_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/graphics"
	"gridfm/internal/graphics/kitty"
	"gridfm/internal/thumbs"
)

// TestImageResolutionPins pins that the start-up options resolve against
// the real environment: capable terminals get thumbnails, the off switch
// always wins.
func TestImageResolutionPins(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")

	m := app.New("/d", app.Options{})

	if proto, ok := m.ImageProtocol(); !ok || proto != "kitty" {
		t.Fatalf("auto on kitty = (%q, %v), want (kitty, true)", proto, ok)
	}

	off := app.New("/d", app.Options{Images: graphics.ModeOff})
	if _, ok := off.ImageProtocol(); ok {
		t.Fatal("mode off must disable thumbnails")
	}
}

// imageModel builds a grid-only model with thumbnails enabled and the
// image plumbing connected to test channels.
func imageModel(t *testing.T, root string, entries []browser.Entry) (*app.Model, <-chan []kitty.Slot, *[]app.ThumbJob) {
	t.Helper()

	m := app.New(root, app.Options{Images: graphics.ModeOn})
	m = gridOnly(t, resize(t, m, 120, 30))

	sink := make(chan []kitty.Slot, 4)
	m.SetImageSink(sink)

	var jobs []app.ThumbJob
	m.SetThumbLoader(func(job app.ThumbJob) { jobs = append(jobs, job) })

	m = loaded(t, m, 1, root, entries, nil)
	m.View()

	return m, sink, &jobs
}

func imageEntry(root, name string, size int64, when time.Time) browser.Entry {
	return browser.Entry{
		Name: name, Path: filepath.Join(root, name),
		Size: size, ModTime: when,
	}
}

func testPNGs(t *testing.T, entries []browser.Entry) map[string][]byte {
	t.Helper()

	out := map[string][]byte{}
	for _, e := range entries {
		out[thumbs.KeyOf(thumbs.Entry{Path: e.Path, Size: e.Size, MtimeNanos: e.ModTime.UnixNano()})] = tinyPNG(t)
	}

	return out
}

// drainSlots reads every pending snapshot, returning the latest.
func drainSlots(sink <-chan []kitty.Slot) []kitty.Slot {
	var last []kitty.Slot
	for {
		select {
		case s := <-sink:
			last = s
		default:
			return last
		}
	}
}

func TestImageSlotsMatchCardGeometry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	when := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	entries := []browser.Entry{
		{Name: "doc.txt", Path: filepath.Join(root, "doc.txt")},
		imageEntry(root, "pic.png", 999, when),
	}

	m, sink, _ := imageModel(t, root, entries)
	pngs := testPNGs(t, entries)

	// Deliver the generated thumbnail, then render: the next frame must
	// ship exactly one slot, for the image entry.
	m = feed(t, m, app.ThumbReadyMsg{
		Key: thumbs.KeyOf(thumbs.Entry{Path: entries[1].Path, Size: 999, MtimeNanos: when.UnixNano()}),
		PNG: pngs[thumbs.KeyOf(thumbs.Entry{Path: entries[1].Path, Size: 999, MtimeNanos: when.UnixNano()})],
	})
	m.View()

	slots := drainSlots(sink)
	if len(slots) != 1 {
		t.Fatalf("slots = %d, want 1", len(slots))
	}

	s := slots[0]
	// The image is the second entry: column index 1 of row 0 on a
	// 12-column grid (card 14 wide, 2 gap), inner origin at col 18,
	// row 3 below header and border.
	if s.Col != 18 || s.Row != 3 {
		t.Errorf("slot origin = (%d,%d), want (18,3)", s.Col, s.Row)
	}
	if s.Cols != 12 || s.Rows != 2 {
		t.Errorf("slot coverage = %dx%d, want 12x2", s.Cols, s.Rows)
	}
	if len(s.PNG) == 0 {
		t.Error("slot must carry the PNG payload")
	}
}

func TestImageSlotsClearOnOverlay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	when := time.Now()
	entries := []browser.Entry{imageEntry(root, "pic.png", 10, when)}
	m, sink, _ := imageModel(t, root, entries)
	pngs := testPNGs(t, entries)

	key := thumbs.KeyOf(thumbs.Entry{Path: entries[0].Path, Size: 10, MtimeNanos: when.UnixNano()})
	m = feed(t, m, app.ThumbReadyMsg{Key: key, PNG: pngs[key]})

	// Baseline: images flow.
	m.View()
	if slots := drainSlots(sink); len(slots) != 1 {
		t.Fatalf("baseline slots = %d, want 1", len(slots))
	}

	// Open the help overlay: the next frame must ship an empty set so the
	// sync loop deletes every image the legend would cover.
	m = press(t, m, "?")
	m.View()
	if slots := drainSlots(sink); len(slots) != 0 {
		t.Fatalf("overlay slots = %d, want none", len(slots))
	}
}

func TestThumbLoaderRequestedForVisibleImages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	when := time.Now()
	entries := []browser.Entry{
		{Name: "doc.txt", Path: filepath.Join(root, "doc.txt")},
		imageEntry(root, "pic.png", 10, when),
	}
	_, _, jobs := imageModel(t, root, entries)

	if len(*jobs) != 1 {
		t.Fatalf("jobs = %+v, want exactly the one image entry", *jobs)
	}
	if (*jobs)[0].Path != entries[1].Path {
		t.Errorf("job path = %q, want the image", (*jobs)[0].Path)
	}
	if (*jobs)[0].Cols != 12 || (*jobs)[0].Rows != 2 {
		t.Errorf("job box = %dx%d cells, want 12x2", (*jobs)[0].Cols, (*jobs)[0].Rows)
	}
}

func TestImagesDisabledKeepsRenderingClean(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("KITTY_PID", "")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "")
	t.Setenv("TMUX", "")
	t.Setenv("ZELLIJ", "")

	root := t.TempDir()
	m := app.New(root, app.Options{}) // auto: no terminal markers here
	if _, ok := m.ImageProtocol(); ok {
		t.Fatal("auto mode without terminal markers must stay disabled")
	}

	m = loaded(t, m, 1, root, entriesAt(root, 3), nil)
	out := m.View()
	if strings.Contains(out, "\x1b_G") {
		t.Error("rendered view must never contain graphics sequences")
	}
}

// lockedBuffer is a thread-safe bytes.Buffer for reading sync output
// while the goroutine writes it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestImageSyncWritesAndClears(t *testing.T) {
	t.Parallel()

	var out lockedBuffer
	syncer := app.NewImageSync(&out, nil, 10, 20, func(app.ThumbReadyMsg) {})

	png := tinyPNG(t)
	syncer.Slots([]kitty.Slot{{Key: "a", PNG: png, Row: 3, Col: 2, Cols: 12, Rows: 2}})
	waitFor(t, func() bool {
		return strings.Contains(out.String(), "a=p,")
	})

	// Clearing the screen removes everything.
	syncer.Slots(nil)
	waitFor(t, func() bool {
		return strings.Contains(out.String(), "a=d,d=p,")
	})

	syncer.Stop()
	if !strings.Contains(out.String(), "a=d,d=A") {
		t.Error("stop must delete all images")
	}
	if !strings.HasPrefix(out.String(), kitty.CursorSave) {
		t.Error("output batches must be cursor-wrapped")
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{0xff, 0, 0, 0xff})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}
