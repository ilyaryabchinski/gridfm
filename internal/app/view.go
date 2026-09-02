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
	if l.SidebarOverlay {
		// The overlay panel keeps the full sidebar width even though no
		// grid columns are reserved for it.
		body = ui.RenderSidebarOverlay(body,
			min(ui.SidebarWidth, l.ContentWidth), m.sidebarItems(), m.placeIdx)
	}

	middle := body
	if l.SidebarVisible {
		sidebar := ui.RenderSidebar(
			l.SidebarWidth,
			l.ContentHeight,
			m.sidebarItems(),
			m.placeIdx,
			m.region == RegionSidebar,
		)
		middle = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	}

	if m.sortOpen {
		middle = m.renderSortMenu(l)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ui.RenderBreadcrumbs(m.width, m.browser.Path, m.home, m.browser.CanBack(), m.browser.CanForward()),
		middle,
		ui.RenderStatusBar(m.width, m.statusInfo(l)),
	)
}

func (m *Model) layout() ui.Layout {
	return ui.ComputeLayout(m.width, m.height, m.zoom, m.sidebarOn)
}

func (m *Model) sidebarItems() []ui.SidebarItem {
	items := make([]ui.SidebarItem, len(m.places))
	for i, p := range m.places {
		items[i] = ui.SidebarItem{Label: p.Label, Path: p.Path}
	}

	return items
}

// statusInfo assembles the status bar contents for the current state.
func (m *Model) statusInfo(l ui.Layout) ui.StatusInfo {
	mode := "NORMAL"
	if m.region == RegionSidebar {
		mode = "PLACES"
	}
	if m.filterInput {
		mode = "FILTER"
	}
	if m.sortOpen {
		mode = "SORT"
	}

	filter := m.browser.Filter()
	if m.filterInput {
		filter += "▌"
	}

	sort := m.browser.SortMode().String() + " " + m.browser.SortOrder().String()
	if l.Zoom != m.zoom {
		sort += " (compact)"
	}

	visible := len(m.browser.Visible())

	return ui.StatusInfo{
		Mode:       mode,
		Sort:       sort,
		HiddenOn:   m.browser.ShowHidden(),
		Filter:     filter,
		Loading:    m.loading,
		Note:       m.note,
		Items:      visible,
		TotalItems: len(m.browser.Entries),
	}
}

// renderBody draws the grid viewport or the full-body state message.
func (m *Model) renderBody(l ui.Layout) string {
	var body string
	visible := len(m.browser.Visible())

	switch {
	case m.loadErr != nil && len(m.browser.Entries) == 0:
		body = renderStateLine(l, "error: "+m.loadErr.Error())
	case m.loading && len(m.browser.Entries) == 0:
		body = renderStateLine(l, "loading…")
	case len(m.browser.Entries) == 0:
		body = renderStateLine(l, "empty directory")
	case visible == 0:
		body = renderStateLine(l, "no matches")
	default:
		body = m.renderViewport(l)
	}

	return lipgloss.NewStyle().
		Height(l.ContentHeight).
		MaxHeight(l.ContentHeight).
		Width(l.GridWidth).
		MaxWidth(l.GridWidth).
		Render(body)
}

// renderViewport renders only the visible cards: rows scrollRow through
// scrollRow+RowsVisible of the grid. Card gaps are rendered as literal
// spacer cells so the visible geometry matches the layout calculations.
func (m *Model) renderViewport(l ui.Layout) string {
	entries := m.browser.Visible()
	focus := m.browser.FocusIndex()

	first := m.scrollRow * l.Columns
	last := min(first+l.Columns*l.RowsVisible, len(entries))

	cardGap := strings.Repeat(" ", ui.CardGapX)
	// A spacer block of full card height so JoinHorizontal keeps every card
	// on the same baseline.
	gapBlock := strings.TrimSuffix(strings.Repeat(cardGap+"\n", l.Card.Height), "\n")
	rowGap := strings.Repeat(" ", l.GridWidth)
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
			blocks = append(blocks, ui.RenderCard(entries[i], l.Zoom, i == focus, m.icons))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderSortMenu centers the blocking sort menu over the interface.
func (m *Model) renderSortMenu(l ui.Layout) string {
	active := m.browser.SortMode()
	order := m.browser.SortOrder()

	lines := make([]string, 0, len(sortModes)+2)
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(" sort "))
	lines = append(lines, "")

	for i, mode := range sortModes {
		marker := "  "
		if m.sortCursor == i {
			marker = "> "
		}

		suffix := ""
		if mode == active {
			suffix = "  " + order.String()
		}
		lines = append(lines, marker+mode.String()+suffix)
	}

	lines = append(lines, "",
		lipgloss.NewStyle().Faint(true).Render(" enter apply · o reverse · esc close"))

	menu := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(l.ContentWidth, max(l.ContentHeight, 1), lipgloss.Center, lipgloss.Center, menu)
}

// renderStateLine centers a single status message in the grid area. The
// message is sanitized because error strings can embed filesystem paths.
func renderStateLine(l ui.Layout, message string) string {
	return lipgloss.Place(
		l.GridWidth,
		max(l.ContentHeight, 1),
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().Faint(true).Render(ui.SanitizeName(message)),
	)
}
