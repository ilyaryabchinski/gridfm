package ui

import (
	"strings"
	"testing"
)

func TestRenderSidebarShowsOperationsShelf(t *testing.T) {
	t.Parallel()

	out := RenderSidebar(24, 20, []SidebarItem{{Label: "Home", Path: "/home"}}, 0, false,
		[]string{"• copy 1/2 big.bin", "✓ trash: 1 ok, 0 skipped, 0 faileds"})

	for _, want := range []string{"places", "Home", "operations", "copy 1/2", "trash: 1 ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("sidebar should show %q, got:\n%s", want, out)
		}
	}

	// Long lines are cut to the panel width.
	if strings.Contains(out, "0 skipped, 0 faileds") {
		t.Error("operation lines should be truncated to the sidebar width")
	}
}

func TestRenderSidebarWithoutOperations(t *testing.T) {
	t.Parallel()

	out := RenderSidebar(24, 20, []SidebarItem{{Label: "Home", Path: "/home"}}, 0, false, nil)
	if strings.Contains(out, "operations") {
		t.Error("the operations header should not render when the shelf is empty")
	}
}
