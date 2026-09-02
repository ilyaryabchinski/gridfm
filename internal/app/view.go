package app

import (
	"gridfm/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// View renders one frame. It is a pure function of model state: no I/O, no
// mutation.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "gridfm: loading…"
	}
	l := m.layout()
	if !l.Usable {
		return ui.RenderTooSmall(m.width, m.height)
	}

	body := m.renderBody(l)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ui.RenderHeader(m.width, m.browser.Path),
		body,
		ui.RenderStatusBar(m.width, len(m.browser.Entries), m.loading, m.note),
	)
}

func (m *Model) layout() ui.Layout {
	return ui.ComputeLayout(m.width, m.height)
}

// renderBody draws the grid viewport or the full-body state message.
func (m *Model) renderBody(l ui.Layout) string {
	var body string
	switch {
	case m.loadErr != nil && len(m.browser.Entries) == 0:
		body = renderStateLine(l, "error: "+m.loadErr.Error())
	case m.loading && len(m.browser.Entries) == 0:
		body = renderStateLine(l, "loading…")
	case len(m.browser.Entries) == 0:
		body = renderStateLine(l, "empty directory")
	default:
		body = m.renderViewport(l)
	}

	return lipgloss.NewStyle().
		Height(l.ContentHeight).
		MaxHeight(l.ContentHeight).
		Width(l.ContentWidth).
		MaxWidth(l.ContentWidth).
		Render(body)
}

// renderViewport renders only the visible cards: rows scrollRow through
// scrollRow+RowsVisible of the grid.
func (m *Model) renderViewport(l ui.Layout) string {
	nav := m.browser.Nav()
	entries := m.browser.Entries
	focus := nav.Focus()

	first := m.scrollRow * l.Columns
	last := min(first+l.Columns*l.RowsVisible, len(entries))

	rows := make([]string, 0, l.RowsVisible)
	for start := first; start < last; start += l.Columns {
		row := make([]string, 0, l.Columns)
		for end := min(start+l.Columns, last); start < end; start++ {
			row = append(row, ui.RenderCard(entries[start], l.Card, start == focus))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderStateLine centers a single status message in the grid area.
func renderStateLine(l ui.Layout, message string) string {
	return lipgloss.Place(
		l.ContentWidth,
		max(l.ContentHeight, 1),
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().Faint(true).Render(message),
	)
}
