// Package gitstatus reports per-file git status for a working tree by shelling
// out to the git binary. It deliberately uses `git` rather than a pure-Go
// implementation: `git status --porcelain` reuses git's index/untracked caches
// (and fsmonitor when configured), and the same binary will later back a diff
// view via `git diff` — both far cheaper and simpler through the CLI.
package gitstatus

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Status is the display-ready git state for a single path.
type Status int

const (
	StatusNone      Status = iota // tracked and clean, or outside any change
	StatusModified                // content changed (staged or unstaged)
	StatusAdded                   // newly staged file
	StatusDeleted                 // removed
	StatusRenamed                 // renamed (or copied)
	StatusUntracked               // not tracked by git
	StatusConflict                // merge conflict / unmerged
)

// Mark returns the single-character badge shown at the row's right edge.
func (s Status) Mark() string {
	switch s {
	case StatusModified:
		return "M"
	case StatusAdded:
		return "A"
	case StatusDeleted:
		return "D"
	case StatusRenamed:
		return "R"
	case StatusUntracked:
		return "A"
	case StatusConflict:
		return "U"
	default:
		return ""
	}
}

// Result holds the parsed status for one working tree.
type Result struct {
	Root  string            // absolute path to the repository worktree root
	Files map[string]Status // absolute file path -> status
}

// DiffLineKind classifies one rendered inline-diff line.
type DiffLineKind int

const (
	DiffLineContext DiffLineKind = iota
	DiffLineAdded
	DiffLineDeleted
)

// DiffLine is one source-like line for the viewer's inline git changes view.
type DiffLine struct {
	Kind DiffLineKind
	Text string
}

// InlineDiffResult is a single-file, source-shaped diff for the viewer.
type InlineDiffResult struct {
	Root    string
	RelPath string
	Status  Status
	Lines   []DiffLine
	HasDiff bool
}

// defaultTimeout bounds each git invocation so a wedged repo can never stall
// the caller (the UI polls this on a 1s tick).
const defaultTimeout = 3 * time.Second

// Load resolves the worktree root containing dir and returns the status of every
// changed/untracked path, keyed by absolute path. If dir is not inside a git
// repository, it returns an empty result and a nil error — "no git" is normal,
// not an error condition for the UI.
func Load(dir string) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	root, err := repoRoot(ctx, dir)
	if err != nil || root == "" {
		return Result{Files: map[string]Status{}}, nil
	}

	out, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return Result{Root: root, Files: map[string]Status{}}, err
	}

	rel := parsePorcelainZ(out)
	files := make(map[string]Status, len(rel))
	for p, st := range rel {
		files[filepath.Join(root, p)] = st
	}
	return Result{Root: root, Files: files}, nil
}

// InlineDiff returns a source-shaped diff for one path. Clean files, paths
// outside git, and paths outside the containing repo return HasDiff=false with a
// nil error so callers can fall back to normal file viewing.
func InlineDiff(path string) (InlineDiffResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	workDir := abs
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		workDir = filepath.Dir(abs)
	} else if err != nil {
		workDir = filepath.Dir(abs)
	}

	root, err := repoRoot(ctx, workDir)
	if err != nil || root == "" {
		return InlineDiffResult{}, nil
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return InlineDiffResult{}, nil
	}

	status, err := statusForPath(ctx, root, rel)
	if err != nil {
		return InlineDiffResult{Root: root, RelPath: rel}, err
	}
	result := InlineDiffResult{Root: root, RelPath: rel, Status: status}
	if status == StatusNone {
		return result, nil
	}
	if status == StatusUntracked {
		lines, err := addedFileLines(abs)
		if err != nil {
			return result, err
		}
		result.Lines = lines
		result.HasDiff = len(lines) > 0
		return result, nil
	}

	out, err := runGit(ctx, root, "diff", "--no-ext-diff", "--no-color", "--unified=1000000", "--", rel)
	if err != nil {
		return result, err
	}
	if len(out) == 0 {
		out, err = runGit(ctx, root, "diff", "--cached", "--no-ext-diff", "--no-color", "--unified=1000000", "--", rel)
		if err != nil {
			return result, err
		}
	}
	if len(out) == 0 {
		return result, nil
	}
	result.Lines = parseInlineDiff(out)
	result.HasDiff = len(result.Lines) > 0
	return result, nil
}

// repoRoot returns the absolute worktree root for dir, or "" if dir is not in a
// git repository.
func repoRoot(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil // not a repo (or git missing) — treat as no status
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil // discard; we only care about success + stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

func statusForPath(ctx context.Context, root, rel string) (Status, error) {
	out, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", rel)
	if err != nil {
		return StatusNone, err
	}
	statuses := parsePorcelainZ(out)
	if st, ok := statuses[rel]; ok {
		return st, nil
	}
	return StatusNone, nil
}

func addedFileLines(path string) ([]DiffLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSuffix(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	parts := strings.Split(content, "\n")
	lines := make([]DiffLine, 0, len(parts))
	for _, line := range parts {
		lines = append(lines, DiffLine{Kind: DiffLineAdded, Text: line})
	}
	return lines, nil
}

// parsePorcelainZ parses `git status --porcelain=v1 -z` output into a map of
// repo-relative path -> Status.
//
// In -z mode records are NUL-separated. Each record is "XY <path>" where X is
// the staged state and Y the worktree state. Rename/copy records (X is R or C)
// carry a second NUL-terminated field — the original path — which we consume
// and ignore, keeping only the new path.
func parsePorcelainZ(data []byte) map[string]Status {
	result := make(map[string]Status)
	fields := strings.Split(string(data), "\x00")
	for i := 0; i < len(fields); i++ {
		rec := fields[i]
		if len(rec) < 3 {
			continue
		}
		x, y := rec[0], rec[1]
		path := rec[3:] // skip "XY " (two codes + one space)

		// Rename/copy: the next field is the original path. Consume it.
		if x == 'R' || x == 'C' {
			i++
		}

		result[path] = classify(x, y)
	}
	return result
}

func parseInlineDiff(data []byte) []DiffLine {
	patchLines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	lines := make([]DiffLine, 0, len(patchLines))
	for _, line := range patchLines {
		if line == "" || strings.HasPrefix(line, "\\ No newline") {
			continue
		}
		if isPatchMetadata(line) {
			continue
		}
		switch line[0] {
		case ' ':
			lines = append(lines, DiffLine{Kind: DiffLineContext, Text: line[1:]})
		case '+':
			lines = append(lines, DiffLine{Kind: DiffLineAdded, Text: line[1:]})
		case '-':
			lines = append(lines, DiffLine{Kind: DiffLineDeleted, Text: line[1:]})
		}
	}
	return lines
}

func isPatchMetadata(line string) bool {
	return strings.HasPrefix(line, "diff --git ") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ") ||
		strings.HasPrefix(line, "@@ ") ||
		strings.HasPrefix(line, "old mode ") ||
		strings.HasPrefix(line, "new mode ") ||
		strings.HasPrefix(line, "deleted file mode ") ||
		strings.HasPrefix(line, "new file mode ") ||
		strings.HasPrefix(line, "similarity index ") ||
		strings.HasPrefix(line, "rename from ") ||
		strings.HasPrefix(line, "rename to ")
}

// classify maps the two porcelain status codes to a single display status.
func classify(x, y byte) Status {
	// Unmerged paths: any side is U, or the symmetric DD/AA pairs.
	if x == 'U' || y == 'U' || (x == 'D' && y == 'D') || (x == 'A' && y == 'A') {
		return StatusConflict
	}
	if x == '?' && y == '?' {
		return StatusUntracked
	}
	// Prefer the staged code; fall back to the worktree code. This collapses
	// "staged + further unstaged edits" to the staged intent for display.
	code := x
	if code == ' ' || code == 0 {
		code = y
	}
	switch code {
	case 'A':
		return StatusAdded
	case 'D':
		return StatusDeleted
	case 'R', 'C':
		return StatusRenamed
	case 'M', 'T':
		return StatusModified
	default:
		return StatusModified
	}
}

// Summarize returns one aggregate status for a set of files. A single kind of
// descendant change keeps that status, multiple non-conflict kinds become
// modified, and conflict dominates everything.
func Summarize(files map[string]Status) Status {
	seen := StatusNone
	status := StatusNone
	for _, st := range files {
		if st == StatusNone {
			continue
		}
		switch {
		case status == StatusConflict:
		case st == StatusConflict:
			status = StatusConflict
		case status == StatusModified:
		case seen == StatusNone:
			seen = st
			status = st
		case seen != st:
			status = StatusModified
		}
	}
	return status
}

// Aggregate folds file statuses up into their ancestor directories. Files keep
// their exact git state. A directory with one kind of descendant change shows
// that status, a directory with multiple non-conflict change kinds shows
// modified, and conflict dominates everything. The returned map is keyed by
// absolute directory path; root itself is never included (the tree renders
// root's children, not root).
//
// files is the per-file map (absolute paths). root bounds the walk so we never
// climb above the repository.
func Aggregate(files map[string]Status, root string) map[string]Status {
	type aggregateState struct {
		status Status
		seen   Status
	}

	states := make(map[string]aggregateState)
	if root == "" {
		return map[string]Status{}
	}
	cleanRoot := filepath.Clean(root)
	for path, st := range files {
		if st == StatusNone {
			continue
		}
		dir := filepath.Dir(path)
		for {
			if dir == cleanRoot || !strings.HasPrefix(dir, cleanRoot) {
				break
			}
			state := states[dir]
			switch {
			case state.status == StatusConflict:
			case st == StatusConflict:
				state.status = StatusConflict
			case state.status == StatusModified:
			case state.seen == StatusNone:
				state.seen = st
				state.status = st
			case state.seen != st:
				state.status = StatusModified
			}
			states[dir] = state

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	dirs := make(map[string]Status, len(states))
	for dir, state := range states {
		if state.status != StatusNone {
			dirs[dir] = state.status
		}
	}
	return dirs
}
