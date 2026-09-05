package kitty

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Slot describes one thumbnail the screen should show: a stable identity
// (Key — content-derived, so the same image at a new cell reuses the
// upload), its cell origin and coverage.
type Slot struct {
	Key  string
	PNG  []byte
	Row  int // 1-based
	Col  int // 1-based
	Rows int
	Cols int
}

type placed struct {
	key         string
	imageID     uint32
	placementID uint32
	rows, cols  int
}

// Table tracks which images are uploaded and which placements are on
// screen, and produces the minimal byte sequence reconciling the screen
// with a desired slot set: untouched slots are left alone, stale
// placements are deleted, freed images are evicted so the terminal's
// graphics memory stays bounded.
type Table struct {
	nextID   uint32
	active   map[string]placed // by position "row:col"
	uploaded map[string]uint32 // by slot key
}

// NewTable returns an empty table matching a terminal with no images.
func NewTable() *Table {
	return &Table{
		active:   map[string]placed{},
		uploaded: map[string]uint32{},
	}
}

// Sync returns the bytes that make the screen show exactly desired.
func (t *Table) Sync(desired []Slot) ([]byte, error) {
	var out strings.Builder

	wanted := make(map[string]Slot, len(desired))
	for _, s := range desired {
		pos := position(s.Row, s.Col)
		if prev, dup := wanted[pos]; dup && prev.Key != s.Key {
			return nil, fmt.Errorf("two thumbnails want %s: %q and %q", pos, prev.Key, s.Key)
		}
		wanted[pos] = s
	}

	// Delete placements that vanished or changed identity or geometry,
	// in stable order so output is deterministic.
	for _, pos := range t.sortedPositions() {
		p := t.active[pos]
		want, keep := wanted[pos]
		if keep && want.Key == p.key && want.Rows == p.rows && want.Cols == p.cols {
			continue
		}
		out.Write(EncodeDeletePlacement(p.imageID, p.placementID))
		delete(t.active, pos)

		if !t.inUse(p.key) && !wantedAnywhere(wanted, p.key) {
			out.Write(EncodeDeleteImage(p.imageID))
			delete(t.uploaded, p.key)
		}
	}

	// Place new or moved slots, in stable order.
	for _, pos := range sortedWanted(wanted) {
		s := wanted[pos]
		if _, on := t.active[pos]; on {
			continue // kept above
		}

		id, seen := t.uploaded[s.Key]
		if !seen {
			t.nextID++
			id = t.nextID
			payload, err := EncodeTransmit(id, s.PNG)
			if err != nil {
				return nil, fmt.Errorf("thumbnail %q: %w", s.Key, err)
			}
			out.Write(payload)
			t.uploaded[s.Key] = id
		}

		t.nextID++
		placeID := t.nextID
		out.Write(EncodeAt(id, placeID, s.Row, s.Col, s.Cols, s.Rows))
		t.active[pos] = placed{key: s.Key, imageID: id, placementID: placeID, rows: s.Rows, cols: s.Cols}
	}

	return []byte(out.String()), nil
}

// Clear returns the bytes deleting every image and placement and resets
// the table to match the now-blank screen.
func (t *Table) Clear() []byte {
	t.active = map[string]placed{}
	t.uploaded = map[string]uint32{}

	return EncodeDeleteAll()
}

// Active reports how many placements the table believes are on screen.
func (t *Table) Active() int { return len(t.active) }

func position(row, col int) string {
	return strconv.Itoa(row) + ":" + strconv.Itoa(col)
}

func (t *Table) sortedPositions() []string {
	positions := make([]string, 0, len(t.active))
	for pos := range t.active {
		positions = append(positions, pos)
	}
	sort.Strings(positions)

	return positions
}

func sortedWanted(wanted map[string]Slot) []string {
	positions := make([]string, 0, len(wanted))
	for pos := range wanted {
		positions = append(positions, pos)
	}
	sort.Strings(positions)

	return positions
}

func (t *Table) inUse(key string) bool {
	for _, p := range t.active {
		if p.key == key {
			return true
		}
	}

	return false
}

func wantedAnywhere(wanted map[string]Slot, key string) bool {
	for _, s := range wanted {
		if s.Key == key {
			return true
		}
	}

	return false
}
