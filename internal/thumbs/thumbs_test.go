package thumbs_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/thumbs"
)

func writeTestImage(t *testing.T, dir, name string, w, h int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 0x80, 0xff})
		}
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestGenerateScalesPreservingAspect(t *testing.T) {
	t.Parallel()

	src := writeTestImage(t, t.TempDir(), "wide.png", 200, 100)

	raw, err := thumbs.Generate(src, 40, 40, thumbs.Default)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 40 || cfg.Height != 20 {
		t.Fatalf("thumbnail = %dx%d, want 40x20", cfg.Width, cfg.Height)
	}
}

func TestGenerateKeepsTinyImagesAtOwnSize(t *testing.T) {
	t.Parallel()

	src := writeTestImage(t, t.TempDir(), "tiny.png", 8, 4)

	raw, err := thumbs.Generate(src, 40, 40, thumbs.Default)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 8 || cfg.Height != 4 {
		t.Fatalf("thumbnail = %dx%d, want the original 8x4", cfg.Width, cfg.Height)
	}
}

func TestGenerateRefusesOversizedFiles(t *testing.T) {
	t.Parallel()

	src := writeTestImage(t, t.TempDir(), "big.png", 10, 10)
	limits := thumbs.Default
	limits.MaxSourceBytes = 10 // far below the real PNG's size

	if _, err := thumbs.Generate(src, 40, 40, limits); !errors.Is(err, thumbs.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestGenerateRefusesPixelBombs(t *testing.T) {
	t.Parallel()

	// A valid PNG header declaring 20000x20000 pixels but containing no
	// actual scanlines: the guard must refuse from the header alone,
	// before any full decode could allocate.
	path := filepath.Join(t.TempDir(), "bomb.png")
	if err := os.WriteFile(path, fakePNGHeader(20000, 20000), 0o644); err != nil {
		t.Fatal(err)
	}

	limits := thumbs.Default
	if _, err := thumbs.Generate(path, 40, 40, limits); !errors.Is(err, thumbs.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestGenerateRejectsNonImages(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "text.png")
	if err := os.WriteFile(path, []byte("definitely not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := thumbs.Generate(path, 40, 40, thumbs.Default); !errors.Is(err, thumbs.ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

// fakePNGHeader crafts a minimal PNG whose IHDR declares the given
// dimensions, followed by a truncated body. image.DecodeConfig reads only
// the header, which is exactly what the pixel guard must do before any
// allocation.
func fakePNGHeader(w, h uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8  // bit depth
	ihdr[9] = 2  // color type: truecolor
	ihdr[10] = 0 // compression
	ihdr[11] = 0 // filter
	ihdr[12] = 0 // interlace

	buf.Write([]byte{0, 0, 0, 13})
	buf.Write([]byte("IHDR"))
	buf.Write(ihdr)
	crc := crc32.ChecksumIEEE(append([]byte("IHDR"), ihdr...))
	buf.Write([]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)})

	// One empty IDAT chunk; DecodeConfig never reaches it.
	var body bytes.Buffer
	z := zlib.NewWriter(&body)
	z.Close()
	buf.Write([]byte{0, 0, 0, byte(body.Len())})
	buf.Write([]byte("IDAT"))
	buf.Write(body.Bytes())

	return buf.Bytes()
}
