package ui

import (
	"path/filepath"
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
	result       gitstatus.Result
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
	if explorerRoot == "" {
		return nil
	}
	cleanExplorerRoot := filepath.Clean(explorerRoot)
	return func() tea.Msg {
		result, err := gitstatus.Load(cleanExplorerRoot)
		return gitStatusRefreshedMsg{explorerRoot: cleanExplorerRoot, result: result, err: err}
	}
}
