package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SidebarItem is one entry in the sidebar. Section names the group the
// entry belongs to ("places", "bookmarks", "mounts", "recents"); the panel
// renders a header whenever the section changes.
type SidebarItem struct {
	Label   string
	Path    string
	Section string
}

// RenderSidebar draws the places panel at the given width. selected is the
// highlighted index into the flat item list; focused adds the active marker
// and bright styling used when the sidebar owns keyboard focus. operations
// renders the non-blocking operation shelf beneath the sections: one line
// for a running job plus the most recent finished summaries, already
// summarized by the caller.
func RenderSidebar(width, height int, items []SidebarItem, selected int, focused bool, operations []string) string {
	if width < 1 {
		width = 1
	}

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	lines := make([]string, 0, 4+len(items)+len(operations))
	rowStyle := lipgloss.NewStyle().Faint(true)
	if focused {
		rowStyle = lipgloss.NewStyle()
	}

	section := ""
	for i, item := range items {
		name := item.Section
		if name == "" {
			name = "places" // unsectioned items keep the original header
		}
		if name != section {
			section = name
			lines = append(lines, "", sectionStyle.Render(" "+section))
		}

		marker := "  "
		if focused && i == selected {
			marker = "> "
		}

		row := rowStyle.Render(TruncateName(SanitizeName(item.Label), width-3))
		lines = append(lines, marker+row)
	}

	if len(operations) > 0 {
		lines = append(lines, "", sectionStyle.Render(" operations"))
		for _, op := range operations {
			lines = append(lines, "  "+rowStyle.Render(TruncateName(SanitizeName(op), width-3)))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, sectionStyle.Render(" places"))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		MaxHeight(max(height, 1)).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("8")).
		Render(body)
}

// RenderSidebarOverlay draws the sidebar floating over a grid frame: the
// sidebar lines replace the leftmost cells of each grid line. Overlay lines
// are cut with ANSI awareness so styling survives.
func RenderSidebarOverlay(gridFrame string, sidebarWidth int, items []SidebarItem, selected int, operations []string) string {
	panel := RenderSidebar(sidebarWidth, strings.Count(gridFrame, "\n")+1, items, selected, true, operations)
	panelLines := strings.Split(panel, "\n")
	gridLines := strings.Split(gridFrame, "\n")

	out := make([]string, 0, len(gridLines))
	for i, gridLine := range gridLines {
		if i >= len(panelLines) {
			out = append(out, gridLine)

			continue
		}

		right := ansiCut(gridLine, sidebarWidth)
		out = append(out, panelLines[i]+right)
	}

	return strings.Join(out, "\n")
}
