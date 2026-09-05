// Package kitty emits the Kitty graphics protocol: transmitting
// thumbnails, placing them at absolute cell positions, and deleting
// them again. Nothing here performs I/O; callers write the returned
// bytes to the terminal.
package kitty

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Escape framing for the graphics protocol.
const (
	// APC introduction and terminator wrap every graphics command.
	apcStart = "\x1b_G"
	apcEnd   = "\x1b\\"

	// cursorSave and cursorRestore bracket emission batches so image
	// placement never leaves the real cursor where bubbletea's renderer
	// does not expect it.
	CursorSave    = "\x1b7"
	CursorRestore = "\x1b8"

	// cupPrefix starts a Cursor Position row;col sequence.
	cupPrefix = "\x1b["
	cupSuffix = "H"

	// chunkSize is the base64 payload size per transmit chunk, the
	// protocol's recommended maximum.
	chunkSize = 4096
)

// pngDims reads the width and height from a PNG's IHDR chunk header.
func pngDims(png []byte) (w, h int, err error) {
	const ihdrDims = 24 // 8 signature + 4 length + 4 type + 8 dims
	if len(png) < ihdrDims {
		return 0, 0, fmt.Errorf("png too short for dimensions")
	}
	if string(png[12:16]) != "IHDR" {
		return 0, 0, fmt.Errorf("not a PNG IHDR chunk")
	}
	w = int(png[16])<<24 | int(png[17])<<16 | int(png[18])<<8 | int(png[19])
	h = int(png[20])<<24 | int(png[21])<<16 | int(png[22])<<8 | int(png[23])
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("png declares invalid dimensions %dx%d", w, h)
	}

	return w, h, nil
}

// EncodeTransmit returns the commands that upload png as image id. The
// payload is base64 and split into protocol-sized chunks; responses are
// suppressed so the render loop never has to read a reply.
func EncodeTransmit(id uint32, png []byte) ([]byte, error) {
	w, h, err := pngDims(png)
	if err != nil {
		return nil, err
	}

	var out strings.Builder
	encoded := base64.StdEncoding.EncodeToString(png)
	for off := 0; off < len(encoded) || off == 0; off += chunkSize {
		end := min(off+chunkSize, len(encoded))
		chunk := encoded[off:end]

		more := end < len(encoded)
		if off == 0 {
			fmt.Fprintf(&out, "%sf=100,s=%d,v=%d,i=%d,q=2,m=%d;", apcStart, w, h, id, b2i(more))
		} else {
			fmt.Fprintf(&out, "%si=%d,q=2,m=%d;", apcStart, id, b2i(more))
		}
		out.WriteString(chunk)
		out.WriteString(apcEnd)
		if !more {
			break
		}
	}

	return []byte(out.String()), nil
}

// EncodePlace returns the command that displays image id covering
// cols x rows cells. C=1 keeps the cursor motionless during placement —
// the default policy moves it right and down by the placement size, which
// would desync the TUI's renderer. The caller must position the cursor
// first; EncodeAt does both.
func EncodePlace(id, placementID uint32, cols, rows int) []byte {
	return []byte(fmt.Sprintf("%sa=p,C=1,i=%d,p=%d,c=%d,r=%d,q=2;%s", apcStart, id, placementID, cols, rows, apcEnd))
}

// EncodeAt returns the cursor positioning plus placement for one image:
// move to row, col (1-based), then cover cols x rows cells.
func EncodeAt(id, placementID uint32, row, col, cols, rows int) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "%s%d;%d%s", cupPrefix, row, col, cupSuffix)
	out.Write(EncodePlace(id, placementID, cols, rows))

	return []byte(out.String())
}

// EncodeDeletePlacement removes one placement identified by its image id
// and placement id. Per the delete table, that selector is lowercase d=i
// with both keys; d=p means "placements intersecting a cell" and would
// misfire. The lowercase form keeps the image data uploaded for reuse.
func EncodeDeletePlacement(id, placementID uint32) []byte {
	return []byte(fmt.Sprintf("%sa=d,d=i,i=%d,p=%d,q=2;%s", apcStart, id, placementID, apcEnd))
}

// EncodeDeleteImage frees the uploaded image data and all its placements.
// The uppercase variant is what releases the terminal-side memory; the
// lowercase d=i only hides the image while keeping the data. Data is kept
// if the image lives on in scrollback, which the alt screen never has.
func EncodeDeleteImage(id uint32) []byte {
	return []byte(fmt.Sprintf("%sa=d,d=I,i=%d,q=2;%s", apcStart, id, apcEnd))
}

// EncodeDeleteAll removes every image and placement on screen: the
// guaranteed-clean-slate command used on quit and before overlays that
// must not fight the grid for pixels.
func EncodeDeleteAll() []byte {
	return []byte(fmt.Sprintf("%sa=d,d=A,q=2;%s", apcStart, apcEnd))
}

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}
