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
