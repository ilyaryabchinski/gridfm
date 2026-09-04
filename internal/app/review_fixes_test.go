package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/operations"
)

// failedResult is a finished job with a failure, which opens the blocking
// results overlay.
func failedResult() operations.Result {
	return operations.Result{
		OpID: "op-1", Kind: operations.OpCopy, Failed: 1,
		Failures: []operations.ItemError{{Path: "/d/x", Err: errTestLoad}},
	}
}

// TestFinishedEventClosesOtherOverlays pins the one-blocking-overlay rule:
// a finished job's results overlay must not coexist with an open create
// input, confirmation dialog, sort menu, or filter.
func TestFinishedEventClosesOtherOverlays(t *testing.T) {
	t.Parallel()

	m := gridOnly(t, resize(t, app.New("/d", app.Options{}), 80, 24))
	m = loaded(t, m, 1, "/d", entriesAt("/d", 2), nil)

	// Create input open when the results arrive: the input closes.
	m = press(t, m, "n")
	m = press(t, m, "a")
	m = feed(t, m, app.OperationEventMsg{Event: operations.FinishedEvent{ID: "op-1", Result: failedResult()}})
	if view := m.View(); !strings.Contains(view, "permission denied") {
		t.Fatal("the results overlay should render")
	}
	if strings.Contains(m.View(), "NEW FILE") {
		t.Error("the create input stayed open beside the results overlay")
	}
	// Keys reach the results overlay, not the closed input.
	m = press(t, m, "e")
	if strings.Contains(m.View(), "permission denied") {
		t.Error("e should close the results overlay")
	}
	if strings.Contains(m.View(), "NEW FILE") {
		t.Error("the closed input must not come back")
	}

	// Confirmation dialog open when the results arrive: it closes.
	m = press(t, m, "D")
	if !strings.Contains(m.View(), "DELETE PERMANENTLY") {
		t.Fatal("D should open the delete confirmation")
	}
	m = feed(t, m, app.OperationEventMsg{Event: operations.FinishedEvent{ID: "op-2", Result: failedResult()}})
	if view := m.View(); strings.Contains(view, "DELETE PERMANENTLY") {
		t.Error("the confirmation stayed open beside the results overlay")
	}
	if !strings.Contains(m.View(), "permission denied") {
		t.Error("the results overlay should render over the dialog")
	}

	// Sort menu open when the results arrive: it closes.
	m = press(t, m, "e") // close results first
	m = press(t, m, "s")
	if !strings.Contains(m.View(), "sort") {
		t.Fatal("s should open the sort menu")
	}
	m = feed(t, m, app.OperationEventMsg{Event: operations.FinishedEvent{ID: "op-3", Result: failedResult()}})
	if view := m.View(); strings.Contains(view, "enter apply") {
		t.Error("the sort menu stayed open beside the results overlay")
	}
	if !strings.Contains(m.View(), "permission denied") {
		t.Error("the results overlay should render over the sort menu")
	}
}

// TestInspectorOverlaysInNarrowTerminals pins the narrow-terminal form: i
// still works below the docking threshold, rendering the panel over the
// grid's right edge instead of silently doing nothing.
func TestInspectorOverlaysInNarrowTerminals(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	victim := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(victim, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 60 columns is below NarrowThreshold (70).
	m := gridOnly(t, resize(t, app.New(root, app.Options{}), 60, 15))
	m = loaded(t, m, 1, root, []browser.Entry{
		{Name: "notes.txt", Path: victim},
	}, nil)

	m, cmd := pressI(t, m)
	if !m.InspectorOn() {
		t.Fatal("i should toggle the inspector on in narrow terminals")
	}
	if cmd == nil {
		t.Fatal("opening the inspector should request metadata")
	}

	m = feed(t, m, cmd().(app.InspectorLoadedMsg))
	view := m.View()
	if !strings.Contains(view, "inspector") || !strings.Contains(view, "notes.txt") {
		t.Errorf("the overlaid inspector should render its panel, got:\n%s", view)
	}

	// i again closes it.
	m, _ = pressI(t, m)
	if strings.Contains(m.View(), "notes.txt") {
		t.Error("the closed inspector should not render")
	}
}
