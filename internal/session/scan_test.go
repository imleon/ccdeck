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

func TestParseFileExtractsWorkspaceProjectDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	content := `{"type":"user","cwd":"/repo/sub","workspace":{"project_dir":"/repo"},"message":{"content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.CWD != "/repo/sub" {
		t.Fatalf("cwd = %q, want /repo/sub", s.CWD)
	}
	if s.ProjectDir != "/repo" {
		t.Fatalf("projectDir = %q, want /repo", s.ProjectDir)
	}
}

func TestParseFileFallsBackProjectDirToCWD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	content := `{"type":"user","cwd":"/repo/sub","message":{"content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.ProjectDir != "/repo/sub" {
		t.Fatalf("projectDir = %q, want cwd fallback /repo/sub", s.ProjectDir)
	}
}
