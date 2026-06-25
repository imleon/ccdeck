package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func writeViewerFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileAppUpdateLoadFileMsg(t *testing.T) {
	path := writeViewerFile(t, "hello\nworld\n")
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.file, _ = m.file.LoadPath(path)
	msg := renderFileMsg(path, m.file.viewer.requestID, false)
	model, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("load file update should not return command")
	}
	got := model.(FileAppModel)
	if got.file.Status() != "2 lines" {
		t.Fatalf("status = %q, want 2 lines", got.file.Status())
	}
}

func TestFileAppToggleWrap(t *testing.T) {
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(viewerKey("w"))
	got := model.(FileAppModel)
	if !got.file.viewer.SoftWrap() {
		t.Fatal("w should toggle soft wrap")
	}
}

func TestFileAppLinkedOpenFileLoadsPath(t *testing.T) {
	path := writeViewerFile(t, "hello\n")
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, cmd := m.Update(ipcOpenFileMsg{path: path})
	got := model.(FileAppModel)
	if got.file.Path() != path {
		t.Fatalf("path = %q, want %q", got.file.Path(), path)
	}
	if cmd == nil {
		t.Fatal("linked file open should return load command")
	}
}

func TestFileAppClearResetsCurrentFile(t *testing.T) {
	path := writeViewerFile(t, "hello\n")
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.file, _ = m.file.LoadPath(path)
	model, cmd := m.Update(ipcClearFileMsg{root: "/repo"})
	if cmd != nil {
		t.Fatal("clear file without IPC listener should not schedule command")
	}
	got := model.(FileAppModel)
	if got.file.Path() != "" {
		t.Fatalf("path = %q, want empty", got.file.Path())
	}
	if !strings.Contains(got.statusText(), "waiting for file selection") {
		t.Fatalf("status = %q", got.statusText())
	}
}

func TestFileAppAllowsEmptyInitialPath(t *testing.T) {
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	if got := m.file.Path(); got != "" {
		t.Fatalf("path = %q, want empty", got)
	}
}

func TestFileAppStandaloneLayoutUsesInsetBodyWidth(t *testing.T) {
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := model.(FileAppModel)
	if got.file.viewer.vp.Width() != 79 {
		t.Fatalf("file width = %d, want 79", got.file.viewer.vp.Width())
	}
	if got.file.viewer.vp.Height() != 23 {
		t.Fatalf("file height = %d, want 23", got.file.viewer.vp.Height())
	}
}

func TestFileAppStandaloneRenderHasNoPanelBorder(t *testing.T) {
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := stripANSI(model.(FileAppModel).render())
	if strings.Contains(plain, "File") || strings.Contains(plain, "Group: dev") {
		t.Fatalf("render should not repeat pane title or group: %q", plain)
	}
	if strings.ContainsAny(plain, "╭╮╰╯") {
		t.Fatalf("standalone render should not contain panel border: %q", plain)
	}
}

func TestFileAppHelpView(t *testing.T) {
	m := NewFileApp(FileAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = model.(FileAppModel).Update(viewerKey("?"))
	got := model.(FileAppModel).render()
	if !strings.Contains(got, "File pane") || strings.Contains(got, "Runtime: dev") {
		t.Fatalf("help view = %q", got)
	}
}
