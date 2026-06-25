package deck

import (
	"strings"
	"testing"
)

func TestBuildLayoutOrdersPanesLeftToRight(t *testing.T) {
	layout := BuildLayout(
		"/tmp/ccdeck",
		"dev",
		"/tmp/projects",
		"__sessions",
		"__explorer",
		"__file",
		"__claude-host",
	)

	if !strings.Contains(layout, `plugin location="tab-bar"`) {
		t.Fatalf("layout should include zellij tab bar plugin\n%s", layout)
	}
	if !strings.Contains(layout, `plugin location="status-bar"`) {
		t.Fatalf("layout should include zellij status bar plugin\n%s", layout)
	}
	if !strings.Contains(layout, `pane split_direction="vertical"`) {
		t.Fatalf("layout should use vertical split for horizontal columns\n%s", layout)
	}

	checks := []string{
		`pane size="15%" name="Project" command="/tmp/ccdeck" {
            args "__sessions" "--group" "dev" "--projects-dir" "/tmp/projects"`,
		`pane size="35%" name="Claude Code" command="/tmp/ccdeck" {
            args "__claude-host" "--group" "dev"`,
		`pane size="35%" name="File" command="/tmp/ccdeck" {
            args "__file" "--group" "dev"`,
		`pane size="15%" name="Explorer" command="/tmp/ccdeck" {
            args "__explorer" "--group" "dev"`,
	}

	if strings.Contains(layout, "--alt-screen=false") {
		t.Fatalf("layout should not include removed --alt-screen flag\n%s", layout)
	}

	last := -1
	for _, needle := range checks {
		idx := strings.Index(layout, needle)
		if idx == -1 {
			t.Fatalf("layout missing %q\n%s", needle, layout)
		}
		if idx <= last {
			t.Fatalf("layout order incorrect around %q\n%s", needle, layout)
		}
		last = idx
	}
}
