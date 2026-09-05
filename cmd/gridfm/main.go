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
	"gridfm/internal/config"
	"gridfm/internal/graphics"
	"gridfm/internal/ui"
)

// loadConfig reads the user's configuration file, falling back to
// defaults when it cannot be located. A malformed file is fatal: silent
// config errors hide typos forever.
func loadConfig() (config.Resolved, error) {
	path, err := config.Path()
	if err != nil {
		// No resolvable home: run on defaults rather than refuse to start.
		return config.Config{}.Resolve(), nil
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.Resolved{}, err
	}

	return cfg.Resolve(), nil
}

func sortModeValid(s string) bool {
	switch s {
	case "name", "size", "modified", "type":
		return true
	}

	return false
}

func main() {
	// Layered configuration: built-in defaults, then the user's TOML
	// file, then command-line flags. Each layer only fills what it
	// explicitly sets.
	cfg, err := loadConfig()
	if err != nil {
		fatalf("%v", err)
	}

	icons := flag.String("icons", cfg.Icons,
		"file type representation: labels, unicode, or nerdfont "+
			"(nerdfont needs a patched font; use labels if glyphs render wrong)")
	images := flag.String("images", cfg.Images,
		"terminal image thumbnails: auto, on, or off "+
			"(auto enables them on kitty and ghostty; on forces them, e.g. for a "+
			"foot build verified to speak kitty graphics; off disables them)")
	sidebar := flag.Bool("sidebar", cfg.Sidebar, "show the sidebar at start")
	inspector := flag.Bool("inspector", cfg.Inspector, "open the inspector panel at start")
	hidden := flag.Bool("hidden", cfg.ShowHidden, "show dot-prefixed entries")
	sortBy := flag.String("sort", cfg.Sort, "initial sort: name, size, modified, or type")
	order := flag.String("order", cfg.Order, "initial sort direction: asc or desc")

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

	if *sortBy != "" && !sortModeValid(*sortBy) {
		fatalf("invalid sort %q (want name, size, modified, or type)", *sortBy)
	}
	if *order != "" && *order != "asc" && *order != "desc" {
		fatalf("invalid order %q (want asc or desc)", *order)
	}

	path, err := filepath.Abs(start)
	if err != nil {
		fatalf("resolve start location: %v", err)
	}

	theme, err := ui.ThemeFromMap(cfg.Theme)
	if err != nil {
		fatalf("%v", err)
	}

	model := app.New(path, app.Options{
		Icons:      mode,
		Images:     imageMode,
		Sidebar:    sidebar,
		Inspector:  inspector,
		ShowHidden: *hidden,
		Sort:       *sortBy,
		Order:      *order,
		Keys:       cfg.Keys,
		Theme:      &theme,
	})
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Thumbnails ride on a side goroutine: placements go to the terminal
	// outside the render loop, generation results come back as messages.
	var syncer *app.ImageSync
	if _, ok := model.ImageProtocol(); ok {
		cellW, cellH := cellSize()
		syncer = app.NewImageSync(os.Stdout, app.DefaultThumbCache(), cellW, cellH,
			func(msg app.ThumbReadyMsg) { program.Send(msg) })
		model.SetImageSink(syncer)
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
