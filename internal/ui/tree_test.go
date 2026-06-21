package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cc-sidecar/internal/gitstatus"
)

// treeNodePaths 返回当前可见节点的 path 列表，便于断言。
func treeNodePaths(m TreeModel) []string {
	paths := make([]string, 0, len(m.nodes))
	for _, n := range m.nodes {
		paths = append(paths, n.path)
	}
	return paths
}

func treeCursorPath(m TreeModel) string {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	return m.nodes[m.cursor].path
}

func TestTreeRefreshReflectsDiskAndPreservesState(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "z.txt"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(40, 20)

	// 展开 sub（光标在 index 0 = sub 目录），再把光标移到子文件 a.go。
	m = m.toggle()
	if !m.nodes[0].expanded {
		t.Fatal("sub should be expanded after toggle")
	}
	m.cursor = 1 // sub/a.go
	if got := treeCursorPath(m); got != filepath.Join(sub, "a.go") {
		t.Fatalf("cursor path = %q, want sub/a.go", got)
	}

	// 磁盘上在已展开目录里新增一个文件。
	if err := os.WriteFile(filepath.Join(sub, "b.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = m.Refresh()

	// 新文件应出现，且 sub 仍展开。
	paths := treeNodePaths(m)
	wantNew := filepath.Join(sub, "b.go")
	found := false
	for _, p := range paths {
		if p == wantNew {
			found = true
		}
	}
	if !found {
		t.Fatalf("refresh should surface new file %q, got %v", wantNew, paths)
	}
	if !m.nodes[0].expanded {
		t.Fatal("sub should stay expanded after refresh")
	}
	// 光标应仍指向 a.go。
	if got := treeCursorPath(m); got != filepath.Join(sub, "a.go") {
		t.Fatalf("cursor path after refresh = %q, want sub/a.go", got)
	}
}

func TestTreeRefreshHandlesDeletedCursorNode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(40, 20)
	m.cursor = 1 // b.txt
	if got := treeCursorPath(m); got != filepath.Join(root, "b.txt") {
		t.Fatalf("cursor path = %q, want b.txt", got)
	}

	// 删除光标所在文件，刷新不应越界且光标 clamp 到合法范围。
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	m = m.Refresh()

	if len(m.nodes) != 1 {
		t.Fatalf("expected 1 node after deletion, got %d", len(m.nodes))
	}
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		t.Fatalf("cursor out of range after refresh: %d", m.cursor)
	}
	if got := treeCursorPath(m); got != filepath.Join(root, "a.txt") {
		t.Fatalf("cursor path after refresh = %q, want a.txt", got)
	}
}

func TestTreeRefreshNoRootIsNoop(t *testing.T) {
	m := NewTree()
	m = m.Refresh()
	if len(m.nodes) != 0 {
		t.Fatalf("refresh without root should keep empty nodes, got %d", len(m.nodes))
	}
}

func TestTreeSetRootClearsGitStatusOnRootChange(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	fileA := filepath.Join(rootA, "a.go")
	if err := os.WriteFile(fileA, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(rootA)
	m = m.SetGitStatus(map[string]gitstatus.Status{fileA: gitstatus.StatusModified}, rootA)
	if len(m.gitStatus) == 0 {
		t.Fatal("expected git status to be set")
	}

	m = m.SetRoot(rootB)
	if len(m.gitStatus) != 0 {
		t.Fatalf("git status should be cleared after root change, got %v", m.gitStatus)
	}
}

func TestTreeSetGitStatusAggregatesDirectoryStatus(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "a.go")
	if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	m = m.SetGitStatus(map[string]gitstatus.Status{file: gitstatus.StatusModified}, root)

	if got := m.gitStatus[file]; got != gitstatus.StatusModified {
		t.Fatalf("file status = %v, want Modified", got)
	}
	if got := m.gitStatus[sub]; got != gitstatus.StatusModified {
		t.Fatalf("sub dir status = %v, want Modified", got)
	}
}
