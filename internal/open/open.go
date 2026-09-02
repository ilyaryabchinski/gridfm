// Package open classifies files for opening and builds the corresponding
// commands: text files go to the user's editor, everything else to the
// desktop opener. Paths are always passed as arguments, never interpolated
// into shell strings.
package open

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Target is the handler chosen for a file.
type Target int

const (
	// TargetEditor opens the file in $VISUAL or $EDITOR.
	TargetEditor Target = iota
	// TargetDesktop hands the file to xdg-open.
	TargetDesktop
)

// editorExts are text-like extensions opened in the editor.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var editorExts = map[string]bool{
	".bash": true, ".c": true, ".cc": true, ".conf": true, ".cpp": true,
	".css": true, ".csv": true, ".env": true, ".go": true, ".h": true,
	".hpp": true, ".hs": true, ".html": true, ".ini": true, ".java": true,
	".js": true, ".json": true, ".log": true, ".lua": true, ".md": true,
	".php": true, ".py": true, ".rb": true, ".rs": true, ".scss": true,
	".sh": true, ".sql": true, ".svg": true, ".toml": true, ".ts": true,
	".txt": true, ".xml": true, ".yaml": true, ".yml": true, ".zsh": true,
}

// editorNames are extension-less text files opened in the editor.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var editorNames = map[string]bool{
	"dockerfile": true, "gitignore": true, "license": true, "makefile": true,
	"readme": true,
}

// TargetFor classifies a path. Anything text-like goes to the editor; the
// rest goes to the desktop opener.
func TargetFor(path string) Target {
	base := strings.ToLower(filepath.Base(path))
	// Dotfiles such as .gitignore match their extension-less name.
	if editorNames[strings.TrimPrefix(base, ".")] {
		return TargetEditor
	}
	if editorExts[strings.ToLower(filepath.Ext(path))] {
		return TargetEditor
	}

	return TargetDesktop
}

// EditorCommand returns the editor command from $VISUAL, then $EDITOR. The
// bool is false when neither variable is set to a non-empty value.
func EditorCommand() (string, []string, bool) {
	for _, key := range []string{"VISUAL", "EDITOR"} {
		value := strings.TrimSpace(execEnv(key))
		if value != "" {
			fields := strings.Fields(value)

			return fields[0], fields[1:], true
		}
	}

	return "", nil, false
}

// DesktopOpen hands a path to xdg-open without blocking. The process is
// reaped in the background; the returned error covers only the failure to
// start it.
func DesktopOpen(path string) error {
	//nolint:gosec // xdg-open is a fixed program; the path is passed as an argument, never a shell string
	cmd := exec.CommandContext(context.Background(), "xdg-open", path)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("start xdg-open: %w", err)
	}

	go func() { _ = cmd.Wait() }()

	return nil
}
