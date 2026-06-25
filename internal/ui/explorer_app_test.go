package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/gitstatus"
	"ccdeck/internal/ipc"
)

func TestExplorerAppOpenFileOnlyUpdatesSelection(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	writeTestFile(t, file)
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.explorer = m.explorer.SetRoot(root)
	model, cmd := m.Update(explorerOpenFileMsg{path: file})
	if cmd != nil {
		t.Fatal("file selection should not return command")
	}
	got := model.(ExplorerAppModel)
	if got.openedPath != file {
		t.Fatalf("openedPath = %q, want %q", got.openedPath, file)
	}
	if strings.Contains(got.statusText(), "selected file:") {
		t.Fatalf("status should not show selected file: %q", got.statusText())
	}
	if got.statusText() != projectDirLabel(root) {
		t.Fatalf("status = %q, want %q", got.statusText(), projectDirLabel(root))
	}
}

func TestExplorerAppLinkedRootSetsRoot(t *testing.T) {
	root := t.TempDir()
	oldFile := filepath.Join(t.TempDir(), "old.go")
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.openedPath = oldFile
	model, cmd := m.Update(ipcSetRootMsg{path: root, sessionID: "abc"})
	got := model.(ExplorerAppModel)
	if got.explorer.Root() != root {
		t.Fatalf("root = %q, want %q", got.explorer.Root(), root)
	}
	if got.openedPath != "" {
		t.Fatalf("openedPath = %q, want empty", got.openedPath)
	}
	if cmd == nil {
		t.Fatal("linked root should schedule git refresh or IPC wait command")
	}
	if got.statusText() != projectDirLabel(root) {
		t.Fatalf("status = %q, want %q", got.statusText(), projectDirLabel(root))
	}
}

func TestExplorerAppLinkedRootClearsOpenedPathEvenWhenRootMatches(t *testing.T) {
	root := t.TempDir()
	oldFile := filepath.Join(root, "old.go")
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.explorer = m.explorer.SetRoot(root)
	m.openedPath = oldFile

	model, cmd := m.Update(ipcSetRootMsg{path: root, sessionID: "abc"})
	got := model.(ExplorerAppModel)
	if got.openedPath != "" {
		t.Fatalf("openedPath = %q, want empty", got.openedPath)
	}
	if cmd != nil {
		t.Fatal("same-root linked root should not schedule extra work")
	}
}

func TestExplorerAppOpenFileWithSenderReturnsCommand(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	writeTestFile(t, file)
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour, OpenFileSender: ipc.Sender{GroupName: "missing"}})
	m.explorer = m.explorer.SetRoot(root)
	_, cmd := m.Update(explorerOpenFileMsg{path: file})
	if cmd == nil {
		t.Fatal("linked file selection should return send command")
	}
}

func TestExplorerAppLinkedRootUnavailableShowsPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-project-dir")
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, cmd := m.Update(ipcSetRootMsg{path: missing, sessionID: "abc"})
	got := model.(ExplorerAppModel)
	if got.statusText() != "project dir unavailable: "+missing {
		t.Fatalf("status = %q, want %q", got.statusText(), "project dir unavailable: "+missing)
	}
	if cmd != nil {
		t.Fatal("unavailable linked root should not schedule IPC wait without a listener")
	}
}

func TestExplorerAppAppliesGitStatusForCurrentRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	writeTestFile(t, file)
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.explorer = m.explorer.SetRoot(root)
	model, cmd := m.Update(gitStatusRefreshedMsg{
		explorerRoot: root,
		repoRoots:    []string{filepath.Clean(root)},
		results: []gitstatus.Result{{
			Root:  root,
			Files: map[string]gitstatus.Status{file: gitstatus.StatusModified},
		}},
	})
	if cmd != nil {
		t.Fatal("current git status should not return command")
	}
	got := model.(ExplorerAppModel)
	if got.explorer.tree.gitStatus[file] != gitstatus.StatusModified {
		t.Fatalf("file status = %v, want Modified", got.explorer.tree.gitStatus[file])
	}
}

func TestExplorerAppAppliesNestedGitStatusOverlay(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	parentFile := filepath.Join(root, "parent.go")
	childFile := filepath.Join(nested, "child.go")
	writeTestFile(t, parentFile)
	writeTestFile(t, childFile)
	if err := os.Mkdir(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.explorer = m.explorer.SetRoot(root)
	model, cmd := m.Update(gitStatusRefreshedMsg{
		explorerRoot: root,
		repoRoots:    []string{filepath.Clean(root), filepath.Clean(nested)},
		results: []gitstatus.Result{
			{Root: root, Files: map[string]gitstatus.Status{parentFile: gitstatus.StatusModified, childFile: gitstatus.StatusDeleted}},
			{Root: nested, Files: map[string]gitstatus.Status{childFile: gitstatus.StatusAdded}},
		},
	})
	if cmd != nil {
		t.Fatal("current git status should not return command")
	}
	got := model.(ExplorerAppModel)
	if got.explorer.tree.gitStatus[parentFile] != gitstatus.StatusModified {
		t.Fatalf("parent file status = %v, want Modified", got.explorer.tree.gitStatus[parentFile])
	}
	if got.explorer.tree.gitStatus[childFile] != gitstatus.StatusAdded {
		t.Fatalf("child file status = %v, want Added", got.explorer.tree.gitStatus[childFile])
	}
	if got.explorer.tree.gitStatus[nested] != gitstatus.StatusAdded {
		t.Fatalf("nested summary = %v, want Added", got.explorer.tree.gitStatus[nested])
	}
}

func TestExplorerAppIgnoresStaleGitStatus(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	fileA := filepath.Join(rootA, "a.go")
	writeTestFile(t, fileA)
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	m.explorer = m.explorer.SetRoot(rootB)
	m.gitStatusInFlight = true
	model, _ := m.Update(gitStatusRefreshedMsg{
		explorerRoot: rootA,
		repoRoots:    []string{filepath.Clean(rootA)},
		results: []gitstatus.Result{{
			Root:  rootA,
			Files: map[string]gitstatus.Status{fileA: gitstatus.StatusModified},
		}},
	})
	got := model.(ExplorerAppModel)
	if got.explorer.Root() != rootB {
		t.Fatalf("root = %q, want %q", got.explorer.Root(), rootB)
	}
	if _, ok := got.explorer.tree.gitStatus[fileA]; ok {
		t.Fatalf("stale status leaked: %v", got.explorer.tree.gitStatus)
	}
}

func TestExplorerAppStandaloneLayoutUsesInsetBodyWidth(t *testing.T) {
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := model.(ExplorerAppModel)
	if got.explorer.tree.width != 79 {
		t.Fatalf("explorer width = %d, want 79", got.explorer.tree.width)
	}
	if got.explorer.tree.height != 24 {
		t.Fatalf("explorer height = %d, want 24", got.explorer.tree.height)
	}
}

func TestExplorerAppStandaloneRenderHasNoPanelBorder(t *testing.T) {
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := stripANSI(model.(ExplorerAppModel).render())
	if strings.Contains(plain, "Explorer") || strings.Contains(plain, "Group: dev") {
		t.Fatalf("render should not repeat pane title or group: %q", plain)
	}
	if strings.ContainsAny(plain, "╭╮╰╯") {
		t.Fatalf("standalone render should not contain panel border: %q", plain)
	}
}

func TestExplorerAppHelpView(t *testing.T) {
	m := NewExplorerApp(ExplorerAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = model.(ExplorerAppModel).Update(sessionsKey("?"))
	got := model.(ExplorerAppModel).render()
	if !strings.Contains(got, "Explorer pane") || strings.Contains(got, "Runtime: dev") {
		t.Fatalf("help view = %q", got)
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
