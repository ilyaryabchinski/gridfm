package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/preview"
)

// pressI toggles the inspector with the i key.
func pressI(t *testing.T, m *app.Model) (*app.Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}

	return updated, cmd
}

func TestInspectorOpensAndLoads(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	victim := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(victim, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "notes.txt", Path: victim},
	}, nil)

	m, cmd := pressI(t, m)
	if !m.InspectorOn() {
		t.Fatal("i should toggle the inspector on")
	}
	if cmd == nil {
		t.Fatal("opening the inspector should request metadata")
	}
	if m.Inspector() != nil {
		t.Error("panel content should clear while loading")
	}

	// The load result applies and the panel renders it.
	msg := cmd().(app.InspectorLoadedMsg)
	if msg.RequestID != m.InspectorRequestID() {
		t.Fatalf("request id = %d, want %d", msg.RequestID, m.InspectorRequestID())
	}
	m = feed(t, m, msg)
	if m.Inspector() == nil || m.Inspector().Name != "notes.txt" {
		t.Fatalf("inspector = %+v, want notes.txt", m.Inspector())
	}
	if view := m.View(); !strings.Contains(view, "notes.txt") || !strings.Contains(view, "inspector") {
		t.Errorf("the panel should render the loaded entry, got:\n%s", view)
	}

	// i again closes the panel and invalidates requests.
	m, _ = pressI(t, m)
	if m.InspectorOn() || m.Inspector() != nil {
		t.Error("i should close the inspector and drop its content")
	}
	if view := m.View(); strings.Contains(view, " inspector") {
		t.Errorf("the closed panel should not render, got:\n%s", view)
	}
}

func TestInspectorRejectsStaleResults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	first := filepath.Join(root, "a.txt")
	second := filepath.Join(root, "b.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "a.txt", Path: first},
		{Name: "b.txt", Path: second},
	}, nil)

	m, cmd := pressI(t, m)
	staleMsg := cmd().(app.InspectorLoadedMsg)

	// Focus moves before the first result lands: a new request is issued.
	m = press(t, m, "l")
	if m.Inspector() != nil {
		t.Error("moving focus should clear the panel immediately")
	}
	if m.InspectorRequestID() == staleMsg.RequestID {
		t.Fatal("moving focus should issue a new inspector request")
	}
	fresh, freshCmd := m.InspectorRequestID(), func() tea.Msg { return nil }
	_ = freshCmd

	m = feed(t, m, staleMsg) // stale result must be discarded
	if m.Inspector() != nil {
		t.Error("a stale inspector result must not apply")
	}
	if m.InspectorRequestID() != fresh {
		t.Error("a stale result must not change the request identity")
	}
}

func TestInspectorFollowsFocus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := make([]string, 2)
	for i, name := range []string{"a.txt", "b.txt"} {
		paths[i] = filepath.Join(root, name)
		if err := os.WriteFile(paths[i], []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 120, 30))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "a.txt", Path: paths[0]},
		{Name: "b.txt", Path: paths[1]},
	}, nil)

	m, _ = pressI(t, m)
	m = feed(t, m, app.InspectorLoadedMsg{
		RequestID: m.InspectorRequestID(), Path: paths[0],
		Info: &preview.Info{Path: paths[0], Name: "a.txt"},
	})
	if m.Inspector().Name != "a.txt" {
		t.Fatalf("inspector = %+v, want a.txt", m.Inspector())
	}

	// Moving focus re-requests for the new entry and clears the panel.
	m, cmd := func() (*app.Model, tea.Cmd) {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
		updated, ok := next.(*app.Model)
		if !ok {
			t.Fatalf("Update returned %T", next)
		}

		return updated, cmd
	}()
	if cmd == nil {
		t.Fatal("focus movement with the inspector open should re-request")
	}
	msg := cmd().(app.InspectorLoadedMsg)
	if msg.Path != paths[1] {
		t.Errorf("re-request path = %q, want %q", msg.Path, paths[1])
	}

	m = feed(t, m, msg)
	if m.Inspector().Name != "b.txt" {
		t.Errorf("inspector = %+v, want b.txt", m.Inspector())
	}
}
