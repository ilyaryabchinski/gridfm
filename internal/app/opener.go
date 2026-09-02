package app

import (
	"context"
	"os/exec"
	"path/filepath"

	"gridfm/internal/open"

	tea "github.com/charmbracelet/bubbletea"
)

// This file owns the external opener lifecycle: classifying files, building
// editor and desktop commands, and reporting their completion. Process I/O
// happens only inside commands; the update loop never starts a process.

// openFile builds the command for opening a single file. Text targets run
// in the editor with terminal hand-off; everything else goes to the desktop
// opener, whose completion arrives as OpenFinishedMsg.
func (m *Model) openFile(path string) tea.Cmd {
	switch open.TargetFor(path) {
	case open.TargetDesktop:
		return desktopOpenCmd(path)
	case open.TargetEditor:
		program, args, ok := open.EditorCommand()
		if !ok {
			m.note = "no editor configured (set $VISUAL or $EDITOR)"

			return nil
		}

		// ExecProcess suspends the program, runs the editor on the real
		// terminal, and restores the TUI afterwards.
		//nolint:gosec // program and arguments come from trusted env config; no shell involved
		editor := exec.CommandContext(context.Background(), program, append(args, path)...)
		m.note = ""

		return tea.ExecProcess(editor, func(err error) tea.Msg {
			return OpenFinishedMsg{Path: path, Err: err}
		})
	}

	return nil
}

// openedLabel renders the status note for a completed open.
func openedLabel(path string) string {
	return "opened " + filepath.Base(path)
}
