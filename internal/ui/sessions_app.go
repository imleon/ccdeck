package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
	"ccdeck/internal/session"
)

type appRefreshTickMsg struct{}

// SessionsAppOptions controls startup behavior for the standalone sessions mode.
type SessionsAppOptions struct {
	SessionSource   session.Source
	RefreshInterval time.Duration
	GroupName       string
	SetRootSender   ipc.Sender
	ClaudeSender    ipc.Sender
	IPCPresence     *ipc.Listener
}

// SessionsAppModel runs the sessions panel as a standalone TUI.
type SessionsAppModel struct {
	sessions SessionsModel

	width  int
	height int

	sessionSource   session.Source
	refreshInterval time.Duration
	refreshInFlight bool

	showHelp bool
	status   string

	groupName     string
	setRootSender ipc.Sender
	claudeSender  ipc.Sender
	ipcPresence   *ipc.Listener
}

func NewSessionsApp(sessions []session.Session, opts SessionsAppOptions) SessionsAppModel {
	refreshInterval := opts.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultSessionRefreshInterval
	}
	return SessionsAppModel{
		sessions:        NewSessions(sessions),
		sessionSource:   opts.SessionSource,
		refreshInterval: refreshInterval,
		groupName:       opts.GroupName,
		setRootSender:   opts.SetRootSender,
		claudeSender:    opts.ClaudeSender,
		ipcPresence:     opts.IPCPresence,
	}
}

func (m SessionsAppModel) Init() tea.Cmd {
	return m.refreshTickCmd()
}

func (m SessionsAppModel) refreshTickCmd() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return appRefreshTickMsg{}
	})
}

func (m SessionsAppModel) startSessionsRefresh() (SessionsAppModel, tea.Cmd) {
	if m.sessionSource == nil || m.refreshInFlight {
		return m, nil
	}
	m.refreshInFlight = true
	return m, scanSessionsCmd(m.sessionSource)
}

func (m SessionsAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "r":
			var cmd tea.Cmd
			m, cmd = m.startSessionsRefresh()
			return m, cmd
		}
		var cmd tea.Cmd
		m.sessions, cmd = m.sessions.Update(msg)
		return m, cmd
	case appRefreshTickMsg:
		cmds := []tea.Cmd{m.refreshTickCmd()}
		var refreshCmd tea.Cmd
		m, refreshCmd = m.startSessionsRefresh()
		if refreshCmd != nil {
			cmds = append(cmds, refreshCmd)
		}
		return m, tea.Batch(cmds...)
	case sessionsRefreshedMsg:
		m.refreshInFlight = false
		if msg.err == nil {
			m.sessions = m.sessions.SetSessions(msg.sessions)
		}
		return m, nil
	case sessionChosenMsg:
		m.sessions = m.sessions.SetActiveSession(msg.id)
		m.status = ""
		return m, tea.Batch(
			sendSetRootCmd(m.setRootSender, msg.projectDir, msg.id),
			sendActivateChatCmd(m.claudeSender, msg.id, msg.cwd),
			sendClearFileCmd(m.setRootSender, msg.projectDir),
		)
	case ipcSendResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s is not running: %v", msg.target, msg.err)
		}
		return m, nil
	}
	return m, nil
}

func (m *SessionsAppModel) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	contentW, contentH := standalonePaddedContentSize(m.width, m.height, standaloneScrollbarBodyPadding)
	m.sessions = m.sessions.SetSize(contentW, contentH)
}

func (m SessionsAppModel) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m SessionsAppModel) render() string {
	if m.width == 0 {
		return "加载中…"
	}
	if m.showHelp {
		return m.helpView()
	}
	return renderStandalonePanePadded(m.statusText(), m.sessions.View(), m.width, standaloneScrollbarBodyPadding)
}

func (m SessionsAppModel) helpView() string {
	help := `Sessions pane — 快捷键

  ↑ / ↓ / j / k     移动选择
  /                 过滤 sessions
  r                 刷新 sessions
  Enter / l / →     激活 session，并联动 Claude / explorer / file panes
  h / ←             回到项目分组或折叠分组
  ?                 关闭本帮助
  q / Ctrl+C        退出

按 ? 返回。`
	return helpBoxStyle.Render(help)
}

func (m SessionsAppModel) statusText() string {
	if m.status != "" {
		return m.status
	}
	return ""
}
