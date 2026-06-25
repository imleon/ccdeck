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

var sessionGroupHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("180"))

var sessionSelectedBGColor = lipgloss.Color("#4a3a32")
var sessionActiveBGColor = lipgloss.Color("#2a2421")

var sessionSelectedGroupHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("254")).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionActiveGroupHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("180"))

var sessionTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("252"))

var sessionSelectedTitleColor = lipgloss.Color("254")

var sessionSelectedTitleStyle = lipgloss.NewStyle().
	Foreground(sessionSelectedTitleColor).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionActiveTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("180")).
	Background(sessionActiveBGColor).
	Inline(true)

var sessionAgeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

var sessionSelectedAgeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("250")).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionActiveAgeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("250")).
	Background(sessionActiveBGColor).
	Inline(true)

var sessionActiveFillStyle = lipgloss.NewStyle().
	Background(sessionActiveBGColor).
	Inline(true)

var sessionNormalFillStyle = lipgloss.NewStyle().Inline(true)

var sessionSelectedFillStyle = lipgloss.NewStyle().
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionSelectedTrailingFillStyle = lipgloss.NewStyle().
	Foreground(sessionSelectedBGColor).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionActiveTrailingFillStyle = lipgloss.NewStyle().
	Foreground(sessionActiveBGColor).
	Background(sessionActiveBGColor).
	Inline(true)

var sessionDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244")).
	Inline(true)

var sessionSelectedActiveGuideStyle = lipgloss.NewStyle().
	Foreground(sessionSelectedTitleColor).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionSelectedDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244")).
	Background(sessionSelectedBGColor).
	Inline(true)

var sessionActiveGuideStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("180")).
	Background(sessionActiveBGColor).
	Inline(true)

var sessionActiveDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244")).
	Background(sessionActiveBGColor).
	Inline(true)

var sessionFilterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117")).
	Inline(true)

var sessionScrollbarTrackStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("238")).
	Inline(true)

var sessionScrollbarThumbStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("255")).
	Inline(true)

var helpBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(1, 2).
	BorderForeground(focusedBorderColor)

var treeCursorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("254")).
	Background(sessionSelectedBGColor).
	Inline(true)

var treeOpenedFileStyle = lipgloss.NewStyle().
	Background(sessionActiveBGColor).
	Inline(true)

var treeOpenedFileGuideStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("180")).
	Background(sessionActiveBGColor).
	Inline(true)

var treeSelectedOpenedFileGuideStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("255")).
	Background(sessionSelectedBGColor).
	Inline(true)

var treeGitModifiedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("178")).
	Inline(true)

var treeGitAddedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("70")).
	Inline(true)

var treeGitDeletedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("167")).
	Inline(true)

var treeGitUntrackedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("71")).
	Inline(true)

var treeGitRenamedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("81")).
	Inline(true)

var treeGitConflictStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("205")).
	Inline(true)

var lineNumberStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("244"))

var (
	viewerDiffAddedBG   = lipgloss.Color("#022800")
	viewerDiffDeletedBG = lipgloss.Color("#3d0100")
)

var viewerDiffAddedStyle = lipgloss.NewStyle().
	Background(viewerDiffAddedBG).
	Inline(true)

var viewerDiffDeletedStyle = lipgloss.NewStyle().
	Background(viewerDiffDeletedBG).
	Inline(true)

var viewerDiffAddedGutterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("80")).
	Background(viewerDiffAddedBG).
	Inline(true)

var viewerDiffDeletedGutterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("210")).
	Background(viewerDiffDeletedBG).
	Inline(true)

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
