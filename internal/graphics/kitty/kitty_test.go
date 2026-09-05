package kitty_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"gridfm/internal/graphics/kitty"
)

func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{0xff, 0, 0, 0xff})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// noisyPNG returns a PNG whose pixels are deterministic pseudo-random, so
// it compresses poorly and its base64 exceeds one protocol chunk.
func noisyPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	lcg := uint32(12345)
	for y := range h {
		for x := range w {
			lcg = lcg*1664525 + 1013904223
			img.Set(x, y, color.RGBA{uint8(lcg >> 24), uint8(lcg >> 16), uint8(lcg >> 8), 0xff})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func TestEncodeTransmitSingleChunk(t *testing.T) {
	t.Parallel()

	data := testPNG(4, 3)
	out, err := kitty.EncodeTransmit(7, data)
	if err != nil {
		t.Fatal(err)
	}

	s := string(out)
	if !strings.HasPrefix(s, "\x1b_Gf=100,s=4,v=3,i=7,q=2,m=0;") {
		t.Fatalf("transmit header wrong: %q", s[:40])
	}
	if !strings.HasSuffix(s, "\x1b\\") {
		t.Fatal("transmit must end with the APC terminator")
	}

	b64 := strings.TrimPrefix(strings.TrimSuffix(s, "\x1b\\"), "\x1b_Gf=100,s=4,v=3,i=7,q=2,m=0;")
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("payload is not clean base64: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("decoded payload must equal the source PNG")
	}
}

func TestEncodeTransmitChunksLongPayload(t *testing.T) {
	t.Parallel()

	// Noise compresses badly: the base64 payload spans multiple chunks.
	data := noisyPNG(300, 300)
	out, err := kitty.EncodeTransmit(2, data)
	if err != nil {
		t.Fatal(err)
	}

	s := string(out)
	commands := strings.Split(strings.TrimSuffix(s, "\x1b\\"), "\x1b\\\x1b_G")
	if len(commands) < 2 {
		t.Fatalf("expected multiple chunks, got %d commands", len(commands))
	}
	if !strings.Contains(s, "m=1;") {
		t.Error("continuation chunks must be marked m=1")
	}

	// Reassemble the base64 across chunks and check round-trip.
	var joined strings.Builder
	for _, cmd := range commands {
		if _, payload, ok := strings.Cut(cmd, ";"); ok {
			joined.WriteString(strings.TrimSuffix(payload, "\x1b\\"))
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(joined.String())
	if err != nil {
		t.Fatalf("reassembled payload invalid: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("reassembled payload must equal the source PNG")
	}
}

func TestEncodeTransmitRejectsGarbage(t *testing.T) {
	t.Parallel()

	if _, err := kitty.EncodeTransmit(1, []byte("not a png")); err == nil {
		t.Fatal("garbage must be rejected")
	}
}

func TestEncodeAtPositionsCursor(t *testing.T) {
	t.Parallel()

	out := string(kitty.EncodeAt(5, 6, 3, 4, 8, 5))
	if !strings.HasPrefix(out, "\x1b[3;4H") {
		t.Fatalf("must position to row 3 col 4 first: %q", out)
	}
	if !strings.Contains(out, "a=p,C=1,i=5,p=6,c=8,r=5,q=2;") {
		t.Fatalf("placement command wrong: %q", out)
	}
}

func TestDeleteCommands(t *testing.T) {
	t.Parallel()

	if got := string(kitty.EncodeDeletePlacement(5, 6)); got != "\x1b_Ga=d,d=i,i=5,p=6,q=2;\x1b\\" {
		t.Errorf("delete placement = %q", got)
	}
	if got := string(kitty.EncodeDeleteImage(5)); got != "\x1b_Ga=d,d=I,i=5,q=2;\x1b\\" {
		t.Errorf("delete image = %q", got)
	}
	if got := string(kitty.EncodeDeleteAll()); got != "\x1b_Ga=d,d=A,q=2;\x1b\\" {
		t.Errorf("delete all = %q", got)
	}
}

func slot(key string, col int, png []byte) kitty.Slot {
	return kitty.Slot{Key: key, PNG: png, Row: 2, Col: col, Rows: 3, Cols: 8}
}

func TestTableSyncPlacesAndReuses(t *testing.T) {
	t.Parallel()

	tbl := kitty.NewTable()
	pic := testPNG(20, 10)

	out, err := tbl.Sync([]kitty.Slot{slot("a", 2, pic)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "a=p,C=1,i=1,p=2,c=8,r=3") {
		t.Fatalf("first sync must transmit and place: %q", out)
	}
	if tbl.Active() != 1 {
		t.Fatalf("active = %d, want 1", tbl.Active())
	}

	// Identical state produces no bytes at all.
	out, err = tbl.Sync([]kitty.Slot{slot("a", 2, pic)})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("unchanged state must emit nothing, got %q", out)
	}

	// The same content in a new cell reuses the upload (no f=100) and
	// deletes the old placement only.
	out, err = tbl.Sync([]kitty.Slot{slot("a", 12, pic)})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "f=100") {
		t.Error("moving a slot must not retransmit the image")
	}
	if !strings.Contains(s, "a=d,d=i,i=1,p=2") {
		t.Error("the old placement must be deleted")
	}
	if !strings.Contains(s, "a=p,C=1,i=1,p=3") {
		t.Error("the new placement must appear")
	}
}

func TestTableSyncMovesAndClearsUp(t *testing.T) {
	t.Parallel()

	tbl := kitty.NewTable()
	a, b := testPNG(20, 10), testPNG(10, 10)

	if _, err := tbl.Sync([]kitty.Slot{slot("a", 2, a), slot("b", 12, b)}); err != nil {
		t.Fatal(err)
	}

	// b scrolls off; a stays. b's placement must be deleted and its
	// image data freed since nothing else uses it. The exact ids depend
	// on allocation order, so only the delete kinds are asserted.
	out, err := tbl.Sync([]kitty.Slot{slot("a", 2, a)})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "a=d,d=i,i=") {
		t.Error("the scrolled-off placement must be deleted")
	}
	if !strings.Contains(s, "a=d,d=I,i=") {
		t.Error("the scrolled-off image must be freed")
	}

	// Clear wipes everything and resets bookkeeping.
	out = tbl.Clear()
	if !bytes.Equal(out, kitty.EncodeDeleteAll()) {
		t.Fatalf("clear = %q", out)
	}
	if tbl.Active() != 0 {
		t.Fatalf("clear left %d active placements", tbl.Active())
	}

	// After clear, the same content uploads fresh (ids restart at 1).
	out, err = tbl.Sync([]kitty.Slot{slot("a", 2, a)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "f=100") {
		t.Error("after clear the image must be retransmitted")
	}
}

func TestTableSyncReplacesChangedGeometry(t *testing.T) {
	t.Parallel()

	tbl := kitty.NewTable()
	pic := testPNG(20, 10)

	if _, err := tbl.Sync([]kitty.Slot{slot("a", 2, pic)}); err != nil {
		t.Fatal(err)
	}

	// Zoom changes the coverage but not the content: no retransmit, one
	// replacement placement.
	moved := slot("a", 2, pic)
	moved.Cols, moved.Rows = 16, 6
	out, err := tbl.Sync([]kitty.Slot{moved})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "f=100") {
		t.Error("a geometry change must not retransmit")
	}
	if !strings.Contains(s, "c=16,r=6") {
		t.Error("the new coverage must be requested")
	}
}

func TestTableSyncRejectsOverlappingSlots(t *testing.T) {
	t.Parallel()

	tbl := kitty.NewTable()
	if _, err := tbl.Sync([]kitty.Slot{
		slot("a", 2, testPNG(10, 10)),
		slot("b", 2, testPNG(10, 10)),
	}); err == nil {
		t.Fatal("two slots claiming one cell must be rejected")
	}
}
