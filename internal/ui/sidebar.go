package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SidebarItem is one entry in the places sidebar.
type SidebarItem struct {
	Label string
	Path  string
}

// RenderSidebar draws the places panel at the given width. selected is the
// highlighted index; focused adds the active marker and bright styling used
// when the sidebar owns keyboard focus.
func RenderSidebar(width, height int, items []SidebarItem, selected int, focused bool) string {
	if width < 1 {
		width = 1
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).
		Render(" places")

	lines := make([]string, 0, 2+len(items))
	lines = append(lines, title, "")
	rowStyle := lipgloss.NewStyle().Faint(true)
	if focused {
		rowStyle = lipgloss.NewStyle()
	}

	for i, item := range items {
		marker := "  "
		if focused && i == selected {
			marker = "> "
		}

		row := rowStyle.Render(TruncateName(SanitizeName(item.Label), width-3))
		lines = append(lines, marker+row)
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
func RenderSidebarOverlay(gridFrame string, sidebarWidth int, items []SidebarItem, selected int) string {
	panel := RenderSidebar(sidebarWidth, strings.Count(gridFrame, "\n")+1, items, selected, true)
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
