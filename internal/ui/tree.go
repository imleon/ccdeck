package ui

import (
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"ccdeck/internal/gitstatus"
)

// treeNode 是目录树里的一个节点（文件或目录）。
type treeNode struct {
	name     string // 显示名（basename）
	path     string // 绝对路径
	isDir    bool
	depth    int
	expanded bool
}

// treeSelectFileMsg 在树里选中一个文件时发出，由 Explorer 转为产品层消息。
type treeSelectFileMsg struct {
	path string
}

// TreeModel renders a directory tree rooted at a configured directory.
type TreeModel struct {
	root   string     // 根目录
	nodes  []treeNode // 当前展开后可见的扁平节点列表
	cursor int
	width  int
	height int
	offset int // 滚动偏移

	// gitStatus 合并了文件精确状态与目录聚合状态，键为绝对路径。由 root
	// 注入，渲染时按节点 path 查表。nil/缺失表示干净或无 git。
	gitStatus map[string]gitstatus.Status
}

func NewTree() TreeModel {
	return TreeModel{}
}

// SetGitStatus 注入最新的 git 状态（文件级 + 目录聚合）。root 在每次
// git 状态刷新完成时调用。传入 nil 等价于清空（无 git 仓库时）。
func (m TreeModel) SetGitStatus(files map[string]gitstatus.Status, root string) TreeModel {
	if len(files) == 0 {
		m.gitStatus = nil
		return m
	}
	merged := make(map[string]gitstatus.Status, len(files)*2)
	maps.Copy(merged, files)
	maps.Copy(merged, gitstatus.Aggregate(files, root))
	m.gitStatus = merged
	return m
}

func (m TreeModel) SetGitStatusMap(status map[string]gitstatus.Status) TreeModel {
	if len(status) == 0 {
		m.gitStatus = nil
		return m
	}
	m.gitStatus = maps.Clone(status)
	return m
}

// Root returns the current tree root directory for titles/status display.
func (m TreeModel) Root() string {
	return m.root
}

// SetRoot 切换根目录，并把根目录本身作为第一行。
func (m TreeModel) SetRoot(dir string) TreeModel {
	if filepath.Clean(dir) != filepath.Clean(m.root) {
		m.gitStatus = nil
	}
	m.root = dir
	m.cursor = 0
	m.offset = 0
	m.nodes = buildVisibleNodes(dir, 0, map[string]bool{dir: true})
	return m
}

// Refresh 重扫当前根目录，反映磁盘上的文件增删，同时保留已展开的目录、
// 光标所在节点和滚动位置。用于自动刷新轮询。
func (m TreeModel) Refresh() TreeModel {
	if m.root == "" {
		return m
	}

	// 记录当前展开的目录 path 与光标所在节点 path。
	expanded := make(map[string]bool, len(m.nodes))
	for _, n := range m.nodes {
		if n.isDir && n.expanded {
			expanded[n.path] = true
		}
	}
	cursorPath := ""
	if m.cursor >= 0 && m.cursor < len(m.nodes) {
		cursorPath = m.nodes[m.cursor].path
	}

	// 从根重建扁平树，递归展开此前展开的目录。
	m.nodes = buildVisibleNodes(m.root, 0, expanded)

	// 把光标定位回原节点；找不到则交给 clampScroll 兜底。
	if cursorPath != "" {
		for i, n := range m.nodes {
			if n.path == cursorPath {
				m.cursor = i
				break
			}
		}
	}
	return m.clampScroll()
}

// buildVisibleNodes 从 dir 递归构建可见的扁平节点列表：包含 dir 自身，
// 目录仅当其 path 在 expanded 集合中时展开并递归其子项。
func buildVisibleNodes(dir string, depth int, expanded map[string]bool) []treeNode {
	cleanDir := filepath.Clean(dir)
	name := filepath.Base(cleanDir)
	if name == "." || name == string(filepath.Separator) {
		name = cleanDir
	}
	node := treeNode{
		name:     name,
		path:     dir,
		isDir:    true,
		depth:    depth,
		expanded: expanded[dir],
	}
	result := []treeNode{node}
	if !node.expanded {
		return result
	}
	for _, child := range readDir(dir, depth+1) {
		if child.isDir {
			result = append(result, buildVisibleNodes(child.path, depth+1, expanded)...)
			continue
		}
		result = append(result, child)
	}
	return result
}

// SetSize 设置面板尺寸。
func (m TreeModel) SetSize(w, h int) TreeModel {
	m.width = w
	m.height = h
	return m
}

func (m TreeModel) VisibleGitRepoRoots() []string {
	roots := make(map[string]struct{})
	for _, n := range m.nodes {
		if !n.isDir || !hasGitMarker(n.path) {
			continue
		}
		roots[filepath.Clean(n.path)] = struct{}{}
	}
	result := make([]string, 0, len(roots))
	for root := range roots {
		result = append(result, root)
	}
	sort.Strings(result)
	return result
}

func hasGitMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// readDir 读取一个目录的直接子项，返回排序后的节点（目录在前）。
func readDir(dir string, depth int) []treeNode {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs, files []treeNode
	for _, e := range entries {
		name := e.Name()
		if name == ".git" {
			continue // 隐藏 Git 元数据目录，保留其它 dot 项可见。
		}
		n := treeNode{
			name:  name,
			path:  filepath.Join(dir, name),
			isDir: e.IsDir(),
			depth: depth,
		}
		if e.IsDir() {
			dirs = append(dirs, n)
		} else {
			files = append(files, n)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return append(dirs, files...)
}

type treeIconMode string

const (
	treeIconModeNerd  treeIconMode = "nerd"
	treeIconModeASCII treeIconMode = "ascii"

	currentTreeIconMode    = treeIconModeNerd
	treeIndentWidth        = 2
	treeTwistyWidth        = 2
	treeIconWidth          = 2
	treeGitMarkColumnWidth = 2
)

type treeIconKind string

const (
	iconFolder     treeIconKind = "folder"
	iconFolderOpen treeIconKind = "folder_open"
	iconFile       treeIconKind = "file"
	iconGo         treeIconKind = "go"
	iconMarkdown   treeIconKind = "markdown"
	iconJSON       treeIconKind = "json"
	iconYAML       treeIconKind = "yaml"
	iconTOML       treeIconKind = "toml"
	iconShell      treeIconKind = "shell"
	iconJavaScript treeIconKind = "javascript"
	iconTypeScript treeIconKind = "typescript"
	iconHTML       treeIconKind = "html"
	iconCSS        treeIconKind = "css"
	iconImage      treeIconKind = "image"
	iconLock       treeIconKind = "lock"
	iconDocker     treeIconKind = "docker"
	iconMake       treeIconKind = "make"
	iconText       treeIconKind = "text"
)

func treeLinePrefix(n treeNode) string {
	indent := strings.Repeat(" ", n.depth*treeIndentWidth)
	twisty := padCell(treeTwisty(n), treeTwistyWidth)
	icon := padCell(treeIcon(n), treeIconWidth)
	return indent + twisty + icon
}

func treeTwisty(n treeNode) string {
	if !n.isDir {
		return ""
	}
	if n.expanded {
		return "⌄"
	}
	return "›"
}

func treeIcon(n treeNode) string {
	kind := treeIconKindForNode(n)
	if currentTreeIconMode == treeIconModeASCII {
		return treeASCIIIcon(kind)
	}
	return treeNerdIcon(kind)
}

func treeIconKindForNode(n treeNode) treeIconKind {
	if n.isDir {
		if n.expanded {
			return iconFolderOpen
		}
		return iconFolder
	}

	name := strings.ToLower(n.name)
	switch name {
	case "dockerfile", "docker-compose.yml", "docker-compose.yaml":
		return iconDocker
	case "makefile", "gnumakefile":
		return iconMake
	case "go.mod", "go.sum", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb":
		return iconLock
	case "readme", "readme.md", "claude.md":
		return iconMarkdown
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".go":
		return iconGo
	case ".md", ".markdown":
		return iconMarkdown
	case ".json", ".jsonl", ".ndjson":
		return iconJSON
	case ".yaml", ".yml":
		return iconYAML
	case ".toml", ".ini", ".conf", ".config":
		return iconTOML
	case ".sh", ".bash", ".zsh", ".fish":
		return iconShell
	case ".js", ".jsx", ".mjs", ".cjs":
		return iconJavaScript
	case ".ts", ".tsx", ".mts", ".cts":
		return iconTypeScript
	case ".html", ".htm":
		return iconHTML
	case ".css", ".scss", ".sass", ".less":
		return iconCSS
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico":
		return iconImage
	case ".txt", ".log":
		return iconText
	default:
		return iconFile
	}
}

func treeNerdIcon(kind treeIconKind) string {
	switch kind {
	case iconFolder:
		return ""
	case iconFolderOpen:
		return ""
	case iconGo:
		return ""
	case iconMarkdown:
		return ""
	case iconJSON:
		return ""
	case iconYAML, iconTOML:
		return ""
	case iconShell:
		return ""
	case iconJavaScript:
		return ""
	case iconTypeScript:
		return ""
	case iconHTML:
		return ""
	case iconCSS:
		return ""
	case iconImage:
		return ""
	case iconLock:
		return ""
	case iconDocker:
		return ""
	case iconMake:
		return ""
	case iconText:
		return "󰈙"
	default:
		return ""
	}
}

func treeASCIIIcon(kind treeIconKind) string {
	switch kind {
	case iconFolder, iconFolderOpen:
		return "d"
	case iconGo:
		return "go"
	case iconMarkdown:
		return "md"
	case iconJSON:
		return "js"
	case iconYAML:
		return "yml"
	case iconTOML:
		return "tom"
	case iconShell:
		return "sh"
	case iconJavaScript:
		return "js"
	case iconTypeScript:
		return "ts"
	case iconHTML:
		return "htm"
	case iconCSS:
		return "css"
	case iconImage:
		return "img"
	case iconLock:
		return "lock"
	case iconDocker:
		return "dk"
	case iconMake:
		return "mk"
	case iconText:
		return "txt"
	default:
		return "·"
	}
}

// toggle 展开/折叠光标所在的目录节点。
func (m TreeModel) toggle() TreeModel {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return m.clampScroll()
	}
	node := m.nodes[m.cursor]
	if !node.isDir {
		return m.clampScroll()
	}
	if node.expanded {
		m = m.collapseCurrentDir()
	} else {
		// 展开：在其后插入子节点
		children := readDir(node.path, node.depth+1)
		m.nodes[m.cursor].expanded = true
		tail := append([]treeNode{}, m.nodes[m.cursor+1:]...)
		m.nodes = append(m.nodes[:m.cursor+1], children...)
		m.nodes = append(m.nodes, tail...)
	}
	return m.clampScroll()
}

// collapseCurrentDir 折叠光标所在的已展开目录。
func (m TreeModel) collapseCurrentDir() TreeModel {
	node := m.nodes[m.cursor]
	m.nodes[m.cursor].expanded = false
	i := m.cursor + 1
	j := i
	for j < len(m.nodes) && m.nodes[j].depth > node.depth {
		j++
	}
	m.nodes = append(m.nodes[:i], m.nodes[j:]...)
	return m
}

// collapseOrMoveToParent 实现左键语义：展开目录则折叠，否则回到父目录。
func (m TreeModel) collapseOrMoveToParent() TreeModel {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return m.clampScroll()
	}
	node := m.nodes[m.cursor]
	if node.isDir && node.expanded {
		return m.collapseCurrentDir().clampScroll()
	}
	if node.depth == 0 {
		return m.clampScroll()
	}
	parentDepth := node.depth - 1
	for i := m.cursor - 1; i >= 0; i-- {
		if m.nodes[i].isDir && m.nodes[i].depth == parentDepth {
			m.cursor = i
			break
		}
	}
	return m.clampScroll()
}

// Update 处理按键。
func (m TreeModel) Update(msg tea.Msg) (TreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
			}
		case "enter", "right", "l":
			if m.cursor >= 0 && m.cursor < len(m.nodes) {
				node := m.nodes[m.cursor]
				if node.isDir {
					return m.toggle(), nil
				}
				// 文件 → 通知 Explorer 打开文件
				return m, func() tea.Msg { return treeSelectFileMsg{path: node.path} }
			}
		case "left", "h":
			return m.collapseOrMoveToParent(), nil
		}
		m = m.clampScroll()
	}
	return m, nil
}

// clampScroll 保持光标和滚动偏移在合法可视范围内。
func (m TreeModel) clampScroll() TreeModel {
	if len(m.nodes) == 0 {
		m.cursor = 0
		m.offset = 0
		return m
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.nodes) {
		m.cursor = len(m.nodes) - 1
	}
	if m.height <= 0 {
		m.offset = 0
		return m
	}

	maxOffset := len(m.nodes) - m.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	return m
}

// View 渲染目录树。
func (m TreeModel) View(openedPath string) string {
	if m.root == "" {
		return "  (在左栏按 Enter 选中会话\n   以此目录为根浏览文件)"
	}
	if len(m.nodes) == 0 {
		return "  (空目录)"
	}
	scrollbarWidth := 2
	bodyWidth := max(m.width-scrollbarWidth, 1)
	cleanOpenedPath := ""
	if openedPath != "" {
		cleanOpenedPath = filepath.Clean(openedPath)
	}
	end := min(m.offset+m.height, len(m.nodes))
	lines := make([]string, 0, end-m.offset)
	selectedRows := make([]bool, 0, end-m.offset)
	activeRows := make([]bool, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		n := m.nodes[i]
		isCursor := i == m.cursor
		isOpened := cleanOpenedPath != "" && !n.isDir && filepath.Clean(n.path) == cleanOpenedPath
		lines = append(lines, m.renderLine(n, bodyWidth, isCursor, isOpened))
		selectedRows = append(selectedRows, isCursor)
		activeRows = append(activeRows, isOpened && !isCursor)
	}
	scrollbar := renderTreeScrollbar(m.height, len(m.nodes), m.offset, scrollbarWidth)
	scrollbarTail := renderTreeScrollbar(m.height, len(m.nodes), m.offset, 1)
	for i, line := range lines {
		bar := strings.Repeat(" ", scrollbarWidth)
		if i < len(scrollbar) {
			bar = scrollbar[i]
		}
		if selectedRows[i] {
			tail := " "
			if i < len(scrollbarTail) {
				tail = scrollbarTail[i]
			}
			bar = sessionSelectedTrailingFillStyle.Render(" ") + tail
		} else if activeRows[i] {
			tail := " "
			if i < len(scrollbarTail) {
				tail = scrollbarTail[i]
			}
			bar = sessionActiveTrailingFillStyle.Render(" ") + tail
		}
		lines[i] = padCell(truncateCell(line, bodyWidth), bodyWidth) + bar
	}
	return strings.Join(lines, "\n")
}

func renderTreeScrollbar(viewportHeight, totalHeight, topOffset, width int) []string {
	return renderVerticalScrollbar(viewportHeight, totalHeight, topOffset, verticalScrollbarOptions{
		Width:      width,
		Track:      "│",
		Thumb:      "┃",
		TrackStyle: sessionScrollbarTrackStyle,
		ThumbStyle: sessionScrollbarThumbStyle,
		AlignRight: true,
	})
}

func (m TreeModel) renderLine(n treeNode, bodyWidth int, isCursor, isOpened bool) string {
	status := m.nodeStatus(n)
	mark := status.Mark()

	prefix := treeLinePrefix(n)
	nameWidth := bodyWidth - lipgloss.Width(prefix) - treeGitMarkColumnWidth
	if nameWidth < 1 {
		nameWidth = 1
	}

	name := truncateCell(n.name, nameWidth)
	if style, ok := treeGitStyle(status); ok && !isCursor && !isOpened {
		name = style.Render(name)
	}
	line := prefix + name
	if mark != "" {
		plainWidth := lipgloss.Width(prefix) + lipgloss.Width(truncateCell(n.name, nameWidth))
		padding := bodyWidth - plainWidth - lipgloss.Width(mark)
		if padding < 1 {
			padding = 1
		}
		badge := mark
		if style, ok := treeGitStyle(status); ok && !isCursor && !isOpened {
			badge = style.Render(mark)
		}
		line += strings.Repeat(" ", padding) + badge
	}
	line = padCell(truncateCell(line, bodyWidth), bodyWidth)

	if isOpened {
		body := takeCellSuffix(line, max(bodyWidth-1, 0))
		if isCursor {
			return treeSelectedOpenedFileGuideStyle.Render("┃") + treeCursorStyle.Render(body)
		}
		return treeOpenedFileGuideStyle.Render("┃") + treeOpenedFileStyle.Render(body)
	}
	if isCursor {
		return treeCursorStyle.Render(line)
	}
	return line
}

func (m TreeModel) nodeStatus(n treeNode) gitstatus.Status {
	if len(m.gitStatus) == 0 {
		return gitstatus.StatusNone
	}
	return m.gitStatus[filepath.Clean(n.path)]
}

func treeGitStyle(status gitstatus.Status) (lipgloss.Style, bool) {
	switch status {
	case gitstatus.StatusModified:
		return treeGitModifiedStyle, true
	case gitstatus.StatusAdded:
		return treeGitAddedStyle, true
	case gitstatus.StatusDeleted:
		return treeGitDeletedStyle, true
	case gitstatus.StatusRenamed:
		return treeGitRenamedStyle, true
	case gitstatus.StatusUntracked:
		return treeGitUntrackedStyle, true
	case gitstatus.StatusConflict:
		return treeGitConflictStyle, true
	default:
		return lipgloss.Style{}, false
	}
}
