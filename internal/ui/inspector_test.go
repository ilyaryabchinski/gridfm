package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gridfm/internal/preview"
)

func TestComputeLayoutDocksInspector(t *testing.T) {
	t.Parallel()

	full := ComputeLayout(120, 30, ZoomNormal, true)
	with := ComputeLayoutWithInspector(120, 30, ZoomNormal, true, true)

	if full.InspectorWidth != 0 {
		t.Errorf("inspector off should reserve no width, got %d", full.InspectorWidth)
	}
	if with.InspectorWidth != InspectorWidth {
		t.Errorf("inspector on should reserve %d, got %d", InspectorWidth, with.InspectorWidth)
	}
	if want := full.GridWidth - InspectorWidth; with.GridWidth != want {
		t.Errorf("grid width = %d, want %d", with.GridWidth, want)
	}
	// The grid stays usable with both panels docked.
	if !with.Usable || with.Columns < 1 {
		t.Errorf("layout with inspector should stay usable: %+v", with)
	}
}

func TestComputeLayoutInspectorDisabledWhenNarrow(t *testing.T) {
	t.Parallel()

	l := ComputeLayoutWithInspector(60, 15, ZoomNormal, true, true)
	if l.InspectorWidth != 0 {
		t.Errorf("narrow layouts must not dock the inspector, got %d", l.InspectorWidth)
	}
	if !l.SidebarOverlay {
		t.Error("narrow layout should keep the sidebar overlay")
	}
}

func TestRenderInspectorStates(t *testing.T) {
	t.Parallel()

	loading := RenderInspector(26, 20, nil, nil, true)
	if !strings.Contains(loading, "inspector") || !strings.Contains(loading, "loading") {
		t.Errorf("loading state = %q", loading)
	}

	failed := RenderInspector(26, 20, nil, errors.New("lstat gone: no such file"), false)
	if !strings.Contains(failed, "lstat gone") {
		t.Errorf("error state = %q", failed)
	}

	info := &preview.Info{
		Path: "/tmp/demo/notes.txt", Name: "notes.txt", Size: 1536,
		Mode: "-rw-r--r--", Owner: "ilya", Group: "users",
		ModTime: time.Unix(1700000000, 0),
		Preview: []string{"first line"},
	}
	loaded := RenderInspector(26, 20, info, nil, false)
	for _, want := range []string{"notes.txt", "1.5 KB", "rw-r--r--", "ilya", "first line"} {
		if !strings.Contains(loaded, want) {
			t.Errorf("loaded panel should show %q, got:\n%s", want, loaded)
		}
	}
}

func TestRenderInspectorBrokenSymlinkWarning(t *testing.T) {
	t.Parallel()

	info := &preview.Info{
		Path: "/tmp/demo/broken", Name: "broken", Symlink: true,
		LinkTarget: "/tmp/demo/ghost", TargetMissing: true,
	}
	out := RenderInspector(30, 20, info, nil, false)
	if !strings.Contains(out, "-> /tmp/demo/ghost") {
		t.Errorf("symlink target should render, got:\n%s", out)
	}
}
