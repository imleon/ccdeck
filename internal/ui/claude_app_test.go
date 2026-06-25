package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestClaudeAppActivateChatLogsCommandInsteadOfExecuting(t *testing.T) {
	m := NewClaudeApp(ClaudeAppOptions{GroupName: "dev"})
	model, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Fatal("window resize should not return command")
	}
	model, cmd = model.(ClaudeAppModel).Update(ipcActivateChatMsg{sessionID: "abc", cwd: "/repo"})
	if cmd != nil {
		t.Fatal("activate chat without listener should not return command")
	}
	got := stripANSI(model.(ClaudeAppModel).render())
	if !strings.Contains(got, "未执行") || strings.Contains(got, "Runtime: dev") {
		t.Fatalf("render = %q", got)
	}
	if !strings.Contains(got, "claude --resume abc") {
		t.Fatalf("render = %q", got)
	}
	if !strings.Contains(got, "cd /repo") {
		t.Fatalf("render = %q", got)
	}
}
