// Package thumbs generates bounded image thumbnails for the file grid.
//
// Everything here is defensive by default: source files larger than the
// size cap are refused unread, images whose declared dimensions exceed the
// pixel budget are refused before a full decode, and outputs are scaled
// down to the requested cell box. A hostile image can cost a bounded
// amount of memory, never an unbounded one.
package thumbs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	// Blank imports register the image decoders the standard library
	// dispatches to by sniffs of the file header.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"

	// The webp decoder is decode-only, like the stdlib registrations
	// above; thumbnails never re-encode webp.
	_ "golang.org/x/image/webp"
)

// Limits bound the resources one thumbnail generation may consume. The
// zero value is invalid; use Default.
type Limits struct {
	// MaxSourceBytes refuses to read source files above this size.
	MaxSourceBytes int64
	// MaxPixels refuses to decode images whose declared dimensions
	// exceed this many pixels, before allocation happens.
	MaxPixels int64
}

// Default is the production resource budget: 32 MB files, 16 decoded
// megapixels (about 64 MB of RGBA), which comfortably covers photographs
// while capping abuse.
var Default = Limits{
	MaxSourceBytes: 32 << 20,
	MaxPixels:      16 << 20,
}

// Errors surfaced by Generate; wrap with %w to keep them checkable.
var (
	ErrTooLarge = errors.New("image exceeds the thumbnail size limit")
	ErrFormat   = errors.New("unsupported image format")
)

// Generate renders a PNG thumbnail of the image at path, scaled to fit
// boxW x boxH pixels while preserving the aspect ratio. The returned PNG
// is exactly the scaled image, ready to transmit as a terminal graphic.
func Generate(path string, boxW, boxH int, limits Limits) ([]byte, error) {
	if boxW <= 0 || boxH <= 0 {
		return nil, fmt.Errorf("thumbnail box must be positive, got %dx%d", boxW, boxH)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("thumbnail source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("thumbnail source %s is not a regular file", path)
	}
	if info.Size() > limits.MaxSourceBytes {
		return nil, fmt.Errorf("%w: %s is %d bytes", ErrTooLarge, filepath.Base(path), info.Size())
	}

	raw, err := os.ReadFile(path) //nolint:gosec // the path is the user's own image
	if err != nil {
		return nil, fmt.Errorf("thumbnail source: %w", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrFormat, filepath.Base(path), err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > limits.MaxPixels {
		return nil, fmt.Errorf("%w: %s declares %dx%d", ErrTooLarge, filepath.Base(path), cfg.Width, cfg.Height)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("thumbnail decode: %s: %w", filepath.Base(path), err)
	}

	dstW, dstH := fitInto(cfg.Width, cfg.Height, boxW, boxH)
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, fmt.Errorf("thumbnail encode: %s: %w", filepath.Base(path), err)
	}

	return out.Bytes(), nil
}

// fitInto scales w x h down to the largest size preserving aspect ratio
// that fits the box. Tiny images are kept at their own size: upscaling
// adds blur without information.
func fitInto(w, h, boxW, boxH int) (int, int) {
	if w <= boxW && h <= boxH {
		return w, h
	}

	scaleW := float64(boxW) / float64(w)
	scaleH := float64(boxH) / float64(h)
	if scaleW < scaleH {
		return max(int(float64(w)*scaleW), 1), max(int(float64(h)*scaleW), 1)
	}

	return max(int(float64(w)*scaleH), 1), max(int(float64(h)*scaleH), 1)
}

// Key derives the stable cache key for one source file version: the same
// path, size, and mtime always maps to the same key, any change busts it.
func Key(path string, size int64, mtimeNanos int64) string {
	h := sha256.New()
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(size))       //nolint:gosec // sizes and times are non-negative
	binary.LittleEndian.PutUint64(buf[8:], uint64(mtimeNanos)) //nolint:gosec // sizes and times are non-negative
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(path))

	return string(h.Sum(nil))
}

// Entry identifies one cacheable source file version.
type Entry struct {
	Path       string
	Size       int64
	MtimeNanos int64
}

// KeyOf derives the entry's cache key.
func KeyOf(e Entry) string {
	return Key(e.Path, e.Size, e.MtimeNanos)
}
