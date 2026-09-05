package app_test

import (
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/graphics"
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
