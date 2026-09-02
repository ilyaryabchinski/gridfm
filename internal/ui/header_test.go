package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gridfm/internal/ui"
)

func TestBreadcrumbsRenderSegments(t *testing.T) {
	t.Parallel()

	line := ui.RenderBreadcrumbs(80, "/home/tester/Projects/gridfm", "/home/tester", true, false)
	if !strings.Contains(line, "~") {
		t.Errorf("home prefix should abbreviate to ~, got %q", line)
	}
	if !strings.Contains(line, "Projects") || !strings.Contains(line, "gridfm") {
		t.Errorf("breadcrumbs should show trailing segments, got %q", line)
	}
	if !strings.Contains(line, "←") || !strings.Contains(line, "→") {
		t.Errorf("history indicators should render, got %q", line)
	}
	if w := ansi.StringWidth(line); w > 80 {
		t.Errorf("breadcrumbs are %d cells wide, want <= 80", w)
	}
}

func TestBreadcrumbsCollapseLongPaths(t *testing.T) {
	t.Parallel()

	long := "/very/deep/nested/tree/with/many/segments/and/a/long/leaf"
	line := ui.RenderBreadcrumbs(30, long, "", false, false)
	if !strings.Contains(line, "…") {
		t.Errorf("long paths should collapse with an ellipsis, got %q", line)
	}
	if !strings.Contains(line, "leaf") {
		t.Errorf("the leaf segment should survive collapsing, got %q", line)
	}
	if w := ansi.StringWidth(line); w > 30 {
		t.Errorf("breadcrumbs are %d cells wide, want <= 30", w)
	}
}

func TestBreadcrumbsRenderRoot(t *testing.T) {
	t.Parallel()

	line := ui.RenderBreadcrumbs(80, "/tmp/demo", "", false, false)
	if !strings.Contains(line, "/tmp") {
		t.Errorf("absolute root should merge into the first crumb, got %q", line)
	}
	if strings.Contains(line, "/ / ") {
		t.Errorf("no double slashes between crumbs, got %q", line)
	}
}

func TestBreadcrumbsSanitizeSegments(t *testing.T) {
	t.Parallel()

	line := ui.RenderBreadcrumbs(80, "/d/evil\x1b[31mname", "", false, false)
	if strings.ContainsRune(line, '\x1b') {
		t.Error("breadcrumbs must not contain raw escape characters from paths")
	}
}
