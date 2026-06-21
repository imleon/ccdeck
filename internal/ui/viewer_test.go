package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func viewerKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	code := rune(0)
	if len(runes) > 0 {
		code = runes[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func TestViewerDefaultsToNoSoftWrap(t *testing.T) {
	viewer := NewViewer()
	if viewer.SoftWrap() {
		t.Fatal("viewer should default to no soft wrap")
	}
	if got := viewer.WrapStatus(); got != "wrap: off" {
		t.Fatalf("WrapStatus() = %q, want %q", got, "wrap: off")
	}
}

func TestViewerLoadGoesToTopRefreshKeepsScroll(t *testing.T) {
	content := ""
	for range 200 {
		content += "line\n"
	}

	viewer := NewViewer().SetSize(40, 10)

	// 首次加载：应回到顶部。
	var m ViewerModel = viewer
	m.path = "/tmp/a.txt"
	m, _ = m.Update(loadFileMsg{
		requestID: m.requestID,
		path:      "/tmp/a.txt",
		content:   content,
		state:     viewerLoaded,
		lineCount: 200,
	})
	if got := m.vp.YOffset(); got != 0 {
		t.Fatalf("first load should be at top, YOffset = %d, want 0", got)
	}

	// 用户向下滚动。
	m.vp.SetYOffset(50)
	if got := m.vp.YOffset(); got != 50 {
		t.Fatalf("setup scroll failed, YOffset = %d, want 50", got)
	}

	// 自动刷新：内容更新但滚动位置应保留。
	m, _ = m.Update(loadFileMsg{
		path:           "/tmp/a.txt",
		content:        content + "more\n",
		state:          viewerLoaded,
		lineCount:      201,
		preserveScroll: true,
	})
	if got := m.vp.YOffset(); got != 50 {
		t.Fatalf("refresh should preserve scroll, YOffset = %d, want 50", got)
	}
}

func TestViewerTogglesSoftWrapWithW(t *testing.T) {
	viewer := NewViewer()

	var cmd tea.Cmd
	viewer, cmd = viewer.Update(viewerKey("w"))
	if cmd != nil {
		t.Fatal("wrap toggle should not return a command")
	}
	if !viewer.SoftWrap() {
		t.Fatal("viewer should enable soft wrap after first w")
	}
	if got := viewer.WrapStatus(); got != "wrap: panel" {
		t.Fatalf("WrapStatus() = %q, want %q", got, "wrap: panel")
	}

	viewer, cmd = viewer.Update(viewerKey("w"))
	if cmd != nil {
		t.Fatal("wrap toggle should not return a command")
	}
	if viewer.SoftWrap() {
		t.Fatal("viewer should disable soft wrap after second w")
	}
	if got := viewer.WrapStatus(); got != "wrap: off" {
		t.Fatalf("WrapStatus() = %q, want %q", got, "wrap: off")
	}
}
