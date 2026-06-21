package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cc-sidecar/internal/gitstatus"
)

func TestViewerTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: "(none)"},
		{name: "file", path: "/tmp/root.go", want: "root.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := viewerTitle(tt.path); got != tt.want {
				t.Fatalf("viewerTitle(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRootAppliesGitStatusResultForCurrentTreeRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	file := filepath.Join(sub, "a.go")
	writeTestFile(t, file)

	m := NewRoot(nil, Options{InitialRoot: root, RefreshInterval: time.Hour})
	model, cmd := m.Update(gitStatusRefreshedMsg{
		treeRoot: root,
		result: gitstatus.Result{
			Root:  root,
			Files: map[string]gitstatus.Status{file: gitstatus.StatusModified},
		},
	})
	if cmd != nil {
		t.Fatal("git status result should not return a command for current root")
	}
	got := model.(RootModel)
	if got.tree.gitStatus[file] != gitstatus.StatusModified {
		t.Fatalf("file status = %v, want Modified", got.tree.gitStatus[file])
	}
	if got.tree.gitStatus[sub] != gitstatus.StatusModified {
		t.Fatalf("sub dir status = %v, want Modified", got.tree.gitStatus[sub])
	}
}

func TestRootIgnoresStaleGitStatusResult(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	fileA := filepath.Join(rootA, "a.go")
	writeTestFile(t, fileA)
	writeTestFile(t, filepath.Join(rootB, "b.go"))

	m := NewRoot(nil, Options{InitialRoot: rootB, RefreshInterval: time.Hour})
	m.gitStatusInFlight = true
	model, _ := m.Update(gitStatusRefreshedMsg{
		treeRoot: rootA,
		result: gitstatus.Result{
			Root:  rootA,
			Files: map[string]gitstatus.Status{fileA: gitstatus.StatusModified},
		},
	})
	got := model.(RootModel)
	if got.tree.Root() != rootB {
		t.Fatalf("tree root = %q, want %q", got.tree.Root(), rootB)
	}
	if _, ok := got.tree.gitStatus[fileA]; ok {
		t.Fatalf("stale root status leaked into current tree: %v", got.tree.gitStatus)
	}
	if !got.gitStatusInFlight {
		t.Fatal("expected stale result to start a refresh for current root")
	}
}

func TestRootStartGitStatusRefreshSkipsWhenInFlight(t *testing.T) {
	root := t.TempDir()
	m := NewRoot(nil, Options{InitialRoot: root, RefreshInterval: time.Hour})

	m, cmd := m.startGitStatusRefresh()
	if cmd == nil {
		t.Fatal("first git status refresh should return a command")
	}
	if !m.gitStatusInFlight {
		t.Fatal("first git status refresh should mark in-flight")
	}

	m, cmd = m.startGitStatusRefresh()
	if cmd != nil {
		t.Fatal("second git status refresh should be skipped while in-flight")
	}
	if !m.gitStatusInFlight {
		t.Fatal("git status should remain in-flight")
	}
}

func TestRootSessionChosenSetsRootAndStartsGitStatusRefresh(t *testing.T) {
	root := t.TempDir()
	m := NewRoot(nil, Options{RefreshInterval: time.Hour})

	model, cmd := m.Update(sessionChosenMsg{cwd: root, id: "abc"})
	got := model.(RootModel)
	if got.tree.Root() != root {
		t.Fatalf("tree root = %q, want %q", got.tree.Root(), root)
	}
	if !got.gitStatusInFlight {
		t.Fatal("session selection should start git status refresh")
	}
	if cmd == nil {
		t.Fatal("session selection should return git status command")
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
