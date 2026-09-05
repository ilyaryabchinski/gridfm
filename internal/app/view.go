package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gridfm/internal/browser"
	"gridfm/internal/places"
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
		// Anything on screen must go when the terminal collapses.
		m.syncImages(l)

		return ui.RenderTooSmall(m.width, m.height)
	}

	// Thumbnail reconciliation rides on the render pass: it is a pure
	// channel send computing exactly what this frame shows.
	m.syncImages(l)

	body := m.renderBody(l, m.bodyHeight(l))
	if l.SidebarOverlay {
		// The overlay panel keeps the full sidebar width even though no
		// grid columns are reserved for it.
		body = ui.RenderSidebarOverlay(body,
			min(ui.SidebarWidth, l.ContentWidth), m.sidebarItems(), m.placeIdx, m.operationLines())
	}
	if l.InspectorWidth == 0 && m.inspectorOn {
		// Narrow terminals cannot dock the inspector without starving the
		// grid; it floats over the right edge instead, like the sidebar.
		body = ui.RenderInspectorOverlay(body,
			min(ui.InspectorWidth, l.ContentWidth), m.inspector, m.inspectorErr,
			m.inspector == nil && m.inspectorErr == nil)
	}

	middle := body
	if l.SidebarVisible {
		sidebar := ui.RenderSidebar(
			l.SidebarWidth,
			l.ContentHeight,
			m.sidebarItems(),
			m.placeIdx,
			m.region == RegionSidebar,
			m.operationLines(),
		)
		middle = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	}

	if l.InspectorWidth > 0 {
		loading := m.inspector == nil && m.inspectorErr == nil
		panel := ui.RenderInspector(l.InspectorWidth, l.ContentHeight, m.inspector, m.inspectorErr, loading)
		middle = lipgloss.JoinHorizontal(lipgloss.Top, middle, panel)
	}

	middle = m.withOverlay(l, middle)

	rows := []string{
		ui.RenderBreadcrumbs(m.width, m.browser.Path, m.home, m.browser.CanBack(), m.browser.CanForward()),
		middle,
	}

	// The operation shelf is non-blocking: it takes one row when a job is
	// running so the grid can keep rendering around it.
	if m.opProgress != nil {
		rows = append(rows, m.renderShelf(l))
	}

	rows = append(rows, ui.RenderStatusBar(m.width, m.statusInfo(l)))

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m *Model) layout() ui.Layout {
	return ui.ComputeLayoutWithInspector(m.width, m.height, m.zoom, m.sidebarOn, m.inspectorOn)
}

// bodyHeight returns the cells available to the grid: one less while the
// operation shelf row is showing.
func (m *Model) bodyHeight(l ui.Layout) int {
	if m.opProgress != nil {
		return max(l.ContentHeight-1, 1)
	}

	return l.ContentHeight
}

// sidebarItems composes every sidebar section into one flat list: places,
// bookmarks, mounts, and recent locations.
func (m *Model) sidebarItems() []ui.SidebarItem {
	section := func(src []places.Place, name string) []ui.SidebarItem {
		items := make([]ui.SidebarItem, 0, len(src))
		for _, p := range src {
			items = append(items, ui.SidebarItem{Label: p.Label, Path: p.Path, Section: name})
		}

		return items
	}

	items := section(m.places, "places")
	items = append(items, section(m.bookmarks, "bookmarks")...)
	items = append(items, section(m.mounts, "mounts")...)
	items = append(items, section(m.recents, "recents")...)

	return items
}

// withOverlay replaces the interface body with the active blocking
// overlay, if any. Only one blocking overlay is active at a time.
func (m *Model) withOverlay(l ui.Layout, body string) string {
	switch {
	case m.input != inputNone:
		return m.renderInputOverlay(l)
	case m.question != nil:
		return m.renderConflictOverlay(l)
	case m.confirm != confirmNone:
		return m.renderConfirmOverlay(l)
	case m.sortOpen:
		return m.renderSortMenu(l)
	case m.showResults && m.lastResult != nil:
		return m.renderResultsOverlay(l)
	case m.helpOpen:
		return m.renderHelpOverlay(l)
	}

	return body
}

// binding is one row of the keyboard legend: the key (or key pair) and
// what it does.
type binding struct {
	key    string
	action string
}

// helpColumn renders one titled column of the keyboard legend: rows with a
// fixed-width key field.
func helpColumn(title string, rows []binding) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(ui.CurrentTheme().Accent).
		Render(" " + title)
	lines := make([]string, 0, 2+len(rows))
	lines = append(lines, header)
	for _, row := range rows {
		lines = append(lines,
			lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf(" %-13s", row.key))+row.action)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderHelpOverlay centers the keyboard legend: the application's
// everyday bindings, in two columns so it fits a normal terminal. Keys
// render from the bind table so remaps are always accurate.
func (m *Model) renderHelpOverlay(l ui.Layout) string {
	navigate := helpColumn("navigate", []binding{
		{"arrows h j k l", "move"},
		{"enter / " + m.KeyFor("open"), "open"},
		{"backspace / h", "parent (at edge)"},
		{"alt+left/right", "history"},
		{"pgup / pgdn", "page"},
		{"home / end", "first/last"},
		{"tab", "focus pane"},
		{m.KeyFor("sidebar"), "sidebar"},
	})
	view := helpColumn("view", []binding{
		{m.KeyFor("filter"), "filter"},
		{m.KeyFor("hidden"), "hidden files"},
		{m.KeyFor("sort"), "sort menu"},
		{m.KeyFor("zoom_in") + " / " + m.KeyFor("zoom_out"), "card size"},
		{m.KeyFor("inspector"), "inspector"},
		{m.KeyFor("refresh"), "refresh"},
	})
	act := helpColumn("select + files", []binding{
		{"space", "toggle select"},
		{"v", "range mode"},
		{"ctrl+a", "select visible"},
		{"y / x", "stage copy/move"},
		{"p", "paste"},
		{"n", "new (ctrl+d: dir)"},
		{"R", "rename"},
		{"d / D", "trash / delete"},
		{"b / B", "bookmark add/del"},
	})
	jobs := helpColumn("operations", []binding{
		{"c", "cancel job"},
		{"ctrl+c", "quit"},
		{"e", "last result"},
		{"esc", "clear / close"},
		{m.KeyFor("help"), "this legend"},
		{m.KeyFor(actionQuit), actionQuit},
	})

	left := lipgloss.JoinVertical(lipgloss.Left, navigate, "", view)
	right := lipgloss.JoinVertical(lipgloss.Left, act, "", jobs)

	return placeOverlayBox(l, []string{
		lipgloss.NewStyle().Bold(true).Render(" keys · esc closes"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right),
	})
}

// statusInfo assembles the status bar contents for the current state.
func (m *Model) statusInfo(l ui.Layout) ui.StatusInfo {
	visible := len(m.browser.Visible())

	return ui.StatusInfo{
		Mode:       m.modeLabel(),
		Sort:       m.sortDisplay(l),
		HiddenOn:   m.browser.ShowHidden(),
		Filter:     m.filterDisplay(),
		Loading:    m.loading,
		Note:       m.note,
		Items:      visible,
		TotalItems: len(m.browser.Entries),
	}
}

// modeLabel names the surface that owns the keyboard.
func (m *Model) modeLabel() string {
	switch {
	case m.input != inputNone:
		return inputTitle(m.input)
	case m.question != nil:
		return "CONFLICT"
	case m.confirm != confirmNone:
		return "CONFIRM"
	case m.sortOpen:
		return "SORT"
	case m.filterInput:
		return "FILTER"
	case m.region == RegionSidebar:
		return "PLACES"
	}

	return m.mode.String()
}

// sortDisplay renders sort state plus staging and selection indicators.
func (m *Model) sortDisplay(l ui.Layout) string {
	sort := m.browser.SortMode().String() + " " + m.browser.SortOrder().String()
	if l.Zoom != m.zoom {
		sort += " (compact)"
	}
	if m.clipboard != ClipboardNone {
		sort += " · " + strconv.Itoa(len(m.clipboardPath)) + " staged " + m.clipboard.String()
	}
	if selected := m.browser.SelectedCount(); selected > 0 {
		sort += " · " + strconv.Itoa(selected) + " selected"
	}

	return sort
}

// filterDisplay renders the active query with its editing cursors.
func (m *Model) filterDisplay() string {
	filter := m.browser.Filter()
	if m.filterInput {
		filter += "▌"
	}
	if m.input != inputNone {
		filter += m.inputValue + "▌"
	}
	if m.confirmTyped {
		filter = "type yes: " + m.confirmInput + "▌"
	}

	return filter
}

// inputTitle names the active input overlay.
func inputTitle(kind inputKind) string {
	switch kind {
	case inputCreateDir:
		return "NEW DIR"
	case inputRename:
		return "RENAME"
	case inputCreateFile, inputNone:
		return "NEW FILE"
	}

	return "NEW FILE"
}

// renderBody draws the grid viewport or the full-body state message.
func (m *Model) renderBody(l ui.Layout, bodyHeight int) string {
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
		Height(bodyHeight).
		MaxHeight(bodyHeight).
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
			blocks = append(blocks, ui.RenderCard(entries[i], l.Zoom, m.cardState(entries[i], i == focus), m.icons))
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

	return placeOverlayBox(l, lines)
}

// cardState assembles the visual state of one card: focus, selection, the
// dimmed marker for entries staged for a move, and the image reservation
// when a ready thumbnail will cover the label rows.
func (m *Model) cardState(entry browser.Entry, focused bool) ui.CardState {
	dimmed := m.clipboard == ClipboardMove && slices.Contains(m.clipboardPath, entry.Path)

	return ui.CardState{
		Focused:  focused,
		Selected: m.browser.Selection().Has(entry.Path),
		Dimmed:   dimmed,
		Image:    m.hasThumb(entry),
	}
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

// renderShelf draws the non-blocking operation progress line. It borrows
// one row from the grid area while a job runs.
func (m *Model) renderShelf(l ui.Layout) string {
	p := m.opProgress
	line := fmt.Sprintf(" %s %d/%d %s", p.kind, p.done, p.total, ui.SanitizeName(p.target))
	if p.bytesTotal > 0 {
		line += fmt.Sprintf(" · %s/%s", ui.FormatBytes(p.bytes), ui.FormatBytes(p.bytesTotal))
	}
	line += "  (c cancels)"

	return lipgloss.NewStyle().
		Foreground(ui.CurrentTheme().Info).
		Width(l.ContentWidth).
		MaxWidth(l.ContentWidth).
		Render(ui.TruncateName(line, l.ContentWidth))
}

// renderInputOverlay centers the create/rename input.
func (m *Model) renderInputOverlay(l ui.Layout) string {
	title := inputTitle(m.input)
	hint := "enter apply · ctrl+d switch · esc cancel"
	if m.input == inputRename {
		hint = "enter apply · esc cancel"
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(" " + title + " "),
		// The value is sanitized for display only; submission keeps the
		// raw name, so renaming never silently rewrites a hostile name.
		inputBox(ui.SanitizeName(m.inputValue)),
		lipgloss.NewStyle().Faint(true).Render(hint),
	}

	return placeOverlayBox(l, lines)
}

// renderConfirmOverlay centers a confirmation dialog.
func (m *Model) renderConfirmOverlay(l ui.Layout) string {
	var title string
	switch m.confirm {
	case confirmTrash:
		title = " MOVE TO TRASH "
	case confirmDelete:
		title = " DELETE PERMANENTLY "
	case confirmQuit:
		title = " QUIT? "
	case confirmNone:
		title = " CONFIRM "
	}

	lines := []string{lipgloss.NewStyle().Bold(true).Foreground(ui.CurrentTheme().Warning).Render(title)}
	for _, detail := range m.confirmDetail {
		lines = append(lines, ui.TruncateName(ui.SanitizeName(detail), 56))
	}

	if m.confirmTyped {
		lines = append(lines, "",
			inputBox("yes: "+m.confirmInput),
			lipgloss.NewStyle().Faint(true).Render("type yes to confirm · esc cancels"))
	} else {
		lines = append(lines, "",
			lipgloss.NewStyle().Faint(true).Render("y confirm · n/esc cancel"))
	}

	return placeOverlayBox(l, lines)
}

// renderConflictOverlay centers the conflict question for the running job.
func (m *Model) renderConflictOverlay(l ui.Layout) string {
	lines := make([]string, 0, 6)
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Foreground(ui.CurrentTheme().Warning).Render(" TARGET EXISTS "),
		ui.TruncateName(ui.SanitizeName(m.question.target), 56),
		"",
		"s skip · r replace · n rename as copy",
	)
	state := "off"
	if m.applyAll {
		state = "ON for the rest of this job"
	}
	lines = append(lines,
		lipgloss.NewStyle().Faint(true).Render("a apply to all: "+state),
		lipgloss.NewStyle().Faint(true).Render("esc aborts the operation"))

	return placeOverlayBox(l, lines)
}

// renderResultsOverlay lists what a finished operation failed on.
func (m *Model) renderResultsOverlay(l ui.Layout) string {
	result := m.lastResult
	lines := make([]string, 0, len(result.Failures)+4)
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Render(" "+resultSummary(*result)+" "),
		"",
	)

	if len(result.Failures) == 0 {
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("no failures"))
	}
	for _, failure := range result.Failures {
		line := ui.SanitizeName(failure.Err.Error())
		lines = append(lines, ui.TruncateName(line, 64))
	}

	lines = append(lines, "", lipgloss.NewStyle().Faint(true).Render("e/esc close"))

	return placeOverlayBox(l, lines)
}

// inputBox renders a one-line text field with a block cursor.
func inputBox(value string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ui.CurrentTheme().Muted).
		Padding(0, 1).
		Render(value + "▌")
}

// placeOverlayBox centers a bordered dialog in the grid area.
func placeOverlayBox(l ui.Layout, lines []string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.CurrentTheme().Muted).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(l.ContentWidth, max(l.ContentHeight, 1), lipgloss.Center, lipgloss.Center, box)
}
