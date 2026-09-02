package ui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/lipgloss"
)

// ansiCut removes the first width display cells from s, preserving the
// styling of what remains.
func ansiCut(s string, width int) string {
	if width <= 0 {
		return s
	}
	total := ansi.StringWidth(s)
	if total <= width {
		return ""
	}

	return ansi.Cut(s, width, total)
}

// RenderBreadcrumbs draws the top bar: back and forward history indicators
// followed by the current location as breadcrumb segments separated by
// slashes. The home prefix abbreviates to ~, and leading segments collapse
// into an ellipsis when the location is longer than the terminal.
func RenderBreadcrumbs(width int, path, home string, canBack, canForward bool) string {
	history := renderHistoryIndicators(canBack, canForward)
	segments := crumbSegments(path, home)
	line := history + renderCrumbs(segments)

	for len(segments) > 2 && ansi.StringWidth(line) > width {
		segments = collapseLeadSegment(segments)
		line = history + renderCrumbs(segments)
	}

	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Render(TruncateName(line, width))
}

// renderHistoryIndicators renders the back and forward arrows, bright when
// the direction is available and faint when exhausted.
func renderHistoryIndicators(canBack, canForward bool) string {
	dim := lipgloss.NewStyle().Faint(true)
	live := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	back, forward := dim.Render("←"), dim.Render("→")
	if canBack {
		back = live.Render("←")
	}
	if canForward {
		forward = live.Render("→")
	}

	return back + " " + forward + " "
}

// crumbSegments turns a path into display segments: ~ for the home prefix,
// and a merged root crumb for absolute paths so "/tmp" renders as one
// segment rather than "/ / tmp".
func crumbSegments(path, home string) []string {
	display := path
	if home != "" && (path == home || strings.HasPrefix(path, home+string(filepath.Separator))) {
		display = "~" + strings.TrimPrefix(path, home)
	}

	segments := strings.Split(display, string(filepath.Separator))
	if len(segments) > 1 && segments[0] == "" {
		segments = append([]string{"/" + segments[1]}, segments[2:]...)
	}

	return segments
}

// collapseLeadSegment drops the oldest crumb and marks the new leading
// segment with an ellipsis.
func collapseLeadSegment(segments []string) []string {
	segments = segments[1:]
	if len(segments) > 0 {
		segments[0] = "…" + segments[0]
	}

	return segments
}

// renderCrumbs styles and joins segments. Filesystem-provided text is
// sanitized before styling so escape sequences in paths can never reach the
// terminal.
func renderCrumbs(segments []string) string {
	separator := lipgloss.NewStyle().Faint(true).Render(" / ")
	crumbStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	rendered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			rendered = append(rendered, crumbStyle.Render("/"))

			continue
		}
		rendered = append(rendered, crumbStyle.Render(SanitizeName(segment)))
	}

	return strings.Join(rendered, separator)
}
