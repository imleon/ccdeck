package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
)

// ExplorerAppOptions controls startup behavior for the standalone explorer mode.
type ExplorerAppOptions struct {
	GroupName       string
	RefreshInterval time.Duration
	IPCListener     *ipc.Listener
	OpenFileSender  ipc.Sender
	FileSender      ipc.Sender
}

// ExplorerAppModel runs the explorer panel as a standalone TUI.
type ExplorerAppModel struct {
	explorer ExplorerModel

	width  int
	height int

	refreshInterval    time.Duration
	gitStatusInFlight  bool
	gitStatusRepoRoots []string

	showHelp   bool
	status     string
	openedPath string
	groupName  string

	ipcListener    *ipc.Listener
	openFileSender ipc.Sender
	fileSender     ipc.Sender
}

func NewExplorerApp(opts ExplorerAppOptions) ExplorerAppModel {
	refreshInterval := opts.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultSessionRefreshInterval
	}
	return ExplorerAppModel{
		explorer:        NewExplorer(),
		refreshInterval: refreshInterval,
		groupName:       opts.GroupName,
		ipcListener:     opts.IPCListener,
		openFileSender:  opts.OpenFileSender,
		fileSender:      opts.FileSender,
	}
}

func (m ExplorerAppModel) Init() tea.Cmd {
	return tea.Batch(m.refreshTickCmd(), waitIPCCmd(m.ipcListener))
}

func (m ExplorerAppModel) refreshTickCmd() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return appRefreshTickMsg{}
	})
}

func (m ExplorerAppModel) startGitStatusRefresh() (ExplorerAppModel, tea.Cmd) {
	root := m.explorer.Root()
	if root == "" || m.gitStatusInFlight {
		return m, nil
	}
	repoRoots := m.gitStatusTargets()
	m.gitStatusInFlight = true
	m.gitStatusRepoRoots = repoRoots
	return m, loadGitStatusesCmd(root, repoRoots)
}

func (m ExplorerAppModel) gitStatusTargets() []string {
	root := m.explorer.Root()
	if root == "" {
		return nil
	}
	return normalizeRepoRoots(append([]string{root}, m.explorer.VisibleGitRepoRoots()...))
}

func (m ExplorerAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.explorer = m.explorer.Refresh()
			var cmd tea.Cmd
			m, cmd = m.startGitStatusRefresh()
			return m, cmd
		}
		var cmd tea.Cmd
		m.explorer, cmd = m.explorer.Update(msg)
		return m, cmd
	case appRefreshTickMsg:
		cmds := []tea.Cmd{m.refreshTickCmd()}
		m.explorer = m.explorer.Refresh()
		var gitCmd tea.Cmd
		m, gitCmd = m.startGitStatusRefresh()
		if gitCmd != nil {
			cmds = append(cmds, gitCmd)
		}
		return m, tea.Batch(cmds...)
	case gitStatusRefreshedMsg:
		m.gitStatusInFlight = false
		if filepath.Clean(msg.explorerRoot) != filepath.Clean(m.explorer.Root()) {
			var cmd tea.Cmd
			m, cmd = m.startGitStatusRefresh()
			return m, cmd
		}
		if !sameStrings(msg.repoRoots, m.gitStatusTargets()) {
			var cmd tea.Cmd
			m, cmd = m.startGitStatusRefresh()
			return m, cmd
		}
		m.explorer = m.explorer.SetGitStatusMap(mergeGitStatusResults(msg.explorerRoot, msg.results))
		return m, nil
	case explorerOpenFileMsg:
		m.openedPath = msg.path
		return m, sendOpenFileCmd(m.openFileSender, msg.path, m.explorer.Root())
	case ipcSetRootMsg:
		return m.applyLinkedRoot(msg.path)
	case ipcReceiveErrorMsg:
		m.status = fmt.Sprintf("receive failed: %v", msg.err)
		return m, waitIPCCmd(m.ipcListener)
	case ipcSendResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s is not running: %v", msg.target, msg.err)
		}
		return m, nil
	}
	return m, nil
}

func (m ExplorerAppModel) applyLinkedRoot(path string) (tea.Model, tea.Cmd) {
	if path == "" {
		return m, waitIPCCmd(m.ipcListener)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		m.status = fmt.Sprintf("linked root unavailable: %s", path)
		return m, waitIPCCmd(m.ipcListener)
	}
	if filepath.Clean(path) == filepath.Clean(m.explorer.Root()) {
		m.openedPath = ""
		return m, waitIPCCmd(m.ipcListener)
	}
	m.explorer = m.explorer.SetRoot(path)
	m.openedPath = ""
	m.status = ""
	var gitCmd tea.Cmd
	m, gitCmd = m.startGitStatusRefresh()
	return m, tea.Batch(waitIPCCmd(m.ipcListener), gitCmd)
}

func (m *ExplorerAppModel) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	contentW := paneContentWidth(m.width, standaloneScrollbarBodyPadding)
	m.explorer = m.explorer.SetSize(contentW, max(m.height, 1))
}

func (m ExplorerAppModel) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m ExplorerAppModel) render() string {
	if m.width == 0 {
		return "加载中…"
	}
	if m.showHelp {
		return m.helpView()
	}
	return renderStandalonePanePadded(m.status, m.explorer.View(m.openedPath), m.width, standaloneScrollbarBodyPadding)
}

func (m ExplorerAppModel) helpView() string {
	help := `Explorer pane — 快捷键

接收 sessions pane 推送的项目根目录，并把选中的文件发送到 file pane

  ↑ / ↓ / j / k     移动选择
  Enter / l / →     展开目录；在文件上选择路径
  h / ←             折叠当前目录；已折叠则移动到父目录
  r                 刷新 explorer 和 git status
  ?                 关闭本帮助
  q / Ctrl+C        退出

按 ? 返回。`
	return helpBoxStyle.Render(help)
}

func (m ExplorerAppModel) statusText() string {
	if m.status != "" {
		return m.status
	}
	if root := m.explorer.Root(); root != "" {
		return projectDirLabel(root)
	}
	return "waiting for active session"
}
