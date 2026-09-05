package app_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/operations"
)

// errTestLoad is a package-level sentinel so tests can assert error
// identity through errors.Is.
var errTestLoad = errors.New("permission denied")

// errTestEscape embeds a control character to prove rendering sanitizes it.
var errTestEscape = errors.New("esc \x1b[31mred")

// feed applies a message to the model and returns the updated model.
func feed(tb testing.TB, m tea.Model, msg tea.Msg) *app.Model {
	tb.Helper()
	next, _ := m.Update(msg)
	updated, ok := next.(*app.Model)
	if !ok {
		tb.Fatalf("Update returned %T, want *app.Model", next)
	}

	return updated
}

// namedKeyTypes maps a key's string vocabulary to the key type a real
// terminal reports for it: named keys are structurally different from
// rune keys, and the helper must not smuggle them through as runes — a
// typed burst of t,a,b arrives as one rune message whose text happens to
// be "tab", and only the type distinguishes them.
var namedKeyTypes = map[string]tea.KeyType{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEsc,
	"tab":       tea.KeyTab,
	"backspace": tea.KeyBackspace,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"ctrl+c":    tea.KeyCtrlC,
	"ctrl+a":    tea.KeyCtrlA,
	"ctrl+d":    tea.KeyCtrlD,
	"ctrl+s":    tea.KeyCtrlS,
	"ctrl+u":    tea.KeyCtrlU,
}

// keyMsg builds the key message a real terminal would deliver for s.
func keyMsg(s string) tea.KeyMsg {
	if kt, ok := namedKeyTypes[s]; ok {
		return tea.KeyMsg{Type: kt}
	}

	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(t *testing.T, m *app.Model, s string) *app.Model {
	t.Helper()

	return feed(t, m, keyMsg(s))
}

func pressBackspace(t *testing.T, m *app.Model) *app.Model {
	t.Helper()

	return feed(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
}

func resize(tb testing.TB, m *app.Model, w, h int) *app.Model {
	tb.Helper()

	return feed(tb, m, tea.WindowSizeMsg{Width: w, Height: h})
}

func loaded(tb testing.TB, m *app.Model, requestID uint64, path string, entries []browser.Entry, err error) *app.Model {
	tb.Helper()

	return feed(tb, m, app.DirectoryLoadedMsg{RequestID: requestID, Path: path, Entries: entries, Err: err})
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

// TestFastDoubleQuit pins that a quickly repeated q — which arrives as one
// multi-rune key message, string "qq" — still quits: each rune is replayed
// as its own keypress. Without this, users pressing q repeatedly because
// "it does not work" only ever produce keys nothing listens to.
func TestFastDoubleQuit(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("qq")}); cmd == nil {
		t.Error("a fast qq burst should produce a quit command")
	}
}

// TestRuneBurstKeepsEveryCommand pins that a burst like rj preserves the
// refresh command the r started: overwriting it with the j's nil would
// strand the browser in the loading state with no load in flight.
func TestRuneBurstKeepsEveryCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 80, 24))
	m = loaded(t, m, 1, root, entriesAt(root, 10), nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("rj")})
	if cmd == nil {
		t.Fatal("a burst whose first key starts a refresh must keep that command")
	}
	m = next.(*app.Model)
	if !m.IsLoading() {
		t.Fatal("r should have started a load")
	}

	// The preserved command must actually run the load to completion —
	// with the old bug the j's nil overwrote it and the model was stuck
	// loading forever.
	for _, msg := range runBatch(t, cmd) {
		if loaded, ok := msg.(app.DirectoryLoadedMsg); ok {
			m = feed(t, m, loaded)
		}
	}
	if m.IsLoading() {
		t.Fatal("the load never completed: the refresh command was lost")
	}
	if m.LoadError() != nil {
		t.Fatalf("load failed: %v", m.LoadError())
	}
}

// TestRuneBurstReplaysEachRune pins that a fast jjj run navigates three
// entries down, like three separate presses would.
func TestRuneBurstReplaysEachRune(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 30), nil)

	// Five columns at this width, so each j steps a full row: three
	// replayed presses land five rows down.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjj")})
	m = next.(*app.Model)
	if got := m.FocusedPath(); filepath.Base(got) != "entry-15.txt" {
		t.Fatalf("focus after jjj = %q, want entry-15", got)
	}
}

// TestBracketedPasteStaysOpaque pins that multi-rune input marked as a
// paste is not replayed as key presses: paste is text, and treating it as
// keys would let pasted content trigger shortcuts.
func TestBracketedPasteStaysOpaque(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("qq"), Paste: true})
	if cmd != nil {
		t.Error("a pasted qq must not trigger quit or any other shortcut")
	}
	if _, ok := next.(*app.Model); !ok {
		t.Fatalf("paste returned an unexpected model type %T", next)
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

func TestEnterOnSymlinkedFileOpensIt(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "f.bin", Path: "/d/f.bin", Symlink: true},
	}, nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	if !opened.IsLoading() {
		t.Fatal("entering a symlink should start a resolve request")
	}

	// The resolve lands on a file target: the request completes and the
	// file goes to the opener instead of dying with a note.
	resolved := feed(t, opened, app.EntryResolvedMsg{Path: "/d/f.bin", RequestID: 2})
	if resolved.IsLoading() {
		t.Error("a completed resolve must end the loading state")
	}
}

func TestStaleEntryResolvedIsIgnored(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", []browser.Entry{
		{Name: "f.bin", Path: "/d/f.bin", Symlink: true},
	}, nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(*app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want *app.Model", next)
	}
	resolved := feed(t, opened, app.EntryResolvedMsg{Path: "/d/f.bin", RequestID: 1})
	if !resolved.IsLoading() {
		t.Error("a stale resolve result must not end the current request")
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

func TestFinishedEventRearmsOperationListener(t *testing.T) {
	t.Parallel()

	m := resize(t, app.New("/d", app.Options{}), 80, 24)
	m = loaded(t, m, 1, "/d", entriesAt("/d", 3), nil)

	_, cmd := m.Update(app.OperationEventMsg{Event: operations.FinishedEvent{
		ID:     "op-1",
		Result: operations.Result{OpID: "op-1", Kind: operations.OpCopy, Succeeded: 1},
	}})
	if cmd == nil {
		t.Fatal("a finished event must return a command")
	}

	// The refresh and the re-armed listener must come back together:
	// returning only the refresh dropped the listener, and the serial
	// manager then blocked forever on its next event publication.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("finished event command produced %T, want tea.BatchMsg", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("batch has %d commands, want refresh + operation listener (2)", len(batch))
	}
}

// TestLeftEdgeHIsANoOp pins that h and left at the left edge do nothing:
// movement never doubles as navigation, so the parent is reached only
// through backspace.
func TestLeftEdgeHIsANoOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	m := gridOnly(t, resize(t, app.New(sub, app.Options{}), 80, 24))
	m = loaded(t, m, 1, sub, entriesAt(sub, 3), nil)
	before := m.Path()

	next, _ := m.Update(keyMsg("h"))
	m = next.(*app.Model)
	if m.Path() != before {
		t.Fatalf("h at the left edge navigated to %q, want no movement", m.Path())
	}
	if m.IsLoading() {
		t.Fatal("h at the left edge must not start a parent load")
	}

	next, _ = m.Update(keyMsg("left"))
	m = next.(*app.Model)
	if m.Path() != before {
		t.Fatalf("left at the left edge navigated to %q, want no movement", m.Path())
	}

	// Backspace remains the parent shortcut.
	next, _ = m.Update(keyMsg("backspace"))
	m = next.(*app.Model)
	if !m.IsLoading() && m.Path() == before {
		t.Fatal("backspace should navigate to the parent")
	}
}
