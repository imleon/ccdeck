package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
)

// ClaudeAppOptions controls startup behavior for the standalone Claude host pane.
type ClaudeAppOptions struct {
	GroupName   string
	IPCListener *ipc.Listener
}

// ClaudeAppModel runs inside the reserved Claude host pane and reacts to session activation events.
type ClaudeAppModel struct {
	width  int
	height int

	status    string
	body      string
	groupName string

	ipcListener *ipc.Listener
}

func NewClaudeApp(opts ClaudeAppOptions) ClaudeAppModel {
	return ClaudeAppModel{
		groupName:   opts.GroupName,
		ipcListener: opts.IPCListener,
	}
}

func (m ClaudeAppModel) Init() tea.Cmd {
	return waitIPCCmd(m.ipcListener)
}

func (m ClaudeAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case ipcActivateChatMsg:
		m.status = "已记录 Claude 命令，未执行"
		m.body = strings.TrimSpace(fmt.Sprintf(`这个 pane 只记录将执行的 Claude 命令，不会真实执行。

would run:
cd %s && claude --resume %s`, msg.cwd, msg.sessionID))
		return m, waitIPCCmd(m.ipcListener)
	case ipcReceiveErrorMsg:
		m.status = fmt.Sprintf("receive failed: %v", msg.err)
		return m, waitIPCCmd(m.ipcListener)
	}
	return m, nil
}

func (m ClaudeAppModel) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m ClaudeAppModel) render() string {
	if m.width == 0 {
		return "加载中…"
	}
	status := m.status
	if status == "" {
		status = "等待 sessions pane 激活 Claude 命令预览"
	}
	body := m.body
	if body == "" {
		body = "这个 pane 只记录将执行的 Claude 命令，不会真实执行。\n\n当左侧 sessions 选中某个 session 时，这里会打印 would-run 日志，例如：\ncd <cwd> && claude --resume <session-id>"
	}
	return renderStandalonePane("", status, body, m.width)
}
