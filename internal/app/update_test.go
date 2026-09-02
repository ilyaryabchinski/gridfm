package app_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
)

// errTestLoad is a package-level sentinel so tests can assert error
// identity through errors.Is.
var errTestLoad = errors.New("permission denied")

// errTestEscape embeds a control character to prove rendering sanitizes it.
var errTestEscape = errors.New("esc \x1b[31mred")

// feed applies a message to the model and returns the updated model.
func feed(t *testing.T, m tea.Model, msg tea.Msg) *app.Model {
	t.Helper()
	next, _ := m.Update(msg)
	updated, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}

	return updated
}

func press(t *testing.T, m *app.Model, s string) *app.Model {
	t.Helper()

	return feed(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func pressBackspace(t *testing.T, m *app.Model) *app.Model {
	t.Helper()

	return feed(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
}

func resize(t *testing.T, m *app.Model, w, h int) *app.Model {
	t.Helper()

	return feed(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}

func loaded(t *testing.T, m *app.Model, requestID uint64, path string, entries []browser.Entry, err error) *app.Model {
	t.Helper()

	return feed(t, m, app.DirectoryLoadedMsg{RequestID: requestID, Path: path, Entries: entries, Err: err})
}

func entriesAt(base string, count int) []browser.Entry {
	entries := make([]browser.Entry, count)
	for i := range entries {
		name := fmt.Sprintf("entry-%02d.txt", i)
		entries[i] = browser.Entry{Name: name, Path: filepath.Join(base, name)}
	}

	return entries
}

// gridOnly presses ~ to hide the sidebar so the grid spans the full width.
func gridOnly(t *testing.T, m *app.Model) *app.Model {
	t.Helper()

	return press(t, m, "~")
}

func TestInitialLoadAppliesEntries(t *testing.T) {
	t.Parallel()

	m := app.New("/tmp/demo", app.Options{})
	if !m.IsLoading() {
		t.Error("a fresh model should be loading")
	}
	if m.Init() == nil {
		t.Error("Init must return the initial load command")
	}

	m = resize(t, m, 80, 24)
	m = loaded(t, m, 1, "/tmp/demo", []browser.Entry{
		{Name: "src", Path: "/tmp/demo/src", IsDir: true},
		{Name: "main.go", Path: "/tmp/demo/main.go"},
	}, nil)

	if m.IsLoading() {
		t.Error("loading should end once the result arrives")
	}
	if len(m.Entries()) != 2 {
		t.Errorf("entries = %d, want 2", len(m.Entries()))
	}
	if got := m.Path(); got != "/tmp/demo" {
		t.Errorf("Path = %q, want /tmp/demo", got)
	}
}

func TestStaleDirectoryResultIsIgnored(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/tmp/old", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/tmp/old", entriesAt("/tmp/old", 3), nil)

	// Navigating to the parent starts request 2. Its result applies...
	m = pressBackspace(t, m)
	m = loaded(t, m, 2, "/tmp", []browser.Entry{{Name: "x", Path: "/tmp/x"}}, nil)
	if got := m.Path(); got != "/tmp" {
		t.Fatalf("newest result should apply, Path = %q", got)
	}

	// ...while the late result of superseded request 1 must be dropped
	// instead of replacing the newer location.
	m = loaded(t, m, 1, "/tmp/old", entriesAt("/tmp/old", 3), nil)
	if got := m.Path(); got != "/tmp" {
		t.Errorf("stale result replaced the current directory: Path = %q", got)
	}
	if m.IsLoading() {
		t.Error("loading should be false after the newest result")
	}
}

func TestLoadErrorIsStoredAndRendered(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/tmp/boom", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/tmp/boom", nil, errTestLoad)

	if !errors.Is(m.LoadError(), errTestLoad) {
		t.Errorf("LoadError = %v, want %v", m.LoadError(), errTestLoad)
	}
	if view := m.View(); !strings.Contains(view, "permission denied") {
		t.Errorf("view should surface the load error, got %q", view)
	}
}

func TestNavigationKeysMoveFocus(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	// 12 entries at 5 columns:
	//   row 0: 00 01 02 03 04
	//   row 1: 05 06 07 08 09
	//   row 2: 10 11
	m = loaded(t, m, 1, "/d", entriesAt("/d", 12), nil)

	if got := m.FocusedPath(); !strings.HasSuffix(got, "entry-00.txt") {
		t.Errorf("initial focus = %q, want entry-00.txt", got)
	}
	m = press(t, m, "j")
	if got := m.FocusedPath(); !strings.HasSuffix(got, "entry-05.txt") {
		t.Errorf("focus after j = %q, want entry-05.txt", got)
	}
	m = press(t, m, "l")
	if got := m.FocusedPath(); !strings.HasSuffix(got, "entry-06.txt") {
		t.Errorf("focus after l = %q, want entry-06.txt", got)
	}
	m = press(t, m, "h")
	m = press(t, m, "k")
	if got := m.FocusedPath(); !strings.HasSuffix(got, "entry-00.txt") {
		t.Errorf("focus after h,k = %q, want entry-00.txt", got)
	}
}

func TestBackspaceAtLeftEdgeGoesToParent(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d/sub", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d/sub", entriesAt("/d/sub", 6), nil)

	m = press(t, m, "h") // moves left, no navigation
	if m.Path() != "/d/sub" {
		t.Errorf("h with room to move should not navigate, Path = %q", m.Path())
	}

	m = pressBackspace(t, m)
	if m.Path() != "/d/sub" {
		t.Errorf("Path should not change until the load completes, got %q", m.Path())
	}
	if !m.IsLoading() {
		t.Error("backspace should start a parent load")
	}
}

func TestBackspaceAtRootIsNoOp(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/", []browser.Entry{{Name: "x", Path: "/x"}}, nil)

	m = pressBackspace(t, m)
	if m.IsLoading() {
		t.Error("backspace at the filesystem root should do nothing")
	}
}

func TestQuitKeysReturnQuitCommand(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Error("q should produce a quit command")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c should produce a quit command")
	}
}

func TestResizePreservesFocusedEntry(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 120, 30)
	m = gridOnly(t, m)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 30), nil)
	focused := m.FocusedPath()
	if focused == "" {
		t.Fatal("expected a focused entry")
	}

	// Shrink to compact mode: the same entry must stay focused.
	m = resize(t, m, 60, 15)
	if got := m.FocusedPath(); got != focused {
		t.Errorf("focus after resize = %q, want %q", got, focused)
	}
}

func TestViewRendersHeaderStatusAndEntries(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/tmp/demo", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/tmp/demo", []browser.Entry{
		{Name: "src", Path: "/tmp/demo/src", IsDir: true},
		{Name: "main.go", Path: "/tmp/demo/main.go"},
	}, nil)

	// The header renders breadcrumbs, not the raw path: segments joined by
	// separators.
	view := m.View()
	if !strings.Contains(view, "/tmp") || !strings.Contains(view, "demo") {
		t.Errorf("view should show the location crumbs, got %q", view)
	}
	if !strings.Contains(view, "main.go") {
		t.Errorf("view should show card names, got %q", view)
	}
	if !strings.Contains(view, "2 items") {
		t.Errorf("view should show the item count, got %q", view)
	}
	if !strings.Contains(view, "name asc") {
		t.Errorf("view should show the sort state, got %q", view)
	}
}

func TestViewTooSmallKeepsState(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{{Name: "keep.txt", Path: "/d/keep.txt"}}, nil)

	m = resize(t, m, 4, 3)
	if view := m.View(); !strings.Contains(view, "too small") {
		t.Errorf("tiny terminal should show the resize message, got %q", view)
	}
	entries := m.Entries()
	if len(entries) != 1 || entries[0].Name != "keep.txt" {
		t.Error("state must be retained while the resize message is shown")
	}

	m = resize(t, m, 80, 24)
	if !strings.Contains(m.View(), "keep.txt") {
		t.Error("growing back should restore the grid with state intact")
	}
}

func TestViewRendersEveryRowWithGaps(t *testing.T) {
	t.Parallel()

	// 12 entries at 80x24 (sidebar off, 5 columns) render 3 rows with a
	// spacer line between card rows. Regression: an inner-loop index bug
	// used to drop every second row.
	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 12), nil)

	view := m.View()
	for _, want := range []string{"entry-00.txt", "entry-05.txt", "entry-10.txt"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should render %q (rows 0, 1, and 2)", want)
		}
	}

	lines := strings.Split(view, "\n")
	// Header (1) + card row 0 (5 lines) lands the first gap at line 7 (index 6).
	if len(lines) < 8 {
		t.Fatalf("view has %d lines, want at least 8", len(lines))
	}
	if strings.TrimSpace(lines[6]) != "" {
		t.Errorf("line 7 should be the blank gap between card rows, got %q", lines[6])
	}
	if strings.TrimSpace(lines[12]) != "" {
		t.Errorf("line 13 should be the blank gap between card rows, got %q", lines[12])
	}
}

func TestEnterOnSymlinkFileShowsNoteAndClearsLoading(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "f.txt", Path: "/d/f.txt", Symlink: true},
	}, nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if !opened.IsLoading() {
		t.Fatal("entering a symlink should start a resolve request")
	}

	opened = feed(t, opened, app.EntryNotDirectoryMsg{Path: "/d/f.txt", RequestID: 2})
	if opened.IsLoading() {
		t.Error("a completed resolve must end the loading state")
	}
	if view := opened.View(); !strings.Contains(view, "not a directory: f.txt") {
		t.Errorf("view should surface the note, got %q", view)
	}
}

func TestStaleEntryNotDirectoryIsIgnored(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "f.txt", Path: "/d/f.txt", Symlink: true},
	}, nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	opened = feed(t, opened, app.EntryNotDirectoryMsg{Path: "/d/f.txt", RequestID: 1})
	if !opened.IsLoading() {
		t.Error("a stale resolve result must not end the current request")
	}
	if view := opened.View(); strings.Contains(view, "not a directory") {
		t.Errorf("a stale resolve result must not surface its note, got %q", view)
	}
}

func TestFailedNavigationShowsErrorWhileEntriesRemain(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d/sub", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d/sub", entriesAt("/d/sub", 6), nil)

	m = pressBackspace(t, m)
	m = loaded(t, m, 2, "/d", nil, errTestLoad)

	entries := m.Entries()
	if len(entries) != 6 {
		t.Errorf("previous entries should remain visible after a failed load, got %d", len(entries))
	}
	if view := m.View(); !strings.Contains(view, "permission denied") {
		t.Errorf("view should surface the navigation error, got %q", view)
	}
}

func TestErrorStateIsSanitized(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", nil, errTestEscape)

	view := m.View()
	if strings.ContainsRune(view, '\x1b') {
		t.Error("view must not contain raw escape characters from error strings")
	}
	if !strings.Contains(view, "[31mred") {
		t.Errorf("sanitized error fragment should be visible, got %q", view)
	}
}
