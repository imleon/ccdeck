package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	focusedBorderColor = lipgloss.Color("63")
	normalBorderColor  = lipgloss.Color("240")
)

func panelStyle(focused bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)
	if focused {
		return s.BorderForeground(focusedBorderColor)
	}
	return s.BorderForeground(normalBorderColor)
}

var statusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("245")).
	Padding(0, 1)

var panelTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("75"))

var panelFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244"))

var panelFooterActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117"))

var helpBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(1, 2).
	BorderForeground(focusedBorderColor)

var treeCursorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("229")).
	Background(lipgloss.Color("63")).
	Inline(true)

var treeOpenedFileStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117")).
	Bold(true).
	Inline(true)

var lineNumberStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244"))

var warningStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("214"))

var errorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("203"))

// joinHorizontal horizontally joins already-rendered blocks. Lip Gloss v2 no
// longer exposes the old top-level JoinHorizontal helper, and this simple
// fixed-panel joiner is enough for our three-column layout.
func joinHorizontal(blocks ...string) string {
	rows := make([][]string, len(blocks))
	widths := make([]int, len(blocks))
	maxLines := 0

	for i, b := range blocks {
		rows[i] = strings.Split(strings.TrimSuffix(b, "\n"), "\n")
		if len(rows[i]) > maxLines {
			maxLines = len(rows[i])
		}
		for _, line := range rows[i] {
			if w := lipgloss.Width(line); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var out strings.Builder
	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		for blockIdx := range blocks {
			line := ""
			if lineIdx < len(rows[blockIdx]) {
				line = rows[blockIdx][lineIdx]
			}
			if pad := widths[blockIdx] - lipgloss.Width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			out.WriteString(line)
		}
		if lineIdx != maxLines-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func joinVertical(blocks ...string) string {
	return strings.Join(blocks, "\n")
}

func padCell(s string, width int) string {
	if width <= 0 {
		return s
	}
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
