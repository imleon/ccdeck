package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"ccdeck/internal/gitstatus"
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

func TestViewerNowrapViewRendersScrollbarWhenContentOverflows(t *testing.T) {
	viewer := NewViewer().SetSize(24, 3)
	viewer.path = "/tmp/a.txt"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content:   strings.Repeat("line\n", 8),
		state:     viewerLoaded,
		lineCount: 8,
	})

	got := stripANSI(viewer.View())
	if !strings.Contains(got, "┃") {
		t.Fatalf("expected scrollbar thumb\n%s", got)
	}
}

func TestViewerNowrapViewKeepsScrollbarGutterWhenContentFits(t *testing.T) {
	viewer := NewViewer().SetSize(24, 3)
	viewer.path = "/tmp/a.txt"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content:   "one\ntwo",
		state:     viewerLoaded,
		lineCount: 2,
	})

	got := stripANSI(viewer.View())
	line := strings.Split(got, "\n")[0]
	if width := lipgloss.Width(line); width != 24 {
		t.Fatalf("line width = %d, want 24: %q", width, line)
	}
}

func TestViewerSoftWrapViewRendersScrollbarForWrappedRows(t *testing.T) {
	viewer := NewViewer().SetSize(14, 3)
	viewer.vp.SoftWrap = true
	viewer.path = "/tmp/a.txt"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content:   "abcdefghijklmnopqrstuvwxyz",
		state:     viewerLoaded,
		lineCount: 1,
	})

	got := stripANSI(viewer.View())
	if !strings.Contains(got, "┃") {
		t.Fatalf("expected scrollbar thumb for wrapped rows\n%s", got)
	}
}

func TestViewerInlineDiffChangedRowsHaveBackground(t *testing.T) {
	viewer := NewViewer().SetSize(40, 4)
	viewer.path = "/tmp/a.go"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content: strings.Join([]string{
			"package a",
			"func old() {}",
			"func new() {}",
		}, "\n"),
		state:     viewerLoaded,
		lineCount: 3,
		diffKinds: []gitstatus.DiffLineKind{
			gitstatus.DiffLineContext,
			gitstatus.DiffLineDeleted,
			gitstatus.DiffLineAdded,
		},
	})

	got := viewer.View()
	if !strings.Contains(got, viewerDiffDeletedStyle.Render("2-│ func old() {}                     ")) {
		t.Fatalf("deleted line should include body background\n%q", got)
	}
	if !strings.Contains(got, viewerDiffAddedStyle.Render("3+│ func new() {}                     ")) {
		t.Fatalf("added line should include body background\n%q", got)
	}
	if strings.Contains(got, viewerDiffAddedStyle.Render("1 │ package a")) || strings.Contains(got, viewerDiffDeletedStyle.Render("1 │ package a")) {
		t.Fatalf("context line should not use diff background\n%q", got)
	}
}

func TestViewerInlineDiffMarkersRenderInGutter(t *testing.T) {
	viewer := NewViewer().SetSize(40, 4)
	viewer.path = "/tmp/a.go"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content: strings.Join([]string{
			"package a",
			"func old() {}",
			"func new() {}",
		}, "\n"),
		state:     viewerLoaded,
		lineCount: 3,
		diffKinds: []gitstatus.DiffLineKind{
			gitstatus.DiffLineContext,
			gitstatus.DiffLineDeleted,
			gitstatus.DiffLineAdded,
		},
	})

	got := stripANSI(viewer.View())
	for _, want := range []string{
		"1 │ package a",
		"2-│ func old() {}",
		"3+│ func new() {}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("view missing %q\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"2 │ -func old() {}",
		"3 │ +func new() {}",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("diff marker should not be in content: %q\n%s", unwanted, got)
		}
	}
}

func TestViewerInlineDiffSoftWrapContinuationHasBlankMarker(t *testing.T) {
	viewer := NewViewer().SetSize(14, 3)
	viewer.vp.SoftWrap = true
	viewer.path = "/tmp/a.go"
	viewer, _ = viewer.Update(loadFileMsg{
		requestID: viewer.requestID,
		path:      viewer.path,
		content:   "abcdefghijklmnop",
		state:     viewerLoaded,
		lineCount: 1,
		diffKinds: []gitstatus.DiffLineKind{gitstatus.DiffLineAdded},
	})

	raw := viewer.View()
	got := stripANSI(raw)
	if !strings.Contains(got, "1+│ abcdefgh") {
		t.Fatalf("first wrapped segment should keep added marker\n%s", got)
	}
	if !strings.Contains(got, "  │ ijklmnop") {
		t.Fatalf("wrapped continuation should use blank marker\n%s", got)
	}
	if strings.Contains(got, " +│ ijklmnop") {
		t.Fatalf("wrapped continuation should not repeat marker\n%s", got)
	}
	if !strings.Contains(raw, viewerDiffAddedStyle.Render("  │ ijklmnop")) {
		t.Fatalf("wrapped continuation should keep changed-line background\n%q", raw)
	}
}
