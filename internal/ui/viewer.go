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
	return ViewerModel{vp: viewport.New(), state: viewerIdle}
}

// Path returns the currently loaded file path for titles/status display.
func (m ViewerModel) Path() string {
	return m.path
}

// Status returns a short human-readable state for status lines.
func (m ViewerModel) Status() string {
	return m.message
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
	return m.vp.View()
}
