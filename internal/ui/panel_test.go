package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTakeCellSuffixPreservesOrder(t *testing.T) {
	got := takeCellSuffix("/data00/home/gaolei.veew/sourcecode/cc-sidecar", 10)
	if got != "cc-sidecar" {
		t.Fatalf("suffix order changed: got %q, want %q", got, "cc-sidecar")
	}
}

func TestFormatFooterKeepsDirectoryTail(t *testing.T) {
	got := formatFooter([]string{"/data00/home/gaolei.veew/sourcecode/cc-sidecar"}, 30)

	if lipgloss.Width(got) > 30 {
		t.Fatalf("footer width = %d, want <= 30: %q", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, "cc-sidecar") {
		t.Fatalf("footer should keep current directory tail, got %q", got)
	}
	if strings.Contains(got, "rac-edis-cc") {
		t.Fatalf("footer suffix appears reversed, got %q", got)
	}
}
