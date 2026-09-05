package ui

import (
	"strconv"
	"strings"
	"time"

	"gridfm/internal/preview"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// RenderInspector draws the right-side metadata panel: identity, size,
// permissions, ownership, modification time, symlink target, and a bounded
// text preview. loading renders a placeholder; loadErr renders the failure.
func RenderInspector(width, height int, info *preview.Info, loadErr error, loading bool) string {
	if width < 1 {
		width = 1
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(CurrentTheme().Accent).
		Render(" inspector")

	lines := []string{title, ""}
	switch {
	case loadErr != nil:
		lines = append(lines, faint(TruncateName(SanitizeName(loadErr.Error()), width-3)))
	case loading || info == nil:
		lines = append(lines, faint("loading…"))
	default:
		lines = append(lines, detail(info, width)...)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		MaxHeight(max(height, 1)).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(CurrentTheme().Muted).
		Render(body)
}

func detail(info *preview.Info, width int) []string {
	fit := func(s string) string {
		return TruncateName(SanitizeName(s), width-3)
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(fit(info.Name)),
		faint(fit(info.Path)),
		"",
	}

	kind := "file"
	switch {
	case info.Symlink:
		kind = "symlink"
	case info.IsDir:
		kind = "directory"
	}
	lines = append(lines, faint("kind    ")+fit(kind))
	lines = append(lines, faint("size    ")+fit(FormatBytes(info.Size)+" ("+strconv.FormatInt(info.Size, 10)+" B)"))
	lines = append(lines, faint("mode    ")+fit(info.Mode))
	lines = append(lines, faint("owner   ")+fit(info.Owner+" : "+info.Group))
	lines = append(lines, faint("modified")+fit(" "+info.ModTime.Format(time.DateTime)))
	if info.ReadOnly {
		lines = append(lines, lipgloss.NewStyle().Foreground(CurrentTheme().Warning).Render(fit("read-only")))
	}
	if info.Symlink {
		target := fit("-> " + info.LinkTarget)
		if info.TargetMissing {
			target = lipgloss.NewStyle().Foreground(CurrentTheme().Warning).Render(target)
		}
		lines = append(lines, target)
	}

	if len(info.Preview) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Foreground(CurrentTheme().Accent).
			Render(" preview"))
		for _, line := range info.Preview {
			lines = append(lines, faint(fit(line)))
		}
		if info.PreviewTruncated {
			lines = append(lines, faint("… more"))
		}
	}

	return lines
}

func faint(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}

	return lipgloss.NewStyle().Faint(true).Render(s)
}

// RenderInspectorOverlay draws the inspector floating over the right edge
// of a grid frame: the panel lines replace the rightmost cells of each grid
// line. It is the narrow-terminal form of the panel, which cannot dock
// without starving the grid. Overlay lines are cut with ANSI awareness so
// styling survives.
func RenderInspectorOverlay(gridFrame string, width int, info *preview.Info, loadErr error, loading bool) string {
	if width < 1 {
		width = 1
	}

	panel := RenderInspector(width, strings.Count(gridFrame, "\n")+1, info, loadErr, loading)
	panelLines := strings.Split(panel, "\n")
	gridLines := strings.Split(gridFrame, "\n")

	out := make([]string, 0, len(gridLines))
	for i, gridLine := range gridLines {
		if i >= len(panelLines) {
			out = append(out, gridLine)

			continue
		}

		leftWidth := ansi.StringWidth(gridLine) - width
		if leftWidth < 0 {
			leftWidth = 0
		}
		out = append(out, ansiCut(gridLine, leftWidth)+panelLines[i])
	}

	return strings.Join(out, "\n")
}
