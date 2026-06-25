package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"ccdeck/internal/gitstatus"
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

func TestTreeSetRootRendersRootAsExpandedDirectory(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)

	if len(m.nodes) != 3 {
		t.Fatalf("nodes = %v, want root plus two children", treeNodePaths(m))
	}
	if got := m.nodes[0]; got.path != root || !got.isDir || got.depth != 0 || !got.expanded {
		t.Fatalf("root node = %+v, want expanded directory at depth 0", got)
	}
	if got := m.nodes[1]; got.path != sub || !got.isDir || got.depth != 1 {
		t.Fatalf("first child = %+v, want sub directory at depth 1", got)
	}
	if got := m.nodes[2]; got.path != file || got.isDir || got.depth != 1 {
		t.Fatalf("second child = %+v, want file at depth 1", got)
	}
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

	// 展开 sub（index 0 = root, index 1 = sub），再把光标移到子文件 a.go。
	m.cursor = 1
	m = m.toggle()
	if !m.nodes[1].expanded {
		t.Fatal("sub should be expanded after toggle")
	}
	m.cursor = 2 // sub/a.go
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
	if !m.nodes[1].expanded {
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
	m.cursor = 2 // b.txt
	if got := treeCursorPath(m); got != filepath.Join(root, "b.txt") {
		t.Fatalf("cursor path = %q, want b.txt", got)
	}

	// 删除光标所在文件，刷新不应越界且光标 clamp 到合法范围。
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	m = m.Refresh()

	if len(m.nodes) != 2 {
		t.Fatalf("expected root plus 1 child after deletion, got %d", len(m.nodes))
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

func TestTreeRefreshPreservesCollapsedRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(40, 20)
	m = m.toggle()
	if len(m.nodes) != 1 || m.nodes[0].expanded {
		t.Fatalf("root should be collapsed, got %+v", m.nodes)
	}

	m = m.Refresh()
	if len(m.nodes) != 1 || m.nodes[0].path != root || m.nodes[0].expanded {
		t.Fatalf("refresh should preserve collapsed root, got %+v", m.nodes)
	}
}

func TestTreeLeftFromRootChildMovesToRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(40, 20)
	m.cursor = 1
	m = m.collapseOrMoveToParent()

	if got := treeCursorPath(m); got != root {
		t.Fatalf("cursor path after left = %q, want root", got)
	}
}

func TestTreeEnterOnRootTogglesWithoutSelectingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(40, 20)
	m, cmd := m.Update(sessionsKey("enter"))
	if cmd != nil {
		t.Fatal("enter on root directory should not select a file")
	}
	if len(m.nodes) != 1 || m.nodes[0].expanded {
		t.Fatalf("enter should collapse root, got %+v", m.nodes)
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

func TestTreeVisibleGitRepoRootsDetectsGitDirAndFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo-dir")
	repoFile := filepath.Join(root, "repo-file")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoFile, ".git"), []byte("gitdir: ../meta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	got := m.VisibleGitRepoRoots()
	want := []string{filepath.Clean(repoDir), filepath.Clean(repoFile)}
	if !slices.Equal(got, want) {
		t.Fatalf("visible git roots = %v, want %v", got, want)
	}
}

func TestTreeVisibleGitRepoRootsOnlyChecksVisibleNodes(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	nested := filepath.Join(parent, "nested")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	if got := m.VisibleGitRepoRoots(); len(got) != 0 {
		t.Fatalf("collapsed nested git roots = %v, want none", got)
	}
	m.cursor = 1
	m = m.toggle()
	if got := m.VisibleGitRepoRoots(); !slices.Equal(got, []string{filepath.Clean(nested)}) {
		t.Fatalf("expanded nested git roots = %v, want nested", got)
	}
}

func TestTreeSetGitStatusMapInjectsNestedStatus(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	file := filepath.Join(nested, "a.go")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	m = m.SetGitStatusMap(map[string]gitstatus.Status{
		nested: gitstatus.StatusModified,
		file:   gitstatus.StatusModified,
	})
	if got := m.gitStatus[nested]; got != gitstatus.StatusModified {
		t.Fatalf("nested status = %v, want Modified", got)
	}
	if got := m.gitStatus[file]; got != gitstatus.StatusModified {
		t.Fatalf("file status = %v, want Modified", got)
	}
}

func TestTreeViewRendersScrollbarWhenContentOverflows(t *testing.T) {
	root := t.TempDir()
	for i := range 8 {
		path := filepath.Join(root, string(rune('a'+i))+".txt")
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	got := stripANSI(m.View(""))

	if !strings.Contains(got, "┃") {
		t.Fatalf("expected scrollbar thumb\n%s", got)
	}
}

func TestTreeViewKeepsScrollbarGutterWhenContentFits(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	got := stripANSI(m.View(""))
	line := strings.Split(got, "\n")[0]
	if width := lipgloss.Width(line); width != 24 {
		t.Fatalf("line width = %d, want 24: %q", width, line)
	}
}

func TestTreeViewUsesActiveBackgroundForOpenedFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	m.cursor = -1
	raw := m.View(file)
	if raw == stripANSI(raw) || !strings.Contains(raw, "48;2;42;36;33") {
		t.Fatalf("opened file should use active background\n%q", raw)
	}
}

func TestTreeViewUsesContinuousActiveBackgroundForOpenedGitFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	m.cursor = -1
	m = m.SetGitStatusMap(map[string]gitstatus.Status{file: gitstatus.StatusModified})
	raw := m.View(file)
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected opened file line, got %q", raw)
	}
	fileLine := lines[1]
	if !strings.Contains(fileLine, " M") {
		t.Fatalf("opened file line should include git mark: %q", fileLine)
	}
	if strings.Contains(fileLine, "38;2;178") {
		t.Fatalf("opened git file should not split active background with git color style: %q", fileLine)
	}
	if strings.Count(fileLine, "48;2;42;36;33") != 2 {
		t.Fatalf("opened git file should have one active row span plus trailing fill, got %q", fileLine)
	}
}

func TestTreeViewReservesGitMarkColumnForCleanRows(t *testing.T) {
	root := t.TempDir()
	cleanFile := filepath.Join(root, "clean-file-with-long-name.txt")
	dirtyFile := filepath.Join(root, "dirty-file-with-long-name.txt")
	if err := os.WriteFile(cleanFile, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirtyFile, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 4)
	m = m.SetGitStatusMap(map[string]gitstatus.Status{dirtyFile: gitstatus.StatusModified})
	lines := strings.Split(stripANSI(m.View("")), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected root and two file lines, got %q", lines)
	}
	cleanLine := lines[1]
	dirtyLine := lines[2]
	if strings.Contains(cleanLine, "clean-file-with-long-name") {
		t.Fatalf("clean row should truncate before reserved git mark column: %q", cleanLine)
	}
	if strings.Contains(dirtyLine, "dirty-file-with-long-name") {
		t.Fatalf("dirty row should truncate before git mark column: %q", dirtyLine)
	}
	idx := strings.LastIndex(dirtyLine, "M")
	if idx == -1 {
		t.Fatalf("dirty row should include git mark: %q", dirtyLine)
	}
	if markColumn := lipgloss.Width(dirtyLine[:idx]); markColumn != lipgloss.Width(cleanLine)-treeGitMarkColumnWidth-2 {
		t.Fatalf("dirty mark should align with clean reserved column; clean=%q dirty=%q", cleanLine, dirtyLine)
	}
}

func TestTreeViewUsesSelectedBackgroundForCursor(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	raw := m.View("")
	if raw == stripANSI(raw) || !strings.Contains(raw, "48;2;74;58;50") {
		t.Fatalf("cursor should use selected background\n%q", raw)
	}
}

func TestTreeViewSelectedBackgroundWinsOverOpenedFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root).SetSize(24, 3)
	m.cursor = 1
	raw := m.View(file)
	if raw == stripANSI(raw) || !strings.Contains(raw, "48;2;74;58;50") {
		t.Fatalf("selected opened file should use selected background\n%q", raw)
	}
	if strings.Contains(raw, "48;2;42;36;33") {
		t.Fatalf("selected opened file should not use active background\n%q", raw)
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
	m = m.SetGitStatus(map[string]gitstatus.Status{file: gitstatus.StatusDeleted}, root)

	if got := m.gitStatus[file]; got != gitstatus.StatusDeleted {
		t.Fatalf("file status = %v, want Deleted", got)
	}
	if got := m.gitStatus[sub]; got != gitstatus.StatusDeleted {
		t.Fatalf("sub dir status = %v, want Deleted", got)
	}
}

func TestTreeSetGitStatusAggregatesMixedDirectoryStatusAsModified(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	modFile := filepath.Join(sub, "a.go")
	newFile := filepath.Join(sub, "b.go")
	if err := os.WriteFile(modFile, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	m = m.SetGitStatus(map[string]gitstatus.Status{
		modFile: gitstatus.StatusModified,
		newFile: gitstatus.StatusUntracked,
	}, root)

	if got := m.gitStatus[sub]; got != gitstatus.StatusModified {
		t.Fatalf("sub dir status = %v, want Modified", got)
	}
}

func TestTreeSetGitStatusAggregatesConflictDirectoryStatus(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	modFile := filepath.Join(sub, "a.go")
	conflictFile := filepath.Join(sub, "b.go")
	if err := os.WriteFile(modFile, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflictFile, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTree().SetRoot(root)
	m = m.SetGitStatus(map[string]gitstatus.Status{
		modFile:      gitstatus.StatusModified,
		conflictFile: gitstatus.StatusConflict,
	}, root)

	if got := m.gitStatus[sub]; got != gitstatus.StatusConflict {
		t.Fatalf("sub dir status = %v, want Conflict", got)
	}
}
