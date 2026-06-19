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
}

func renderPanel(p Panel) string {
	contentWidth := panelContentWidth(p.Width)
	contentHeight := panelContentHeight(p.Height)

	title := panelTitleStyle.Render(truncateCell(defaultString(p.Title, "(none)"), contentWidth))

	footer := ""
	footerRows := 0
	if len(p.Footer) > 0 {
		footerText := truncateCell(strings.Join(p.Footer, " · "), contentWidth)
		footerStyle := panelFooterStyle
		if p.Focused {
			footerStyle = panelFooterActiveStyle
		}
		footer = footerStyle.Render(footerText)
		footerRows = 1
	}

	bodyHeight := contentHeight - 1 - footerRows
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := fitBlock(p.Body, contentWidth, bodyHeight)

	parts := []string{title, body}
	if footer != "" {
		parts = append(parts, footer)
	}

	return panelStyle(p.Focused).
		Width(p.Width).
		Height(p.Height).
		Render(joinVertical(parts...))
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
