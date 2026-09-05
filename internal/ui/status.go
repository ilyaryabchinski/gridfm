package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/x/ansi"
)

// StatusInfo carries everything the status bar renders.
type StatusInfo struct {
	// Note is a transient message (errors, open results); it wins over
	// indicators when present.
	Note string
	// Filter is the active incremental filter query, empty when inactive.
	Filter string
	// Mode is the input mode label, normally NORMAL.
	Mode string
	// Sort renders the active sort, e.g. "name asc".
	Sort string
	// HiddenOn reflects the hidden-file toggle.
	HiddenOn bool
	// Loading shows the in-flight indicator.
	Loading bool
	// Items and TotalItems are the visible and full entry counts; both are
	// shown only when they differ.
	Items      int
	TotalItems int
}

// RenderStatusBar draws the bottom bar: mode and browsing indicators on the
// left, item counts on the right.
func RenderStatusBar(width int, info StatusInfo) string {
	left := lipgloss.NewStyle().Bold(true).Foreground(CurrentTheme().Strong).
		Render(" " + info.Mode)
	if info.Loading {
		left += lipgloss.NewStyle().Faint(true).Render(" loading…")
	}
	if info.Sort != "" {
		left += lipgloss.NewStyle().Faint(true).Render(" " + info.Sort)
	}
	if info.HiddenOn {
		left += lipgloss.NewStyle().Foreground(CurrentTheme().Warning).Render(" hidden on")
	}
	if info.Filter != "" {
		left += lipgloss.NewStyle().Foreground(CurrentTheme().Info).
			Render(" /" + SanitizeName(info.Filter))
	}
	if info.Note != "" {
		left += lipgloss.NewStyle().Foreground(CurrentTheme().Warning).Render(" " + SanitizeName(info.Note))
	}

	right := " "
	switch {
	case info.Items == 0 && info.TotalItems == 0:
		right += "empty"
	case info.Items != info.TotalItems:
		right += strconv.Itoa(info.Items) + " of " + strconv.Itoa(info.TotalItems) + " items"
	default:
		right += strconv.Itoa(info.Items) + " items"
	}
	right += " "

	pad := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	bar := left + strings.Repeat(" ", max(pad, 1)) + right

	return lipgloss.NewStyle().
		Faint(true).
		Width(width).
		MaxWidth(width).
		Render(bar)
}

// RenderTooSmall draws the minimal message shown when the terminal cannot
// fit even one card. Application state is retained while it is displayed.
func RenderTooSmall(width, height int) string {
	message := "terminal too small to render the grid"

	return lipgloss.Place(width, max(height, 1), lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(CurrentTheme().Warning).Render(message))
}
