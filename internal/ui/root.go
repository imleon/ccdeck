package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"cc-sidecar/internal/session"
)

// focus 表示当前键盘焦点所在的面板。
type focus int

const (
	focusSessions focus = iota
	focusTree
	focusViewer
)

const defaultSessionRefreshInterval = time.Second

// Options controls startup behavior for the TUI.
type Options struct {
	InitialRoot     string
	AltScreen       bool
	SessionSource   session.Source
	RefreshInterval time.Duration
}

type sessionsRefreshTickMsg struct{}

type sessionsRefreshedMsg struct {
	sessions []session.Session
	err      error
}

// RootModel 组合三个子面板，负责布局、焦点切换和全局按键。
type RootModel struct {
	sessions SessionsModel
	tree     TreeModel
	viewer   ViewerModel

	focus  focus
	width  int
	height int

	sessionSource   session.Source
	refreshInterval time.Duration
	refreshInFlight bool

	altScreen bool
	showHelp  bool
	status    string // 底部状态行（如选中 session 的 /resume 提示）
}

// NewRoot 构造根模型。
func NewRoot(sessions []session.Session, opts Options) RootModel {
	sessionsModel := NewSessions(sessions)
	treeModel := NewTree()
	initialRoot := opts.InitialRoot
	if initialRoot == "" {
		initialRoot = sessionsModel.CurrentCWD()
	}
	if initialRoot != "" {
		treeModel = treeModel.SetRoot(initialRoot)
	}
	refreshInterval := opts.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultSessionRefreshInterval
	}
	return RootModel{
		sessions:        sessionsModel,
		tree:            treeModel,
		viewer:          NewViewer(),
		focus:           focusSessions,
		sessionSource:   opts.SessionSource,
		refreshInterval: refreshInterval,
		altScreen:       opts.AltScreen,
	}
}

func (m RootModel) Init() tea.Cmd {
	return m.sessionRefreshTickCmd()
}

func (m RootModel) sessionRefreshTickCmd() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return sessionsRefreshTickMsg{}
	})
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

func (m RootModel) startSessionsRefresh() (RootModel, tea.Cmd) {
	if m.sessionSource == nil || m.refreshInFlight {
		return m, nil
	}
	m.refreshInFlight = true
	return m, scanSessionsCmd(m.sessionSource)
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyPressMsg:
		// 全局键优先
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "tab":
			m.focus = (m.focus + 1) % 3
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			return m, nil
		case "r":
			var cmd tea.Cmd
			m, cmd = m.startSessionsRefresh()
			return m, cmd
		}
		// 其余按键下发给当前焦点面板
		return m.updateFocused(msg)

	case sessionsRefreshTickMsg:
		cmds := []tea.Cmd{m.sessionRefreshTickCmd()}
		var refreshCmd tea.Cmd
		m, refreshCmd = m.startSessionsRefresh()
		if refreshCmd != nil {
			cmds = append(cmds, refreshCmd)
		}
		// 同一节拍顺带刷新中栏目录树与右栏文件：tree 重扫为同步内存操作，
		// viewer 重新读盘为异步 cmd。三栏共享这一条 1s 心跳。
		m.tree = m.tree.Refresh()
		if viewerCmd := m.viewer.Refresh(); viewerCmd != nil {
			cmds = append(cmds, viewerCmd)
		}
		return m, tea.Batch(cmds...)

	case sessionsRefreshedMsg:
		m.refreshInFlight = false
		if msg.err == nil {
			m.sessions = m.sessions.SetSessions(msg.sessions)
		}
		return m, nil

	case treeSelectFileMsg:
		// 树选中文件 → 让 viewer 加载
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		if c := m.viewer.LoadFile(msg.path); c != nil {
			return m, tea.Batch(cmd, c)
		}
		return m, cmd

	case sessionChosenMsg:
		// 左栏 Enter → 状态行显示 cwd + /resume，并把树根切到该 cwd
		m.status = fmt.Sprintf("cd %s  &&  claude --resume %s", msg.cwd, msg.id)
		if msg.cwd != "" {
			m.tree = m.tree.SetRoot(msg.cwd)
		}
		return m, nil

	case loadFileMsg:
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateFocused 把消息下发给当前焦点面板。
func (m RootModel) updateFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusSessions:
		m.sessions, cmd = m.sessions.Update(msg)
	case focusTree:
		m.tree, cmd = m.tree.Update(msg)
	case focusViewer:
		m.viewer, cmd = m.viewer.Update(msg)
	}
	return m, cmd
}

// layout 按当前窗口尺寸分配三栏宽高。
func (m *RootModel) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	// 预留：底部状态行 1；每栏边框 2；每栏内部标题和 footer 各 1。
	bodyH := max(m.height-2, 3)
	contentH := max(panelContentHeight(bodyH)-2, 1)
	// 三栏宽度：左 28% 中 28% 右 44%（减去边框占用）
	lw := m.width * 28 / 100
	tw := m.width * 28 / 100
	vw := m.width - lw - tw
	// 各栏内容区减去边框、左右 padding 和标题/底部信息行。
	m.sessions = m.sessions.SetSize(panelContentWidth(lw), contentH)
	m.tree = m.tree.SetSize(panelContentWidth(tw), contentH)
	m.viewer = m.viewer.SetSize(panelContentWidth(vw), contentH)
}

func (m RootModel) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = m.altScreen
	return v
}

func (m RootModel) render() string {
	if m.width == 0 {
		return "加载中…"
	}
	if m.showHelp {
		return m.helpView()
	}

	bodyH := max(m.height-2, 3)
	lw := m.width * 28 / 100
	tw := m.width * 28 / 100
	vw := m.width - lw - tw

	left := renderPanel(Panel{
		Title:   fmt.Sprintf("Sessions (%d)", m.sessions.Count()),
		Body:    m.sessions.View(),
		Footer:  []string{"/ filter", "r refresh", "Enter/→ open", "← project/collapse"},
		Focused: m.focus == focusSessions,
		Width:   lw,
		Height:  bodyH,
	})
	mid := renderPanel(Panel{
		Title:   "Explorer",
		Body:    m.tree.View(m.viewer.Path()),
		Footer:  []string{defaultString(m.tree.Root(), "no project")},
		Focused: m.focus == focusTree,
		Width:   tw,
		Height:  bodyH,
	})
	viewerFooter := []string{m.viewer.WrapStatus(), "w wrap", "↑/↓ scroll"}
	if rel := m.viewerRelativePath(); rel != "" {
		viewerFooter = append([]string{rel}, viewerFooter...)
	}
	if status := m.viewer.Status(); status != "" {
		viewerFooter = append([]string{status}, viewerFooter...)
	}
	right := renderPanel(Panel{
		Title:   viewerTitle(m.viewer.Path()),
		Body:    m.viewer.View(),
		Footer:  viewerFooter,
		Focused: m.focus == focusViewer,
		Width:   vw,
		Height:  bodyH,
		NoWrap:  true,
	})

	body := joinHorizontal(left, mid, right)
	statusLine := statusStyle.Width(m.width).Render(m.statusText())

	return joinVertical(body, statusLine)
}

func (m RootModel) helpView() string {
	help := `cc-sidecar — 快捷键

  Tab / Shift+Tab   在三个面板间切换焦点
  ↑ / ↓ / j / k     在当前面板内移动
  /                 在 session 列表中过滤
  r                 刷新 session 列表
  Enter / l / →     目录树面板：展开/折叠目录，或在 viewer 中打开文件
  h / ←             目录树面板：折叠当前目录；已折叠则移动到父目录
  Enter             session 面板：显示 /resume 命令并把目录树切到该会话目录
  w                 Viewer 面板：切换不换行 / 按面板宽度换行
  ?                 关闭本帮助
  q / Ctrl+C        退出

按 ? 返回。`
	return helpBoxStyle.Render(help)
}

func panelWithTitle(title, body string) string {
	if title == "" {
		title = "(none)"
	}
	return joinVertical(panelTitleStyle.Render(title), body)
}

func viewerTitle(path string) string {
	if path == "" {
		return "(none)"
	}
	return filepath.Base(path)
}

func shortPath(path string) string {
	if path == "" {
		return "(none)"
	}
	dir := filepath.Base(filepath.Dir(path))
	base := filepath.Base(path)
	if dir == "." || dir == string(filepath.Separator) || dir == "" {
		return base
	}
	return dir + "/" + base
}

func (m RootModel) viewerRelativePath() string {
	path := m.viewer.Path()
	if path == "" {
		return ""
	}
	root := m.tree.Root()
	if root == "" {
		return path
	}
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func (m RootModel) statusText() string {
	if m.status != "" {
		return m.status
	}
	switch m.focus {
	case focusSessions:
		if title := m.sessions.SelectedTitle(); title != "" {
			return fmt.Sprintf("Focus: Sessions · %s · / 过滤 · r 刷新 · Enter 选择 · Tab 切换 · q 退出", title)
		}
		return "Focus: Sessions · / 过滤 · r 刷新 · Enter 选择 · Tab 切换 · q 退出"
	case focusTree:
		return "Focus: Tree · ↑/↓ 移动 · Enter/l 展开或打开 · h 折叠 · Tab 切换 · q 退出"
	case focusViewer:
		if path := m.viewer.Path(); path != "" {
			status := fmt.Sprintf("Focus: Viewer · File: %s", path)
			if viewerStatus := m.viewer.Status(); viewerStatus != "" {
				status += " · " + viewerStatus
			}
			return status + " · " + m.viewer.WrapStatus() + " · w 切换换行 · ↑/↓ 滚动 · Tab 切换 · q 退出"
		}
		return "Focus: Viewer · ↑/↓ 滚动 · Tab 切换 · q 退出"
	default:
		return "Tab 切换面板 · / 过滤 · Enter 选中 · ? 帮助 · q 退出"
	}
}
