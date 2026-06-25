package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
)

// FileAppOptions controls startup behavior for the standalone file mode.
type FileAppOptions struct {
	GroupName       string
	RefreshInterval time.Duration
	IPCListener     *ipc.Listener
}

// FileAppModel runs the file panel as a standalone TUI.
type FileAppModel struct {
	file FileModel

	width  int
	height int

	refreshInterval time.Duration

	showHelp  bool
	status    string
	groupName string

	ipcListener *ipc.Listener
}

func NewFileApp(opts FileAppOptions) FileAppModel {
	refreshInterval := opts.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultSessionRefreshInterval
	}
	return FileAppModel{
		file:            NewFile(),
		refreshInterval: refreshInterval,
		groupName:       opts.GroupName,
		ipcListener:     opts.IPCListener,
	}
}

func (m FileAppModel) Init() tea.Cmd {
	return tea.Batch(m.refreshTickCmd(), waitIPCCmd(m.ipcListener))
}

func (m FileAppModel) refreshTickCmd() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return appRefreshTickMsg{}
	})
}

func (m FileAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.file, cmd = m.file.Refresh()
			return m, cmd
		}
		var cmd tea.Cmd
		m.file, cmd = m.file.Update(msg)
		return m, cmd
	case appRefreshTickMsg:
		cmds := []tea.Cmd{m.refreshTickCmd()}
		var refreshCmd tea.Cmd
		m.file, refreshCmd = m.file.Refresh()
		if refreshCmd != nil {
			cmds = append(cmds, refreshCmd)
		}
		return m, tea.Batch(cmds...)
	case loadFileMsg:
		var cmd tea.Cmd
		m.file, cmd = m.file.Update(msg)
		return m, cmd
	case ipcOpenFileMsg:
		var cmd tea.Cmd
		m.file, cmd = m.file.LoadPath(msg.path)
		m.status = fmt.Sprintf("linked file: %s", msg.path)
		return m, tea.Batch(waitIPCCmd(m.ipcListener), cmd)
	case ipcClearFileMsg:
		m.file = m.file.Clear()
		m.status = "waiting for file selection"
		return m, waitIPCCmd(m.ipcListener)
	case ipcReceiveErrorMsg:
		m.status = fmt.Sprintf("receive failed: %v", msg.err)
		return m, waitIPCCmd(m.ipcListener)
	}
	return m, nil
}

func (m *FileAppModel) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	contentW, contentH := standalonePaddedContentSize(m.width, m.height, standaloneScrollbarBodyPadding)
	m.file = m.file.SetSize(contentW, contentH)
}

func (m FileAppModel) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m FileAppModel) render() string {
	if m.width == 0 {
		return "加载中…"
	}
	if m.showHelp {
		return m.helpView()
	}
	return renderStandalonePanePadded(m.statusText(), m.file.View(), m.width, standaloneScrollbarBodyPadding)
}

func (m FileAppModel) helpView() string {
	help := `File pane — 快捷键

接收 explorer pane 推送的文件路径；切换 session 时会自动清空旧文件

  ↑ / ↓ / j / k     滚动文件
  w                 切换不换行 / 按面板宽度换行
  r                 重新读取当前文件
  ?                 关闭本帮助
  q / Ctrl+C        退出

按 ? 返回。`
	return helpBoxStyle.Render(help)
}

func (m FileAppModel) statusText() string {
	if m.status != "" {
		return m.status
	}
	if path := m.file.Path(); path != "" {
		status := path
		if fileStatus := m.file.Status(); fileStatus != "" {
			status += " · " + fileStatus
		}
		return status + " · " + m.file.WrapStatus()
	}
	return "waiting for file selection"
}
