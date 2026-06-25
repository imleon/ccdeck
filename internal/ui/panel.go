package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Panel describes the shared visual shell used by all top-level panes.
type Panel struct {
	Title   string
	Body    string
	Footer  []string
	Focused bool
	Width   int
	Height  int
	NoWrap  bool
}

const (
	standaloneHeaderRows = 1
	standaloneBodyPadX   = 1
)

var standaloneScrollbarBodyPadding = panePadding{Left: standaloneBodyPadX, Right: 0}

type panePadding struct {
	Left  int
	Right int
}

type verticalScrollbarOptions struct {
	Width      int
	Track      string
	Thumb      string
	TrackStyle lipgloss.Style
	ThumbStyle lipgloss.Style
	AlignRight bool
}

func symmetricPanePadding(padX int) panePadding {
	padX = max(padX, 0)
	return panePadding{Left: padX, Right: padX}
}

func paneContentWidth(width int, padding panePadding) int {
	return max(width-max(padding.Left, 0)-max(padding.Right, 0), 1)
}

func standaloneContentSize(width, height int) (int, int) {
	return max(width, 1), max(height-standaloneHeaderRows, 1)
}

func standaloneInsetContentSize(width, height, padX int) (int, int) {
	return standalonePaddedContentSize(width, height, symmetricPanePadding(padX))
}

func standalonePaddedContentSize(width, height int, padding panePadding) (int, int) {
	return paneContentWidth(width, padding), max(height-standaloneHeaderRows, 1)
}

func renderStandalonePane(_ string, status, body string, width int) string {
	w := max(width, 1)
	if status == "" {
		return body
	}
	statusLine := statusStyle.Width(w).Render(padCell(truncateCell(status, w), w))
	return joinVertical(statusLine, body)
}

func renderStandalonePaneInset(_ string, status, body string, width, padX int) string {
	return renderStandalonePanePadded(status, body, width, symmetricPanePadding(padX))
}

func renderStandalonePanePadded(status, body string, width int, padding panePadding) string {
	w := max(width, 1)
	inset := lipgloss.NewStyle().Padding(0, max(padding.Right, 0), 0, max(padding.Left, 0)).Width(w)
	bodyBlock := inset.Render(fitBlock(body, paneContentWidth(w, padding), max(1, strings.Count(body, "\n")+1)))
	if status == "" {
		return bodyBlock
	}
	statusLine := statusStyle.Width(w).Render(padCell(truncateCell(status, w), w))
	return joinVertical(statusLine, bodyBlock)
}

func renderPanel(p Panel) string {
	contentWidth := panelContentWidth(p.Width)
	contentHeight := panelContentHeight(p.Height)

	titleText := padCell(truncateCell(defaultString(p.Title, "(none)"), contentWidth), contentWidth)
	title := panelTitleStyle.Render(titleText)

	footer := ""
	footerRows := 0
	if len(p.Footer) > 0 {
		footerText := padCell(formatFooter(p.Footer, contentWidth), contentWidth)
		footerStyle := panelFooterStyle
		if p.Focused {
			footerStyle = panelFooterActiveStyle
		}
		footer = footerStyle.Render(footerText)
		footerRows = 1
	}

	bodyHeight := max(contentHeight-1-footerRows, 1)
	body := fitBlock(p.Body, contentWidth, bodyHeight)

	parts := []string{title, body}
	if footer != "" {
		parts = append(parts, footer)
	}

	style := panelStyle(p.Focused).Height(p.Height)
	if !p.NoWrap {
		style = style.Width(p.Width)
	}
	return style.Render(joinVertical(parts...))
}

func panelContentWidth(panelWidth int) int {
	// panelStyle adds a 1-cell border and 1-cell horizontal padding on both sides.
	width := panelWidth - 4
	if width < 1 {
		return 1
	}
	return width
}

func panelContentHeight(panelHeight int) int {
	// panelStyle adds a 1-cell top and bottom border; vertical padding is 0.
	height := panelHeight - 2
	if height < 1 {
		return 1
	}
	return height
}

func defaultString(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func renderVerticalScrollbar(viewportHeight, totalHeight, topOffset int, opts verticalScrollbarOptions) []string {
	if viewportHeight <= 0 {
		return nil
	}
	width := max(opts.Width, 1)
	bars := make([]string, viewportHeight)
	for i := range bars {
		bars[i] = strings.Repeat(" ", width)
	}
	if totalHeight <= viewportHeight {
		return bars
	}

	track := defaultString(opts.Track, "│")
	thumb := defaultString(opts.Thumb, "┃")
	thumbHeight := min(max(1, viewportHeight*viewportHeight/totalHeight), viewportHeight)
	maxTop := max(totalHeight-viewportHeight, 1)
	thumbTop := max(topOffset, 0) * (viewportHeight - thumbHeight) / maxTop
	for i := range bars {
		glyph := track
		style := opts.TrackStyle
		if i >= thumbTop && i < thumbTop+thumbHeight {
			glyph = thumb
			style = opts.ThumbStyle
		}
		bars[i] = renderScrollbarCell(glyph, width, opts.AlignRight, style)
	}
	return bars
}

func renderScrollbarCell(glyph string, width int, alignRight bool, style lipgloss.Style) string {
	cell := truncateCell(glyph, width)
	pad := max(width-lipgloss.Width(cell), 0)
	if alignRight {
		cell = strings.Repeat(" ", pad) + cell
	} else {
		cell += strings.Repeat(" ", pad)
	}
	return style.Render(cell)
}

func truncateCell(s string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}

	var b strings.Builder
	for _, r := range s {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate)+1 > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

func formatFooter(parts []string, maxWidth int) string {
	if len(parts) == 0 || maxWidth <= 0 {
		return ""
	}

	joined := strings.Join(parts, " · ")
	if lipgloss.Width(joined) <= maxWidth {
		return joined
	}
	if len(parts) == 1 {
		return truncateMiddleCell(parts[0], maxWidth)
	}

	sep := " · "
	minPrimaryWidth := min(14, maxWidth)
	for suffixCount := len(parts) - 1; suffixCount >= 0; suffixCount-- {
		suffix := strings.Join(parts[1:1+suffixCount], sep)
		reservedWidth := 0
		if suffix != "" {
			reservedWidth = lipgloss.Width(sep) + lipgloss.Width(suffix)
		}
		primaryWidth := maxWidth - reservedWidth
		if primaryWidth <= 0 {
			continue
		}
		if suffixCount > 0 && primaryWidth < minPrimaryWidth {
			continue
		}
		primary := truncateMiddleCell(parts[0], primaryWidth)
		if suffix == "" {
			return primary
		}
		return primary + sep + suffix
	}

	return truncateMiddleCell(parts[0], maxWidth)
}

func truncateMiddleCell(s string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}

	leftWidth := (maxWidth - 1) / 3
	rightWidth := maxWidth - 1 - leftWidth

	left := takeCellPrefix(s, leftWidth)
	right := takeCellSuffix(s, rightWidth)
	return left + "…" + right
}

// truncatePrefixCell 截断前缀，保留尾部：超长时返回 "…" + 末尾片段，
// 适合需要看清路径末段目录名的场景。
func truncatePrefixCell(s string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	return "…" + takeCellSuffix(s, maxWidth-1)
}

func takeCellPrefix(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func takeCellSuffix(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := string(runes[i:])
		if lipgloss.Width(candidate) > maxWidth {
			break
		}
		start = i
	}
	return string(runes[start:])
}

func fitBlock(s string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = truncateCell(line, width)
	}
	return strings.Join(lines, "\n")
}
