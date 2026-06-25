package ui

import (
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/gitstatus"
	"ccdeck/internal/session"
)

const defaultSessionRefreshInterval = time.Second

type sessionsRefreshedMsg struct {
	sessions []session.Session
	err      error
}

type gitStatusRefreshedMsg struct {
	explorerRoot string
	repoRoots    []string
	results      []gitstatus.Result
	err          error
}

func scanSessionsCmd(source session.Source) tea.Cmd {
	if source == nil {
		return nil
	}
	return func() tea.Msg {
		sessions, err := source.List()
		return sessionsRefreshedMsg{sessions: sessions, err: err}
	}
}

func loadGitStatusCmd(explorerRoot string) tea.Cmd {
	return loadGitStatusesCmd(explorerRoot, []string{explorerRoot})
}

func loadGitStatusesCmd(explorerRoot string, repoRoots []string) tea.Cmd {
	if explorerRoot == "" {
		return nil
	}
	cleanExplorerRoot := filepath.Clean(explorerRoot)
	cleanRepoRoots := normalizeRepoRoots(repoRoots)
	return func() tea.Msg {
		results := make([]gitstatus.Result, 0, len(cleanRepoRoots))
		var firstErr error
		for _, repoRoot := range cleanRepoRoots {
			result, err := gitstatus.Load(repoRoot)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				results = append(results, result)
			}
		}
		return gitStatusRefreshedMsg{explorerRoot: cleanExplorerRoot, repoRoots: cleanRepoRoots, results: results, err: firstErr}
	}
}

func normalizeRepoRoots(repoRoots []string) []string {
	seen := make(map[string]struct{}, len(repoRoots))
	result := make([]string, 0, len(repoRoots))
	for _, root := range repoRoots {
		if root == "" {
			continue
		}
		clean := filepath.Clean(root)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	sort.Strings(result)
	return result
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mergeGitStatusResults(explorerRoot string, results []gitstatus.Result) map[string]gitstatus.Status {
	if len(results) == 0 {
		return nil
	}
	cleanExplorerRoot := filepath.Clean(explorerRoot)
	repoRoots := make([]string, 0, len(results))
	byRoot := make(map[string]gitstatus.Result, len(results))
	for _, result := range results {
		if result.Root == "" {
			continue
		}
		root := filepath.Clean(result.Root)
		result.Root = root
		byRoot[root] = result
		repoRoots = append(repoRoots, root)
	}
	sort.Slice(repoRoots, func(i, j int) bool {
		if depth := pathDepth(repoRoots[i]) - pathDepth(repoRoots[j]); depth != 0 {
			return depth < 0
		}
		return repoRoots[i] < repoRoots[j]
	})

	merged := make(map[string]gitstatus.Status)
	for _, root := range repoRoots {
		result := byRoot[root]
		files := filterRepoFiles(result.Files, root, repoRoots)
		overlay := make(map[string]gitstatus.Status, len(files)*2+1)
		maps.Copy(overlay, files)
		maps.Copy(overlay, gitstatus.Aggregate(files, root))
		if root != cleanExplorerRoot {
			if summary := gitstatus.Summarize(files); summary != gitstatus.StatusNone {
				overlay[root] = summary
			}
		}
		maps.Copy(merged, overlay)
	}
	return merged
}

func filterRepoFiles(files map[string]gitstatus.Status, repoRoot string, repoRoots []string) map[string]gitstatus.Status {
	if len(files) == 0 {
		return nil
	}
	filtered := make(map[string]gitstatus.Status, len(files))
	for path, status := range files {
		cleanPath := filepath.Clean(path)
		if isInsideAnyChildRepo(cleanPath, repoRoot, repoRoots) {
			continue
		}
		filtered[cleanPath] = status
	}
	return filtered
}

func isInsideAnyChildRepo(path, repoRoot string, repoRoots []string) bool {
	for _, child := range repoRoots {
		if child == repoRoot || !isPathInside(child, repoRoot) {
			continue
		}
		if isPathInside(path, child) {
			return true
		}
	}
	return false
}

func isPathInside(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func pathDepth(path string) int {
	path = filepath.Clean(path)
	if path == string(filepath.Separator) {
		return 0
	}
	return strings.Count(path, string(filepath.Separator))
}
