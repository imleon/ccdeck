package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTakeCellSuffixPreservesOrder(t *testing.T) {
	got := takeCellSuffix("/data00/home/gaolei.veew/sourcecode/ccdeck", lipgloss.Width("ccdeck"))
	if got != "ccdeck" {
		t.Fatalf("suffix order changed: got %q, want %q", got, "ccdeck")
	}
}

func TestFormatFooterKeepsDirectoryTail(t *testing.T) {
	got := formatFooter([]string{"/data00/home/gaolei.veew/sourcecode/ccdeck"}, 30)

	if lipgloss.Width(got) > 30 {
		t.Fatalf("footer width = %d, want <= 30: %q", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, "ccdeck") {
		t.Fatalf("footer should keep current directory tail, got %q", got)
	}
	if strings.Contains(got, "rac-edis-cc") {
		t.Fatalf("footer suffix appears reversed, got %q", got)
	}
}

func TestStandaloneInsetContentSizeSubtractsHorizontalPadding(t *testing.T) {
	width, height := standaloneInsetContentSize(80, 24, 1)
	if width != 78 || height != 23 {
		t.Fatalf("content size = (%d, %d), want (78, 23)", width, height)
	}
}

func TestPaneContentWidthUsesAsymmetricPadding(t *testing.T) {
	width := paneContentWidth(20, panePadding{Left: 2, Right: 3})
	if width != 15 {
		t.Fatalf("content width = %d, want 15", width)
	}
}

func TestStandalonePaddedContentSizeSupportsScrollbarPadding(t *testing.T) {
	width, height := standalonePaddedContentSize(80, 24, standaloneScrollbarBodyPadding)
	if width != 79 || height != 23 {
		t.Fatalf("content size = (%d, %d), want (79, 23)", width, height)
	}
}

func TestRenderStandalonePanePaddedSupportsScrollbarPadding(t *testing.T) {
	rendered := renderStandalonePanePadded("Status", "body", 12, standaloneScrollbarBodyPadding)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered lines = %d, want >= 2: %q", len(lines), rendered)
	}
	if !strings.HasPrefix(lines[1], " ") {
		t.Fatalf("body line should keep left padding: %q", lines[1])
	}
	if !strings.HasPrefix(lines[1], " body") {
		t.Fatalf("body line should keep only left structural padding before body: %q", lines[1])
	}
	if lipgloss.Width(lines[1]) != 12 {
		t.Fatalf("body line width = %d, want 12: %q", lipgloss.Width(lines[1]), lines[1])
	}
}

func TestRenderStandalonePaneInsetAddsBodyGutter(t *testing.T) {
	rendered := renderStandalonePaneInset("Title", "Status", "body", 12, 1)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered lines = %d, want >= 2: %q", len(lines), rendered)
	}
	if !strings.HasPrefix(lines[1], " ") {
		t.Fatalf("body line should start with inset space: %q", lines[1])
	}
	if lipgloss.Width(lines[1]) != 12 {
		t.Fatalf("body line width = %d, want 12: %q", lipgloss.Width(lines[1]), lines[1])
	}
}

func TestRenderVerticalScrollbarAlignsThumbRight(t *testing.T) {
	bars := renderVerticalScrollbar(4, 8, 0, verticalScrollbarOptions{Width: 3, Track: "│", Thumb: "┃", AlignRight: true})
	plain := strings.Join(bars, "\n")
	lines := strings.Split(plain, "\n")
	if len(lines) != 4 {
		t.Fatalf("scrollbar lines = %d, want 4", len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) != 3 {
			t.Fatalf("scrollbar line width = %d, want 3: %q", lipgloss.Width(line), line)
		}
	}
	if !strings.HasSuffix(lines[0], "┃") {
		t.Fatalf("thumb should be right-aligned, got %q", lines[0])
	}
}
