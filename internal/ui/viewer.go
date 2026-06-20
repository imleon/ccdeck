package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
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
	path      string
	content   string
	err       error
	state     viewerState
	message   string
	lineCount int
}

func (m ViewerModel) LoadFile(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			message := "无法读取: " + err.Error()
			return loadFileMsg{path: path, err: err, state: viewerReadError, message: message, content: errorStyle.Render(message)}
		}
		if len(data) == 0 {
			message := "空文件"
			return loadFileMsg{path: path, state: viewerEmpty, message: message, content: warningStyle.Render(message)}
		}
		if bytes.Contains(data, []byte{0}) {
			message := "二进制文件，未预览"
			return loadFileMsg{path: path, state: viewerBinary, message: message, content: warningStyle.Render(message)}
		}
		if len(data) > maxPreviewBytes {
			message := fmt.Sprintf("文件超过 %d KiB，未高亮预览", maxPreviewBytes/1024)
			return loadFileMsg{path: path, state: viewerTooLarge, message: message, content: warningStyle.Render(message)}
		}

		content := string(data)
		lineCount := countLines(content)
		rendered := highlightContent(path, content)
		return loadFileMsg{path: path, content: rendered, state: viewerLoaded, lineCount: lineCount, message: fmt.Sprintf("%d lines", lineCount)}
	}
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

func lineNumberGutter(lineCount int) viewport.GutterFunc {
	width := len(fmt.Sprintf("%d", lineCount))
	return func(info viewport.GutterContext) string {
		if info.Soft {
			return lineNumberStyle.Render(strings.Repeat(" ", width) + " │ ")
		}
		if info.Index >= info.TotalLines {
			return lineNumberStyle.Render(strings.Repeat(" ", width) + " │ ")
		}
		return lineNumberStyle.Render(fmt.Sprintf("%*d │ ", width, info.Index+1))
	}
}

func (m ViewerModel) Update(msg tea.Msg) (ViewerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadFileMsg:
		m.path = msg.path
		m.err = msg.err
		m.ready = true
		m.state = msg.state
		m.message = msg.message
		m.lineCount = msg.lineCount
		if msg.lineCount > 0 {
			m.vp.LeftGutterFunc = lineNumberGutter(msg.lineCount)
		} else {
			m.vp.LeftGutterFunc = nil
		}
		m.vp.SetContent(msg.content)
		m.vp.GotoTop()
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

	lines := viewerContentLines(m.vp.GetContent())
	start := min(m.vp.YOffset(), len(lines))
	end := min(start+height, len(lines))
	visible := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		gutter := m.viewerGutter(i, len(lines), false)
		contentWidth := max(width-ansi.StringWidth(gutter), 0)
		visible = append(visible, gutter+ansi.Cut(lines[i], m.vp.XOffset(), m.vp.XOffset()+contentWidth))
	}
	return strings.Join(visible, "\n")
}

func (m ViewerModel) softWrapView() string {
	width := m.vp.Width()
	height := m.vp.Height()
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := viewerContentLines(m.vp.GetContent())
	visible := make([]string, 0, height)
	skip := m.vp.YOffset()
	for i, line := range lines {
		firstGutter := m.viewerGutter(i, len(lines), false)
		contentWidth := max(width-ansi.StringWidth(firstGutter), 0)
		segmentWidth := max(contentWidth, 1)
		segmentCount := max((ansi.StringWidth(line)+segmentWidth-1)/segmentWidth, 1)

		for segment := 0; segment < segmentCount; segment++ {
			if skip > 0 {
				skip--
				continue
			}
			soft := segment > 0
			gutter := firstGutter
			if soft {
				gutter = m.viewerGutter(i, len(lines), true)
			}
			contentWidth = max(width-ansi.StringWidth(gutter), 0)
			start := segment * segmentWidth
			visible = append(visible, gutter+ansi.Cut(line, start, start+contentWidth))
			if len(visible) >= height {
				return strings.Join(visible, "\n")
			}
		}
	}
	return strings.Join(visible, "\n")
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
