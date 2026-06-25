package gitstatus

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParsePorcelainZ(t *testing.T) {
	// Build a -z porcelain stream by hand: NUL-separated records, with a rename
	// record carrying its original-path field.
	data := []byte(
		" M file_mod.go\x00" +
			"A  file_add.go\x00" +
			" D file_del.go\x00" +
			"?? file_new.go\x00" +
			"UU file_conflict.go\x00" +
			"R  new_name.go\x00old_name.go\x00",
	)

	got := parsePorcelainZ(data)

	want := map[string]Status{
		"file_mod.go":      StatusModified,
		"file_add.go":      StatusAdded,
		"file_del.go":      StatusDeleted,
		"file_new.go":      StatusUntracked,
		"file_conflict.go": StatusConflict,
		"new_name.go":      StatusRenamed,
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d records, want %d: %+v", len(got), len(want), got)
	}
	for path, st := range want {
		if got[path] != st {
			t.Errorf("path %q: got %v, want %v", path, got[path], st)
		}
	}
	// The rename's original path must not leak in as its own record.
	if _, ok := got["old_name.go"]; ok {
		t.Error("rename original path old_name.go leaked as a record")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		x, y byte
		want Status
	}{
		{' ', 'M', StatusModified},
		{'M', ' ', StatusModified},
		{'A', ' ', StatusAdded},
		{' ', 'D', StatusDeleted},
		{'?', '?', StatusUntracked},
		{'U', 'U', StatusConflict},
		{'D', 'D', StatusConflict},
		{'A', 'A', StatusConflict},
		{'R', ' ', StatusRenamed},
	}
	for _, c := range cases {
		if got := classify(c.x, c.y); got != c.want {
			t.Errorf("classify(%q,%q) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	if got := Summarize(nil); got != StatusNone {
		t.Fatalf("empty summary = %v, want None", got)
	}
	if got := Summarize(map[string]Status{"a": StatusDeleted}); got != StatusDeleted {
		t.Fatalf("single summary = %v, want Deleted", got)
	}
	if got := Summarize(map[string]Status{"a": StatusModified, "b": StatusUntracked}); got != StatusModified {
		t.Fatalf("mixed summary = %v, want Modified", got)
	}
	if got := Summarize(map[string]Status{"a": StatusModified, "b": StatusConflict}); got != StatusConflict {
		t.Fatalf("conflict summary = %v, want Conflict", got)
	}
}

func TestAggregate(t *testing.T) {
	root := "/repo"
	files := map[string]Status{
		"/repo/deleted/a.go":    StatusDeleted,
		"/repo/mixed/mod.go":    StatusModified,
		"/repo/mixed/new.go":    StatusUntracked,
		"/repo/conflict/mod.go": StatusModified,
		"/repo/conflict/u.go":   StatusConflict,
	}
	dirs := Aggregate(files, root)

	if dirs["/repo/deleted"] != StatusDeleted {
		t.Errorf("/repo/deleted = %v, want Deleted", dirs["/repo/deleted"])
	}
	if dirs["/repo/mixed"] != StatusModified {
		t.Errorf("/repo/mixed = %v, want Modified", dirs["/repo/mixed"])
	}
	if dirs["/repo/conflict"] != StatusConflict {
		t.Errorf("/repo/conflict = %v, want Conflict", dirs["/repo/conflict"])
	}
	if _, ok := dirs[root]; ok {
		t.Error("root should not appear in aggregated dirs")
	}
}

// TestLoadIntegration drives the real git binary against a throwaway repo,
// covering modified / added / deleted / untracked in one shot.
func TestLoadIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")

	// Committed baseline: tracked.go (will modify) and gone.go (will delete).
	writeFile(t, dir, "tracked.go", "package a\n")
	writeFile(t, dir, "gone.go", "package a\n")
	run("add", ".")
	run("commit", "-m", "init")

	// Now create the four states.
	writeFile(t, dir, "tracked.go", "package a\n// changed\n") // modified
	writeFile(t, dir, "fresh.go", "package a\n")               // untracked
	writeFile(t, dir, "staged.go", "package a\n")              // added (staged)
	run("add", "staged.go")
	if err := os.Remove(filepath.Join(dir, "gone.go")); err != nil {
		t.Fatal(err)
	}

	res, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Root == "" {
		t.Fatal("expected a repo root")
	}

	want := map[string]Status{
		"tracked.go": StatusModified,
		"fresh.go":   StatusUntracked,
		"staged.go":  StatusAdded,
		"gone.go":    StatusDeleted,
	}
	for name, st := range want {
		abs := filepath.Join(res.Root, name)
		if res.Files[abs] != st {
			t.Errorf("%s: got %v, want %v", name, res.Files[abs], st)
		}
	}
}

func TestLoadFromSubdirectoryReportsRootRelativePaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "root.go", "package a\n")
	writeFile(t, dir, filepath.Join("sub", "nested.go"), "package a\n")
	run("add", ".")
	run("commit", "-m", "init")

	writeFile(t, dir, "root.go", "package a\n// changed\n")
	writeFile(t, dir, filepath.Join("sub", "nested.go"), "package a\n// changed\n")

	res, err := Load(sub)
	if err != nil {
		t.Fatalf("Load from subdir: %v", err)
	}
	if res.Root != dir {
		t.Fatalf("Root = %q, want %q", res.Root, dir)
	}

	rootFile := filepath.Join(dir, "root.go")
	nestedFile := filepath.Join(dir, "sub", "nested.go")
	if res.Files[rootFile] != StatusModified {
		t.Errorf("root.go status = %v, want Modified", res.Files[rootFile])
	}
	if res.Files[nestedFile] != StatusModified {
		t.Errorf("sub/nested.go status = %v, want Modified", res.Files[nestedFile])
	}
	wrongNested := filepath.Join(dir, "nested.go")
	if _, ok := res.Files[wrongNested]; ok {
		t.Errorf("found incorrectly rooted nested path %q", wrongNested)
	}
}

func TestInlineDiffModifiedFile(t *testing.T) {
	dir, run := initTestRepo(t)
	writeFile(t, dir, "a.go", "package a\nfunc old() {}\n")
	run("add", ".")
	run("commit", "-m", "init")

	writeFile(t, dir, "a.go", "package a\nfunc new() {}\n")

	res, err := InlineDiff(filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatalf("InlineDiff: %v", err)
	}
	if !res.HasDiff {
		t.Fatal("expected inline diff")
	}
	assertDiffLine(t, res.Lines, DiffLineContext, "package a")
	assertDiffLine(t, res.Lines, DiffLineDeleted, "func old() {}")
	assertDiffLine(t, res.Lines, DiffLineAdded, "func new() {}")
}

func TestInlineDiffUntrackedFile(t *testing.T) {
	dir, _ := initTestRepo(t)
	writeFile(t, dir, "new.go", "package a\nfunc newFile() {}\n")

	res, err := InlineDiff(filepath.Join(dir, "new.go"))
	if err != nil {
		t.Fatalf("InlineDiff untracked: %v", err)
	}
	if !res.HasDiff {
		t.Fatal("expected inline diff for untracked file")
	}
	if len(res.Lines) != 2 {
		t.Fatalf("untracked diff lines = %d, want 2", len(res.Lines))
	}
	for _, line := range res.Lines {
		if line.Kind != DiffLineAdded {
			t.Fatalf("untracked line kind = %v, want added", line.Kind)
		}
	}
}

func TestInlineDiffStagedAddedFile(t *testing.T) {
	dir, run := initTestRepo(t)
	writeFile(t, dir, "staged.go", "package a\n")
	run("add", "staged.go")

	res, err := InlineDiff(filepath.Join(dir, "staged.go"))
	if err != nil {
		t.Fatalf("InlineDiff staged added: %v", err)
	}
	if !res.HasDiff {
		t.Fatal("expected staged added inline diff")
	}
	assertDiffLine(t, res.Lines, DiffLineAdded, "package a")
}

func TestInlineDiffCleanAndOutsideRepo(t *testing.T) {
	dir, run := initTestRepo(t)
	writeFile(t, dir, "clean.go", "package a\n")
	run("add", ".")
	run("commit", "-m", "init")

	res, err := InlineDiff(filepath.Join(dir, "clean.go"))
	if err != nil {
		t.Fatalf("InlineDiff clean: %v", err)
	}
	if res.HasDiff {
		t.Fatalf("clean file should not have diff: %+v", res)
	}

	outside := t.TempDir()
	writeFile(t, outside, "plain.go", "package a\n")
	res, err = InlineDiff(filepath.Join(outside, "plain.go"))
	if err != nil {
		t.Fatalf("InlineDiff outside repo: %v", err)
	}
	if res.HasDiff {
		t.Fatalf("outside repo should not have diff: %+v", res)
	}
}

func TestInlineDiffDeletedFile(t *testing.T) {
	dir, run := initTestRepo(t)
	writeFile(t, dir, "gone.go", "package a\nfunc gone() {}\n")
	run("add", ".")
	run("commit", "-m", "init")
	if err := os.Remove(filepath.Join(dir, "gone.go")); err != nil {
		t.Fatal(err)
	}

	res, err := InlineDiff(filepath.Join(dir, "gone.go"))
	if err != nil {
		t.Fatalf("InlineDiff deleted: %v", err)
	}
	if !res.HasDiff {
		t.Fatal("expected deleted inline diff")
	}
	assertDiffLine(t, res.Lines, DiffLineDeleted, "func gone() {}")
}

func TestLoadOutsideRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// t.TempDir is not a git repo (and HOME-independent). Load must return empty
	// without error.
	dir := t.TempDir()
	res, err := Load(dir)
	if err != nil {
		t.Fatalf("Load outside repo should not error, got %v", err)
	}
	if len(res.Files) != 0 {
		t.Errorf("expected no files outside repo, got %d", len(res.Files))
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initTestRepo(t *testing.T) (string, func(...string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	return dir, run
}

func assertDiffLine(t *testing.T, lines []DiffLine, kind DiffLineKind, text string) {
	t.Helper()
	for _, line := range lines {
		if line.Kind == kind && line.Text == text {
			return
		}
	}
	t.Fatalf("missing diff line kind=%v text=%q in %+v", kind, text, lines)
}
