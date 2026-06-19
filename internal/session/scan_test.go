package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScan uses the real ~/.claude/projects directory as a fixture. These
// transcripts are local user data; the test only validates structural invariants.
func TestScan(t *testing.T) {
	projectsDir := ProjectsDir()
	if _, err := os.Stat(projectsDir); err != nil {
		t.Skipf("no Claude Code projects dir: %v", err)
	}

	sessions, err := Scan(projectsDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	for i, s := range sessions {
		if s.ID == "" {
			t.Fatalf("session %d has empty ID: %+v", i, s)
		}
		if s.Path == "" || filepath.Ext(s.Path) != ".jsonl" {
			t.Fatalf("session %s has invalid path: %q", s.ID, s.Path)
		}
		if strings.Contains(s.Path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			t.Fatalf("subagent transcript was not excluded: %s", s.Path)
		}
		if s.Title == "" {
			t.Fatalf("session %s has empty title", s.ID)
		}
	}

	for i := 1; i < len(sessions); i++ {
		if sessions[i-1].ModTime.Before(sessions[i].ModTime) {
			t.Fatalf("sessions not sorted newest-first at %d", i)
		}
	}
}
