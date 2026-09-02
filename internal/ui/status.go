package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/x/ansi"
)

// RenderHeader draws the top bar: the current location, truncated to the
// terminal width.
func RenderHeader(width int, path string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(width).
		MaxWidth(width).
		Render(TruncateName(SanitizeName(path), width))
}

// RenderStatusBar draws the bottom bar: mode, transient note, and item
// count, right aligned.
func RenderStatusBar(width int, itemCount int, loading bool, note string) string {
	left := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).
		Render(" NORMAL")
	if loading {
		left += lipgloss.NewStyle().Faint(true).Render(" loading…")
	}
	if note != "" {
		left += lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(" " + SanitizeName(note))
	}

	right := " "
	if itemCount == 0 {
		right += "empty"
	} else {
		right += strconv.Itoa(itemCount) + " items"
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
		lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(message))
}
