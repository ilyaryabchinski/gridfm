package app

import (
	"strings"

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
// scrollRow+RowsVisible of the grid. Card gaps are rendered as literal
// spacer cells so the visible geometry matches the layout calculations.
func (m *Model) renderViewport(l ui.Layout) string {
	entries := m.browser.Entries
	focus := m.browser.FocusIndex()

	first := m.scrollRow * l.Columns
	last := min(first+l.Columns*l.RowsVisible, len(entries))

	cardGap := strings.Repeat(" ", ui.CardGapX)
	// A spacer block of full card height so JoinHorizontal keeps every card
	// on the same baseline.
	gapBlock := strings.TrimSuffix(strings.Repeat(cardGap+"\n", l.Card.Height), "\n")
	rowGap := strings.Repeat(" ", l.ContentWidth)
	rows := make([]string, 0, 2*l.RowsVisible-1)

	for rowStart, r := first, 0; rowStart < last; rowStart, r = rowStart+l.Columns, r+1 {
		if r > 0 {
			rows = append(rows, rowGap)
		}

		end := min(rowStart+l.Columns, last)
		blocks := make([]string, 0, 2*l.Columns-1)
		for i := rowStart; i < end; i++ {
			if i > rowStart {
				blocks = append(blocks, gapBlock)
			}
			blocks = append(blocks, ui.RenderCard(entries[i], l.Card, i == focus))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderStateLine centers a single status message in the grid area. The
// message is sanitized because error strings can embed filesystem paths.
func renderStateLine(l ui.Layout, message string) string {
	return lipgloss.Place(
		l.ContentWidth,
		max(l.ContentHeight, 1),
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().Faint(true).Render(ui.SanitizeName(message)),
	)
}
