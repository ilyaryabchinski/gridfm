package app

import (
	"gridfm/internal/browser"
	"gridfm/internal/graphics/kitty"
	"gridfm/internal/thumbs"
	"gridfm/internal/ui"
)

// thumbReadyCap bounds the in-memory thumbnail map: roughly a screenful
// of high-resolution thumbnails plus headroom. Oldest keys evict first.
const thumbReadyCap = 512

// syncImages reconciles the desired thumbnail set with the sync loop and
// requests generation for visible images that have none yet. It performs
// no filesystem work: channel sends only, so View can call it every
// frame.
func (m *Model) syncImages(l ui.Layout) {
	if m.imgSink == nil {
		return
	}

	slots := m.imageSlots(l)
	m.imgSink <- slots

	if m.thumbLoad == nil {
		return
	}
	for _, e := range m.visibleImages(l) {
		key := thumbs.KeyOf(thumbs.Entry{Path: e.Path, Size: e.Size, MtimeNanos: e.ModTime.UnixNano()})
		if _, known := m.thumbReady[key]; known {
			continue
		}
		m.thumbLoad(ThumbJob{
			Path:       e.Path,
			Key:        key,
			Size:       e.Size,
			MtimeNanos: e.ModTime.UnixNano(),
			Cols:       l.Card.Width - 2,
			Rows:       imageRows(l),
		})
	}
}

// imageSlots computes the placements for visible ready thumbnails, in
// absolute terminal cells matching the card geometry rendered by
// renderViewport. Any state that draws text over the grid — blocking
// overlays, the floating sidebar or inspector — yields no slots: images
// render above text, so text must never fight them.
func (m *Model) imageSlots(l ui.Layout) []kitty.Slot {
	floatingSidebar := !l.SidebarVisible && m.sidebarOn
	floatingInspector := l.InspectorWidth == 0 && m.inspectorOn
	if !l.Usable || m.overlaysOpen() || floatingSidebar || floatingInspector {
		return nil
	}
	if l.Zoom != ui.ZoomNormal || m.loading || m.loadErr != nil || len(m.browser.Entries) == 0 {
		// Thumbnails replace the two inner rows above the name, a shape
		// only the normal card has; other zooms and empty states keep
		// icons or text.
		return nil
	}

	docked := 0
	if l.SidebarVisible {
		docked = l.SidebarWidth
	}

	var slots []kitty.Slot
	for r := range l.RowsVisible {
		for c := range l.Columns {
			e, ok := m.entryAtGrid(l, r, c)
			if !ok {
				continue
			}
			key := thumbs.KeyOf(thumbs.Entry{Path: e.Path, Size: e.Size, MtimeNanos: e.ModTime.UnixNano()})
			png, ready := m.thumbReady[key]
			if !ready || len(png) == 0 {
				continue
			}

			slots = append(slots, kitty.Slot{
				Key:  key,
				PNG:  png,
				Row:  2 + r*(l.Card.Height+ui.CardGapY) + 1,
				Col:  docked + 1 + c*(l.Card.Width+ui.CardGapX) + 1,
				Cols: l.Card.Width - 2,
				Rows: imageRows(l),
			})
		}
	}

	return slots
}

// imageRows is the thumbnail's coverage in cell rows: the two inner rows
// above the name line on a normal card.
func imageRows(l ui.Layout) int {
	return 2
}

// entryAtGrid returns the visible entry rendered at viewport grid
// position (r, c), mirroring renderViewport's index math.
func (m *Model) entryAtGrid(l ui.Layout, r, c int) (browser.Entry, bool) {
	entries := m.browser.Visible()
	first := m.scrollRow * l.Columns
	i := first + r*l.Columns + c
	if i >= len(entries) {
		return browser.Entry{}, false
	}

	return entries[i], true
}

// visibleImages lists the visible viewport entries that could ever have
// a thumbnail: image-suffixed regular files.
func (m *Model) visibleImages(l ui.Layout) []browser.Entry {
	if m.overlaysOpen() || l.Zoom != ui.ZoomNormal {
		return nil
	}

	var out []browser.Entry
	for r := range l.RowsVisible {
		for c := range l.Columns {
			e, ok := m.entryAtGrid(l, r, c)
			if !ok || ui.Classify(e) != ui.CategoryImage {
				continue
			}
			out = append(out, e)
		}
	}

	return out
}

// overlaysOpen reports whether any blocking overlay owns the screen.
func (m *Model) overlaysOpen() bool {
	return m.input != inputNone || m.question != nil || m.confirm != confirmNone ||
		m.showResults || m.helpOpen || m.sortOpen
}

// hasThumb reports whether a successfully generated thumbnail exists for
// the entry, so the card reserves its label rows for the image.
func (m *Model) hasThumb(e browser.Entry) bool {
	if m.imgSink == nil {
		return false
	}
	key := thumbs.KeyOf(thumbs.Entry{Path: e.Path, Size: e.Size, MtimeNanos: e.ModTime.UnixNano()})
	png, ok := m.thumbReady[key]

	return ok && len(png) > 0
}

// applyThumbReady stores a finished thumbnail. Failures are remembered
// with a nil entry so a corrupt file cannot trigger endless regeneration.
func (m *Model) applyThumbReady(msg ThumbReadyMsg) {
	if _, known := m.thumbReady[msg.Key]; known {
		return
	}
	if len(m.thumbKeys) >= thumbReadyCap {
		oldest := m.thumbKeys[0]
		m.thumbKeys = m.thumbKeys[1:]
		delete(m.thumbReady, oldest)
	}
	m.thumbKeys = append(m.thumbKeys, msg.Key)
	m.thumbReady[msg.Key] = msg.PNG
}
