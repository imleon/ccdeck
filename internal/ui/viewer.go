package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"

	"ccdeck/internal/gitstatus"
)

const maxPreviewBytes = 512 * 1024

type viewerState string

const (
	viewerIdle      viewerState = "idle"
	viewerLoaded    viewerState = "loaded"
	viewerEmpty     viewerState = "empty"
	viewerReadError viewerState = "read_error"
	viewerBinary    viewerState = "binary"
	viewerTooLarge  viewerState = "too_large"
)

// ViewerModel renders the selected file in a scrollable, syntax-highlighted viewport.
type ViewerModel struct {
	vp        viewport.Model
	path      string
	err       error
	ready     bool
	state     viewerState
	message   string
	lineCount int
	diffKinds []gitstatus.DiffLineKind

	requestID uint64
	loading   bool
}

func NewViewer() ViewerModel {
	vp := viewport.New()
	vp.SoftWrap = false
	return ViewerModel{vp: vp, state: viewerIdle}
}

// Path returns the currently loaded file path for titles/status display.
func (m ViewerModel) Path() string {
	return m.path
}

// Status returns a short human-readable state for status lines.
func (m ViewerModel) Status() string {
	return m.message
}

func (m ViewerModel) SoftWrap() bool {
	return m.vp.SoftWrap
}

func (m ViewerModel) WrapStatus() string {
	if m.vp.SoftWrap {
		return "wrap: panel"
	}
	return "wrap: off"
}

func (m ViewerModel) SetSize(w, h int) ViewerModel {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	m.vp.SetWidth(w)
	m.vp.SetHeight(h)
	return m
}

type loadFileMsg struct {
	requestID      uint64
	path           string
	content        string
	err            error
	state          viewerState
	message        string
	lineCount      int
	diffKinds      []gitstatus.DiffLineKind
	preserveScroll bool // true 表示自动刷新：渲染后保留原滚动位置，不回到顶部
}

// LoadFile 读取并渲染文件，渲染后滚动到顶部。用于用户主动选中文件。
func (m ViewerModel) LoadFile(path string) (ViewerModel, tea.Cmd) {
	m.path = path
	m.ready = false
	m.loading = true
	m.requestID++
	return m, loadFileCmd(path, m.requestID, false)
}

// Refresh 重新读取当前文件并保留滚动位置。用于自动刷新轮询；无文件时返回 nil。
func (m ViewerModel) Refresh() (ViewerModel, tea.Cmd) {
	if m.path == "" || m.loading {
		return m, nil
	}
	m.loading = true
	m.requestID++
	return m, loadFileCmd(m.path, m.requestID, true)
}

func loadFileCmd(path string, requestID uint64, preserveScroll bool) tea.Cmd {
	return func() tea.Msg {
		return renderFileMsg(path, requestID, preserveScroll)
	}
}

func renderFileMsg(path string, requestID uint64, preserveScroll bool) loadFileMsg {
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		if len(data) == 0 {
			message := "空文件"
			return loadFileMsg{requestID: requestID, path: path, state: viewerEmpty, message: message, content: warningStyle.Render(message), preserveScroll: preserveScroll}
		}
		if bytes.Contains(data, []byte{0}) {
			message := "二进制文件，未预览"
			return loadFileMsg{requestID: requestID, path: path, state: viewerBinary, message: message, content: warningStyle.Render(message), preserveScroll: preserveScroll}
		}
		if len(data) > maxPreviewBytes {
			message := fmt.Sprintf("文件超过 %d KiB，未高亮预览", maxPreviewBytes/1024)
			return loadFileMsg{requestID: requestID, path: path, state: viewerTooLarge, message: message, content: warningStyle.Render(message), preserveScroll: preserveScroll}
		}

		if diff, err := gitstatus.InlineDiff(path); err == nil && diff.HasDiff {
			content := renderInlineDiff(diff.Lines)
			lineCount := len(diff.Lines)
			return loadFileMsg{requestID: requestID, path: path, content: content, state: viewerLoaded, lineCount: lineCount, diffKinds: diffLineKinds(diff.Lines), message: fmt.Sprintf("%d lines · git changes", lineCount), preserveScroll: preserveScroll}
		}

		content := string(data)
		lineCount := countLines(content)
		rendered := highlightContent(path, content)
		return loadFileMsg{requestID: requestID, path: path, content: rendered, state: viewerLoaded, lineCount: lineCount, message: fmt.Sprintf("%d lines", lineCount), preserveScroll: preserveScroll}
	}

	if diff, err := gitstatus.InlineDiff(path); err == nil && diff.HasDiff {
		content := renderInlineDiff(diff.Lines)
		lineCount := len(diff.Lines)
		return loadFileMsg{requestID: requestID, path: path, content: content, state: viewerLoaded, lineCount: lineCount, diffKinds: diffLineKinds(diff.Lines), message: fmt.Sprintf("%d lines · git changes", lineCount), preserveScroll: preserveScroll}
	}

	message := "无法读取: " + readErr.Error()
	return loadFileMsg{requestID: requestID, path: path, err: readErr, state: viewerReadError, message: message, content: errorStyle.Render(message), preserveScroll: preserveScroll}
}

func renderInlineDiff(lines []gitstatus.DiffLine) string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		switch line.Kind {
		case gitstatus.DiffLineAdded, gitstatus.DiffLineDeleted:
			rendered = append(rendered, line.Text)
		default:
			rendered = append(rendered, line.Text)
		}
	}
	return strings.Join(rendered, "\n")
}

func diffLineKinds(lines []gitstatus.DiffLine) []gitstatus.DiffLineKind {
	kinds := make([]gitstatus.DiffLineKind, len(lines))
	for i, line := range lines {
		kinds[i] = line.Kind
	}
	return kinds
}

func highlightContent(path, content string) string {
	var buf bytes.Buffer
	lexer := strings.TrimPrefix(filepath.Ext(path), ".")
	if lexer == "" {
		lexer = "text"
	}
	if err := quick.Highlight(&buf, content, lexer, "terminal16m", "monokai"); err != nil {
		return content
	}
	return buf.String()
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	lines := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		lines++
	}
	return lines
}

func lineNumberGutter(lineCount int, diffKinds []gitstatus.DiffLineKind) viewport.GutterFunc {
	width := len(fmt.Sprintf("%d", lineCount))
	return func(info viewport.GutterContext) string {
		kind := viewerDiffKind(diffKinds, info.Index)
		marker := diffMarker(kind)
		if info.Soft || info.Index >= info.TotalLines {
			marker = " "
			gutter := strings.Repeat(" ", width) + marker + "│ "
			if _, ok := viewerDiffLineStyle(kind); ok {
				return gutter
			}
			return lineNumberStyle.Render(gutter)
		}
		gutter := fmt.Sprintf("%*d%s│ ", width, info.Index+1, marker)
		if _, ok := viewerDiffLineStyle(kind); ok {
			return gutter
		}
		return lineNumberStyle.Render(gutter)
	}
}

func viewerDiffKind(kinds []gitstatus.DiffLineKind, index int) gitstatus.DiffLineKind {
	if index < 0 || index >= len(kinds) {
		return gitstatus.DiffLineContext
	}
	return kinds[index]
}

func diffMarker(kind gitstatus.DiffLineKind) string {
	switch kind {
	case gitstatus.DiffLineAdded:
		return "+"
	case gitstatus.DiffLineDeleted:
		return "-"
	default:
		return " "
	}
}

func viewerDiffLineStyle(kind gitstatus.DiffLineKind) (lipgloss.Style, bool) {
	switch kind {
	case gitstatus.DiffLineAdded:
		return viewerDiffAddedStyle, true
	case gitstatus.DiffLineDeleted:
		return viewerDiffDeletedStyle, true
	default:
		return lipgloss.Style{}, false
	}
}

func renderViewerBodyLine(line string, bodyWidth int, kind gitstatus.DiffLineKind) string {
	line = padCell(truncateCell(line, bodyWidth), bodyWidth)
	if style, ok := viewerDiffLineStyle(kind); ok {
		return style.Render(line)
	}
	return line
}

func (m ViewerModel) Update(msg tea.Msg) (ViewerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadFileMsg:
		if msg.requestID != m.requestID || msg.path != m.path {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		m.ready = true
		m.state = msg.state
		m.message = msg.message
		m.lineCount = msg.lineCount
		m.diffKinds = msg.diffKinds
		if msg.lineCount > 0 {
			m.vp.LeftGutterFunc = lineNumberGutter(msg.lineCount, msg.diffKinds)
		} else {
			m.vp.LeftGutterFunc = nil
		}
		if msg.preserveScroll {
			// 自动刷新：保留原滚动位置。内容变短时 SetYOffset 内部会自行裁剪。
			yoff := m.vp.YOffset()
			m.vp.SetContent(msg.content)
			m.vp.SetYOffset(yoff)
		} else {
			m.vp.SetContent(msg.content)
			m.vp.GotoTop()
		}
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "w" {
			if !m.vp.SoftWrap {
				m.vp.SetXOffset(0)
			}
			m.vp.SoftWrap = !m.vp.SoftWrap
			return m, nil
		}

		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m ViewerModel) View() string {
	if !m.ready {
		return "  (选中文件查看内容)"
	}
	if m.vp.SoftWrap {
		return m.softWrapView()
	}
	return m.nowrapView()
}

func (m ViewerModel) nowrapView() string {
	width := m.vp.Width()
	height := m.vp.Height()
	if width <= 0 || height <= 0 {
		return ""
	}

	scrollbarWidth := 2
	bodyWidth := max(width-scrollbarWidth, 1)
	lines := viewerContentLines(m.vp.GetContent())
	start := min(m.vp.YOffset(), len(lines))
	end := min(start+height, len(lines))
	visible := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		kind := viewerDiffKind(m.diffKinds, i)
		gutter := m.viewerGutter(i, len(lines), false)
		contentWidth := max(bodyWidth-ansi.StringWidth(gutter), 0)
		visible = append(visible, renderViewerBodyLine(gutter+ansi.Cut(lines[i], m.vp.XOffset(), m.vp.XOffset()+contentWidth), bodyWidth, kind))
	}
	return joinWithViewerScrollbar(visible, height, len(lines), m.vp.YOffset(), scrollbarWidth)
}

func (m ViewerModel) softWrapView() string {
	width := m.vp.Width()
	height := m.vp.Height()
	if width <= 0 || height <= 0 {
		return ""
	}

	scrollbarWidth := 2
	bodyWidth := max(width-scrollbarWidth, 1)
	lines := viewerContentLines(m.vp.GetContent())
	visible := make([]string, 0, height)
	skip := m.vp.YOffset()
	for i, line := range lines {
		firstGutter := m.viewerGutter(i, len(lines), false)
		segmentWidth := m.softWrapSegmentWidth(bodyWidth, firstGutter)
		segmentCount := softWrapSegmentCount(line, segmentWidth)

		for segment := range segmentCount {
			if skip > 0 {
				skip--
				continue
			}
			soft := segment > 0
			gutter := firstGutter
			if soft {
				gutter = m.viewerGutter(i, len(lines), true)
			}
			kind := viewerDiffKind(m.diffKinds, i)
			contentWidth := max(bodyWidth-ansi.StringWidth(gutter), 0)
			start := segment * segmentWidth
			visible = append(visible, renderViewerBodyLine(gutter+ansi.Cut(line, start, start+contentWidth), bodyWidth, kind))
			if len(visible) >= height {
				return joinWithViewerScrollbar(visible, height, m.softWrapTotalHeight(lines, bodyWidth), m.vp.YOffset(), scrollbarWidth)
			}
		}
	}
	return joinWithViewerScrollbar(visible, height, m.softWrapTotalHeight(lines, bodyWidth), m.vp.YOffset(), scrollbarWidth)
}

func (m ViewerModel) softWrapSegmentWidth(bodyWidth int, gutter string) int {
	return max(bodyWidth-ansi.StringWidth(gutter), 1)
}

func softWrapSegmentCount(line string, segmentWidth int) int {
	return max((ansi.StringWidth(line)+segmentWidth-1)/segmentWidth, 1)
}

func (m ViewerModel) softWrapTotalHeight(lines []string, bodyWidth int) int {
	total := 0
	for i, line := range lines {
		gutter := m.viewerGutter(i, len(lines), false)
		total += softWrapSegmentCount(line, m.softWrapSegmentWidth(bodyWidth, gutter))
	}
	return total
}

func joinWithViewerScrollbar(lines []string, viewportHeight, totalHeight, topOffset, scrollbarWidth int) string {
	scrollbar := renderViewerScrollbar(viewportHeight, totalHeight, topOffset, scrollbarWidth)
	for i, line := range lines {
		bar := strings.Repeat(" ", scrollbarWidth)
		if i < len(scrollbar) {
			bar = scrollbar[i]
		}
		lines[i] = line + bar
	}
	return strings.Join(lines, "\n")
}

func renderViewerScrollbar(viewportHeight, totalHeight, topOffset, width int) []string {
	return renderVerticalScrollbar(viewportHeight, totalHeight, topOffset, verticalScrollbarOptions{
		Width:      width,
		Track:      "│",
		Thumb:      "┃",
		TrackStyle: sessionScrollbarTrackStyle,
		ThumbStyle: sessionScrollbarThumbStyle,
		AlignRight: true,
	})
}

func (m ViewerModel) viewerGutter(index, totalLines int, soft bool) string {
	if m.vp.LeftGutterFunc == nil {
		return ""
	}
	return m.vp.LeftGutterFunc(viewport.GutterContext{
		Index:      index,
		TotalLines: totalLines,
		Soft:       soft,
	})
}

func viewerContentLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) == 1 && ansi.StringWidth(lines[0]) == 0 {
		return nil
	}
	return lines
}
