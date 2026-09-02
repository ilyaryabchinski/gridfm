// Command gridfm is a keyboard-first terminal file manager that presents
// files as a responsive grid of visual cards.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
)

func main() {
	start := "."
	if len(os.Args) > 1 {
		start = os.Args[1]
	}
	path, err := filepath.Abs(start)
	if err != nil {
		fatalf("resolve start location: %v", err)
	}

	program := tea.NewProgram(app.New(path), tea.WithAltScreen())

	_, err = program.Run()
	if err != nil {
		fatalf("run: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gridfm: "+format+"\n", args...)
	os.Exit(1)
}
