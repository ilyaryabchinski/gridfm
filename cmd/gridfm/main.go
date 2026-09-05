// Command gridfm is a keyboard-first terminal file manager that presents
// files as a responsive grid of visual cards.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/graphics"
	"gridfm/internal/ui"
)

func main() {
	icons := flag.String("icons", ui.IconModeUnicode.String(),
		"file type representation: labels, unicode, or nerdfont "+
			"(nerdfont needs a patched font; use labels if glyphs render wrong)")
	images := flag.String("images", "auto",
		"terminal image thumbnails: auto, on, or off "+
			"(auto enables them on kitty, foot, and ghostty; on forces them even in multiplexers)")

	flag.Parse()

	start := "."
	if flag.NArg() > 0 {
		start = flag.Arg(0)
	}

	mode, err := ui.ParseIconMode(*icons)
	if err != nil {
		fatalf("%v (want labels, unicode, or nerdfont)", err)
	}

	imageMode, err := graphics.ParseMode(*images)
	if err != nil {
		fatalf("%v", err)
	}

	path, err := filepath.Abs(start)
	if err != nil {
		fatalf("resolve start location: %v", err)
	}

	model := app.New(path, app.Options{Icons: mode, Images: imageMode})
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Thumbnails ride on a side goroutine: placements go to the terminal
	// outside the render loop, generation results come back as messages.
	var syncer *app.ImageSync
	if _, ok := model.ImageProtocol(); ok {
		cellW, cellH := cellSize()
		syncer = app.NewImageSync(os.Stdout, app.DefaultThumbCache(), cellW, cellH,
			func(msg app.ThumbReadyMsg) { program.Send(msg) })
		model.SetImageSink(syncer.SlotChan())
		model.SetThumbLoader(syncer.Load)
	}

	final, err := program.Run()
	if syncer != nil {
		syncer.Stop()
	}
	if closer, ok := final.(*app.Model); ok {
		_ = closer.Close() // release the filesystem watcher
	}
	if err != nil {
		fatalf("run: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gridfm: "+format+"\n", args...)
	os.Exit(1)
}
