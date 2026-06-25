package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
	"ccdeck/internal/session"
)

type fakeSessionSource struct {
	sessions []session.Session
	err      error
	calls    int
}

func (s *fakeSessionSource) List() ([]session.Session, error) {
	s.calls++
	return s.sessions, s.err
}

func TestSessionsAppSessionChosenClearsStatus(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{RefreshInterval: time.Hour})
	m.status = "previous error"
	model, cmd := m.Update(sessionChosenMsg{cwd: "/repo/sub", projectDir: "/repo", id: "abc"})
	if cmd != nil {
		t.Fatal("session chosen should not return command")
	}
	got := model.(SessionsAppModel)
	if got.statusText() != "" {
		t.Fatalf("status = %q, want empty", got.statusText())
	}
}

func TestSessionsAppSessionChosenWithSenderReturnsCommand(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{GroupName: "dev", RefreshInterval: time.Hour, SetRootSender: ipc.Sender{GroupName: "missing"}})
	_, cmd := m.Update(sessionChosenMsg{cwd: "/repo/sub", projectDir: "/repo", id: "abc"})
	if cmd == nil {
		t.Fatal("linked session selection should return send command")
	}
}

func TestSessionsAppSessionChosenWithMissingProjectDirReportsExplorerError(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{GroupName: "dev", RefreshInterval: time.Hour, SetRootSender: ipc.Sender{GroupName: "missing"}})
	_, cmd := m.Update(sessionChosenMsg{id: "abc"})
	if cmd == nil {
		t.Fatal("missing project dir should still report why explorer was not linked")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("command emitted %T, want non-empty tea.BatchMsg", msg)
	}
	result, ok := batch[0]().(ipcSendResultMsg)
	if !ok {
		t.Fatalf("first batch command emitted %T, want ipcSendResultMsg", batch[0]())
	}
	if result.err == nil || !strings.Contains(result.err.Error(), "no project dir") {
		t.Fatalf("error = %v, want no project dir", result.err)
	}
}

func TestSessionsAppRefresh(t *testing.T) {
	source := &fakeSessionSource{sessions: []session.Session{{ID: "new", CWD: "/repo", Title: "new"}}}
	m := NewSessionsApp([]session.Session{{ID: "new", CWD: "/repo", Title: "old"}}, SessionsAppOptions{SessionSource: source, RefreshInterval: time.Hour})
	model, cmd := m.Update(sessionsKey("r"))
	got := model.(SessionsAppModel)
	if cmd == nil {
		t.Fatal("refresh should return command")
	}
	if !got.refreshInFlight {
		t.Fatal("refresh should mark in-flight")
	}

	model, cmd = got.Update(sessionsRefreshedMsg{sessions: source.sessions})
	got = model.(SessionsAppModel)
	if cmd != nil {
		t.Fatal("sessions refreshed should not return command")
	}
	if got.refreshInFlight {
		t.Fatal("refresh should clear in-flight")
	}
	if got.sessions.Count() != 1 {
		t.Fatalf("session count = %d, want 1", got.sessions.Count())
	}
}

func TestSessionsAppStandaloneLayoutUsesInsetBodyWidth(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := model.(SessionsAppModel)
	if got.sessions.width != 79 {
		t.Fatalf("sessions width = %d, want 79", got.sessions.width)
	}
	if got.sessions.height != 23 {
		t.Fatalf("sessions height = %d, want 23", got.sessions.height)
	}
}

func TestSessionsAppStandaloneRenderHasNoPanelBorder(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{GroupName: "dev", RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rendered := model.(SessionsAppModel).render()
	plain := stripANSI(rendered)
	if strings.Contains(plain, "Sessions") || strings.Contains(plain, "Group: dev") {
		t.Fatalf("render should not repeat pane title or group: %q", plain)
	}
	if strings.ContainsAny(plain, "╭╮╰╯") {
		t.Fatalf("standalone render should not contain panel border: %q", plain)
	}
}

func TestSessionsAppHelpView(t *testing.T) {
	m := NewSessionsApp(nil, SessionsAppOptions{RefreshInterval: time.Hour})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = model.(SessionsAppModel).Update(sessionsKey("?"))
	got := model.(SessionsAppModel).render()
	if !strings.Contains(got, "Sessions pane") {
		t.Fatalf("help view = %q", got)
	}
}
