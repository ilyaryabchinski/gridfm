package app_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
)

// Editor opening depends on process environment variables, so these tests
// mutate env and cannot run in parallel.

func TestEnterOnTextFileBuildsEditorCommand(t *testing.T) {
	t.Setenv("VISUAL", "true")
	t.Setenv("EDITOR", "vim")

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{{Name: "notes.md", Path: "/d/notes.md"}}, nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd == nil {
		t.Fatal("enter on a text file should produce the editor command")
	}
	// The command is never executed here; ExecProcess would suspend the
	// program and hand the terminal to the editor.
	if opened.IsLoading() {
		t.Error("opening a file should not start a directory request")
	}
}

func TestEnterWithoutEditorShowsNote(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{{Name: "notes.md", Path: "/d/notes.md"}}, nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if cmd != nil {
		t.Error("no editor configured should produce no command")
	}
	if view := opened.View(); !strings.Contains(view, "no editor configured") {
		t.Errorf("view should surface the missing-editor note, got %q", view)
	}
}

func TestOpenFinishedMessageReportsFailure(t *testing.T) {
	t.Setenv("VISUAL", "true")
	t.Setenv("EDITOR", "vim")

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{{Name: "notes.md", Path: "/d/notes.md"}}, nil)

	m = feed(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = feed(t, m, app.OpenFinishedMsg{Path: "/d/notes.md", Err: errTestLoad})
	if view := m.View(); !strings.Contains(view, "open failed: permission denied") {
		t.Errorf("view should surface editor failures, got %q", view)
	}
}
