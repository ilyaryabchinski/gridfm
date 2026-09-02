package open_test

import (
	"testing"

	"gridfm/internal/open"
)

func TestTargetForTextLikeFiles(t *testing.T) {
	t.Parallel()

	editorCases := []string{
		"/d/main.go", "/d/notes.md", "/d/README", "/d/Makefile",
		"/d/Dockerfile", "/d/data.json", "/d/script.sh", "/d/.gitignore",
	}
	for _, path := range editorCases {
		if got := open.TargetFor(path); got != open.TargetEditor {
			t.Errorf("TargetFor(%q) = %v, want editor", path, got)
		}
	}

	desktopCases := []string{"/d/logo.png", "/d/song.mp3", "/d/archive.zip", "/d/blob"}
	for _, path := range desktopCases {
		if got := open.TargetFor(path); got != open.TargetDesktop {
			t.Errorf("TargetFor(%q) = %v, want desktop", path, got)
		}
	}
}

func TestEditorCommandPrefersVisual(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "vim")

	program, args, ok := open.EditorCommand()
	if !ok || program != "code" || len(args) != 1 || args[0] != "--wait" {
		t.Errorf("EditorCommand() = %q, %v, %v; want code [--wait], true", program, args, ok)
	}
}

func TestEditorCommandFallsBackToEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nvim")

	program, _, ok := open.EditorCommand()
	if !ok || program != "nvim" {
		t.Errorf("EditorCommand() = %q, %v; want nvim, true", program, ok)
	}
}

func TestEditorCommandWithoutConfiguration(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "   ")

	_, _, ok := open.EditorCommand()
	if ok {
		t.Error("EditorCommand should report missing configuration")
	}
}
