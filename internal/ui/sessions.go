package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"ccdeck/internal/session"
)

const (
	unknownProjectDir           = "(unknown project)"
	defaultSessionGroupPageSize = 10
)

// sessionChosenMsg is emitted when Enter is pressed on a selected session.
type sessionChosenMsg struct {
	id         string
	cwd        string
	projectDir string
}

type sessionGroup struct {
	cwd          string
	sessions     []session.Session
	expanded     bool
	visibleCount int
}

type sessionRowKind int

const (
	sessionRowGroup sessionRowKind = iota
	sessionRowSession
	sessionRowMore
)

type sessionRow struct {
	kind         sessionRowKind
	groupIndex   int
	sessionIndex int
}

// SessionsModel is the left panel: a searchable grouped list of Claude Code sessions.
type SessionsModel struct {
	groups []sessionGroup
	rows   []sessionRow

	cursor int
	offset int
	width  int
	height int

	filtering bool
	filter    string

	activeSessionID string
}

func NewSessions(sessions []session.Session) SessionsModel {
	m := SessionsModel{groups: groupSessionGroupsByCWD(sessions)}
	m = m.applyInitialProjectState()
	m = m.rebuildRows()
	return m.selectFirstSessionRow()
}

func (m SessionsModel) applyInitialProjectState() SessionsModel {
	for i := range m.groups {
		m.groups[i].expanded = i == 0
		m.groups[i].visibleCount = initialSessionVisibleCount(len(m.groups[i].sessions))
	}
	return m
}

func initialSessionVisibleCount(total int) int {
	return min(defaultSessionGroupPageSize, total)
}

func clampSessionVisibleCount(count, total int) int {
	if total <= 0 {
		return 0
	}
	if count <= 0 {
		count = defaultSessionGroupPageSize
	}
	return min(count, total)
}

func (m SessionsModel) selectFirstSessionRow() SessionsModel {
	for i, row := range m.rows {
		if row.kind == sessionRowSession {
			m.cursor = i
			m.offset = 0
			return m
		}
	}
	return m.clampScroll()
}

func (m SessionsModel) SetSize(w, h int) SessionsModel {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	m.width = w
	m.height = h
	return m.clampScroll()
}

func (m SessionsModel) Selected() (session.Session, bool) {
	row, ok := m.currentRow()
	if !ok || row.kind != sessionRowSession {
		return session.Session{}, false
	}
	return m.sessionForRow(row)
}

func (m SessionsModel) CurrentCWD() string {
	if row, ok := m.currentRow(); ok {
		switch row.kind {
		case sessionRowGroup, sessionRowMore:
			if g, ok := m.groupForRow(row); ok {
				return groupCWD(g)
			}
		case sessionRowSession:
			if s, ok := m.sessionForRow(row); ok {
				return s.CWD
			}
		}
	}
	return ""
}

func (m SessionsModel) SetActiveSession(id string) SessionsModel {
	m.activeSessionID = id
	return m
}

// SetSessions replaces the session data while preserving filter, expanded
// groups, and the current selection when the same session/group still exists.
func (m SessionsModel) SetSessions(sessions []session.Session) SessionsModel {
	oldOffset := m.offset
	oldCursor := m.cursor
	expanded := make(map[string]bool, len(m.groups))
	visibleCounts := make(map[string]int, len(m.groups))
	for _, g := range m.groups {
		expanded[g.cwd] = g.expanded
		visibleCounts[g.cwd] = g.visibleCount
	}

	selectedSessionKey := ""
	selectedGroupCWD := ""
	selectedWasMoreRow := false
	if row, ok := m.currentRow(); ok {
		switch row.kind {
		case sessionRowSession:
			if s, ok := m.sessionForRow(row); ok {
				selectedSessionKey = sessionKey(s)
				selectedGroupCWD = sessionProjectDir(s)
			}
		case sessionRowGroup, sessionRowMore:
			selectedWasMoreRow = row.kind == sessionRowMore
			if g, ok := m.groupForRow(row); ok {
				selectedGroupCWD = g.cwd
			}
		}
	}

	m.groups = groupSessionGroupsByCWD(sessions)
	activeExists := m.activeSessionID == ""
	for i := range m.groups {
		if wasExpanded, ok := expanded[m.groups[i].cwd]; ok {
			m.groups[i].expanded = wasExpanded
		}
		m.groups[i].visibleCount = initialSessionVisibleCount(len(m.groups[i].sessions))
		if visibleCount, ok := visibleCounts[m.groups[i].cwd]; ok {
			m.groups[i].visibleCount = clampSessionVisibleCount(visibleCount, len(m.groups[i].sessions))
		}
		for _, s := range m.groups[i].sessions {
			if s.ID == m.activeSessionID {
				activeExists = true
			}
		}
	}
	if !activeExists {
		m.activeSessionID = ""
	}

	m.cursor = oldCursor
	m.offset = oldOffset
	m = m.rebuildRows()

	if selectedSessionKey != "" {
		if idx, ok := m.findSessionRowByKey(selectedSessionKey); ok {
			m.cursor = idx
			m.offset = oldOffset
			return m.clampScroll()
		}
	}
	if selectedGroupCWD != "" {
		if selectedWasMoreRow {
			if groupIdx, ok := m.findGroupIndexByCWD(selectedGroupCWD); ok {
				if idx, ok := m.findMoreRowByGroup(groupIdx); ok {
					m.cursor = idx
					m.offset = oldOffset
					return m.clampScroll()
				}
			}
		}
		if idx, ok := m.findGroupRowByCWD(selectedGroupCWD); ok {
			m.cursor = idx
			m.offset = oldOffset
			return m.clampScroll()
		}
	}
	m.cursor = oldCursor
	m.offset = oldOffset
	return m.clampScroll()
}

// Count returns the total number of sessions, excluding project headers.
func (m SessionsModel) Count() int {
	count := 0
	for _, g := range m.groups {
		count += len(g.sessions)
	}
	return count
}

// SelectedTitle returns the selected session title or project dir for status display.
func (m SessionsModel) SelectedTitle() string {
	if row, ok := m.currentRow(); ok {
		switch row.kind {
		case sessionRowGroup, sessionRowMore:
			if g, ok := m.groupForRow(row); ok {
				return g.cwd
			}
		case sessionRowSession:
			if s, ok := m.sessionForRow(row); ok {
				return s.Title
			}
		}
	}
	return ""
}

func (m SessionsModel) Update(msg tea.Msg) (SessionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.filtering {
			return m.updateFilter(msg), nil
		}
		switch msg.String() {
		case "/":
			m.filtering = true
			return m, nil
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m = m.rebuildRows()
			}
			return m.clampScroll(), nil
		case "up", "k":
			m.cursor--
			return m.clampScroll(), nil
		case "down", "j":
			m.cursor++
			return m.clampScroll(), nil
		case "left", "h":
			return m.moveLeft(), nil
		case "right", "l":
			return m.moveRight()
		case "enter":
			return m.activate()
		}
	}
	return m, nil
}

func (m SessionsModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	scrollbarWidth := 2
	bodyWidth := max(m.width-scrollbarWidth, 1)
	lines := make([]string, 0, m.height)
	available := m.height
	if m.showFilterLine() {
		filter := "/" + m.filter
		if !m.filtering && m.filter != "" {
			filter = "filter: " + m.filter
		}
		lines = append(lines, padCell(sessionFilterStyle.Render(truncateCell(filter, bodyWidth)), bodyWidth)+strings.Repeat(" ", scrollbarWidth))
		available--
	}
	if available <= 0 {
		return strings.Join(lines, "\n")
	}

	bodyLines := make([]string, 0, available)
	selectedLineFlags := make([]bool, 0, available)
	activeLineFlags := make([]bool, 0, available)
	if len(m.rows) == 0 {
		text := "  (no sessions)"
		if m.filter != "" {
			text = "  (no matches)"
		}
		bodyLines = append(bodyLines, truncateCell(text, bodyWidth))
		selectedLineFlags = append(selectedLineFlags, false)
		activeLineFlags = append(activeLineFlags, false)
	} else {
		used := 0
		for i := m.offset; i < len(m.rows); i++ {
			rowLines := m.renderRow(i, bodyWidth)
			if len(rowLines) == 0 {
				continue
			}
			if used+len(rowLines) > available {
				break
			}
			bodyLines = append(bodyLines, rowLines...)
			selected := i == m.cursor && !m.filtering
			active := !selected && m.rows[i].kind == sessionRowSession
			if active {
				if s, ok := m.sessionForRow(m.rows[i]); !ok || s.ID != m.activeSessionID {
					active = false
				}
			}
			extendSelectedGutter := selected && m.rows[i].kind != sessionRowGroup
			extendActiveGutter := active
			for range rowLines {
				selectedLineFlags = append(selectedLineFlags, extendSelectedGutter)
				activeLineFlags = append(activeLineFlags, extendActiveGutter)
			}
			used += len(rowLines)
		}
	}

	scrollbar := m.renderScrollbar(available, 1)
	for i, line := range bodyLines {
		scrollGlyph := " "
		if i < len(scrollbar) {
			scrollGlyph = scrollbar[i]
		}
		bar := strings.Repeat(" ", max(scrollbarWidth-1, 0)) + scrollGlyph
		if i < len(selectedLineFlags) && selectedLineFlags[i] && scrollbarWidth > 0 {
			bar = sessionSelectedTrailingFillStyle.Render(" ") + scrollGlyph
		}
		if i < len(activeLineFlags) && activeLineFlags[i] && scrollbarWidth > 0 {
			bar = sessionActiveTrailingFillStyle.Render(" ") + scrollGlyph
		}
		lines = append(lines, padCell(truncateCell(line, bodyWidth), bodyWidth)+bar)
	}
	return strings.Join(lines, "\n")
}

func (m SessionsModel) updateFilter(msg tea.KeyPressMsg) SessionsModel {
	switch msg.String() {
	case "enter":
		m.filtering = false
		return m.clampScroll()
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filtering = false
			m = m.rebuildRows()
			return m.clampScroll()
		}
		m.filtering = false
		return m
	case "backspace":
		if m.filter != "" {
			r := []rune(m.filter)
			m.filter = string(r[:len(r)-1])
			m = m.rebuildRows()
		}
		return m.clampScroll()
	}

	if text := msg.Key().Text; text != "" {
		m.filter += text
		m = m.rebuildRows()
		return m.clampScroll()
	}
	return m
}

func (m SessionsModel) moveLeft() SessionsModel {
	row, ok := m.currentRow()
	if !ok {
		return m.clampScroll()
	}
	switch row.kind {
	case sessionRowGroup:
		m.groups[row.groupIndex].expanded = false
		m = m.resetGroupVisibleCount(row.groupIndex)
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
	case sessionRowSession, sessionRowMore:
		m.cursor = m.groupHeaderIndex(row.groupIndex)
	}
	return m.clampScroll()
}

func (m SessionsModel) moveRight() (SessionsModel, tea.Cmd) {
	row, ok := m.currentRow()
	if !ok {
		return m.clampScroll(), nil
	}
	switch row.kind {
	case sessionRowGroup:
		m.groups[row.groupIndex].expanded = true
		m = m.resetGroupVisibleCount(row.groupIndex)
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
		return m.clampScroll(), nil
	case sessionRowMore:
		return m.loadMore(row), nil
	}
	return m.openCurrentSession()
}

func (m SessionsModel) activate() (SessionsModel, tea.Cmd) {
	row, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	switch row.kind {
	case sessionRowGroup:
		m.groups[row.groupIndex].expanded = !m.groups[row.groupIndex].expanded
		m = m.resetGroupVisibleCount(row.groupIndex)
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
		return m.clampScroll(), nil
	case sessionRowMore:
		return m.loadMore(row), nil
	}
	return m.openCurrentSession()
}

func (m SessionsModel) resetGroupVisibleCount(groupIndex int) SessionsModel {
	if groupIndex < 0 || groupIndex >= len(m.groups) {
		return m
	}
	m.groups[groupIndex].visibleCount = initialSessionVisibleCount(len(m.groups[groupIndex].sessions))
	return m
}

func (m SessionsModel) loadMore(row sessionRow) SessionsModel {
	if row.groupIndex < 0 || row.groupIndex >= len(m.groups) {
		return m.clampScroll()
	}
	group := &m.groups[row.groupIndex]
	oldVisibleCount := group.visibleCount
	group.visibleCount = clampSessionVisibleCount(group.visibleCount+defaultSessionGroupPageSize, len(group.sessions))
	m = m.rebuildRows()
	if moreIndex, ok := m.findMoreRowByGroup(row.groupIndex); ok {
		m.cursor = moreIndex
		return m.clampScroll()
	}
	m.cursor = min(m.groupHeaderIndex(row.groupIndex)+group.visibleCount, len(m.rows)-1)
	if group.visibleCount == oldVisibleCount {
		m.cursor = min(m.cursor, len(m.rows)-1)
	}
	return m.clampScroll()
}

func (m SessionsModel) openCurrentSession() (SessionsModel, tea.Cmd) {
	if s, ok := m.Selected(); ok {
		return m, func() tea.Msg { return sessionChosenMsg{id: s.ID, cwd: s.CWD, projectDir: sessionExplorerRoot(s)} }
	}
	return m, nil
}

func sessionExplorerRoot(s session.Session) string {
	if s.ProjectDir != "" {
		return s.ProjectDir
	}
	return s.CWD
}

func groupSessionGroupsByCWD(sessions []session.Session) []sessionGroup {
	groupsByCWD := make(map[string][]session.Session)
	keys := make([]string, 0)
	seen := make(map[string]bool)
	for _, s := range sessions {
		key := sessionProjectDir(s)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
		groupsByCWD[key] = append(groupsByCWD[key], s)
	}

	groups := make([]sessionGroup, 0, len(keys))
	for _, key := range keys {
		groupSessions := groupsByCWD[key]
		sort.SliceStable(groupSessions, func(i, j int) bool {
			return groupSessions[i].ModTime.After(groupSessions[j].ModTime)
		})
		groups = append(groups, sessionGroup{cwd: key, sessions: groupSessions, expanded: true, visibleCount: initialSessionVisibleCount(len(groupSessions))})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return latestSessionTime(groups[i]).After(latestSessionTime(groups[j]))
	})
	return groups
}

func latestSessionTime(group sessionGroup) time.Time {
	if len(group.sessions) == 0 {
		return time.Time{}
	}
	return group.sessions[0].ModTime
}

func (m SessionsModel) rebuildRows() SessionsModel {
	rows := make([]sessionRow, 0)
	query := strings.ToLower(strings.TrimSpace(m.filter))
	for groupIndex, group := range m.groups {
		matchedSessions := make([]int, 0, len(group.sessions))
		if query != "" {
			for sessionIndex, s := range group.sessions {
				if sessionMatchesFilter(s, query) {
					matchedSessions = append(matchedSessions, sessionIndex)
				}
			}
			if len(matchedSessions) == 0 && !strings.Contains(strings.ToLower(group.cwd), query) {
				continue
			}
		} else {
			for sessionIndex := range group.sessions {
				matchedSessions = append(matchedSessions, sessionIndex)
			}
		}

		rows = append(rows, sessionRow{kind: sessionRowGroup, groupIndex: groupIndex})
		if query != "" {
			for _, sessionIndex := range matchedSessions {
				rows = append(rows, sessionRow{kind: sessionRowSession, groupIndex: groupIndex, sessionIndex: sessionIndex})
			}
			continue
		}
		if group.expanded {
			visibleCount := clampSessionVisibleCount(group.visibleCount, len(matchedSessions))
			for _, sessionIndex := range matchedSessions[:visibleCount] {
				rows = append(rows, sessionRow{kind: sessionRowSession, groupIndex: groupIndex, sessionIndex: sessionIndex})
			}
			if visibleCount < len(matchedSessions) {
				rows = append(rows, sessionRow{kind: sessionRowMore, groupIndex: groupIndex})
			}
		}
	}
	m.rows = rows
	return m.clampScroll()
}

func sessionProjectDir(s session.Session) string {
	if s.CWD == "" {
		return unknownProjectDir
	}
	return s.CWD
}

func groupCWD(g sessionGroup) string {
	if g.cwd == unknownProjectDir {
		return ""
	}
	return g.cwd
}

func sessionMatchesFilter(s session.Session, query string) bool {
	value := strings.ToLower(s.Title + " " + s.LastPrompt + " " + s.CWD + " " + s.GitBranch)
	return strings.Contains(value, query)
}

func (m SessionsModel) currentRow() (sessionRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return sessionRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m SessionsModel) groupForRow(row sessionRow) (sessionGroup, bool) {
	if row.groupIndex < 0 || row.groupIndex >= len(m.groups) {
		return sessionGroup{}, false
	}
	return m.groups[row.groupIndex], true
}

func (m SessionsModel) sessionForRow(row sessionRow) (session.Session, bool) {
	group, ok := m.groupForRow(row)
	if !ok || row.sessionIndex < 0 || row.sessionIndex >= len(group.sessions) {
		return session.Session{}, false
	}
	return group.sessions[row.sessionIndex], true
}

func (m SessionsModel) groupHasActiveSession(group sessionGroup) bool {
	if m.activeSessionID == "" {
		return false
	}
	for _, s := range group.sessions {
		if s.ID == m.activeSessionID {
			return true
		}
	}
	return false
}

func sessionKey(s session.Session) string {
	if s.Path != "" {
		return s.Path
	}
	return s.ID
}

func (m SessionsModel) findSessionRowByKey(key string) (int, bool) {
	for i, row := range m.rows {
		if row.kind != sessionRowSession {
			continue
		}
		if s, ok := m.sessionForRow(row); ok && sessionKey(s) == key {
			return i, true
		}
	}
	return 0, false
}

func (m SessionsModel) findGroupRowByCWD(cwd string) (int, bool) {
	for i, row := range m.rows {
		if row.kind != sessionRowGroup {
			continue
		}
		if g, ok := m.groupForRow(row); ok && g.cwd == cwd {
			return i, true
		}
	}
	return 0, false
}

func (m SessionsModel) findGroupIndexByCWD(cwd string) (int, bool) {
	for i, group := range m.groups {
		if group.cwd == cwd {
			return i, true
		}
	}
	return 0, false
}

func (m SessionsModel) groupHeaderIndex(groupIndex int) int {
	for i, row := range m.rows {
		if row.kind == sessionRowGroup && row.groupIndex == groupIndex {
			return i
		}
	}
	return m.cursor
}

func (m SessionsModel) findMoreRowByGroup(groupIndex int) (int, bool) {
	for i, row := range m.rows {
		if row.kind == sessionRowMore && row.groupIndex == groupIndex {
			return i, true
		}
	}
	return 0, false
}

func (m SessionsModel) rowHeight(index int) int {
	if index < 0 || index >= len(m.rows) {
		return 0
	}
	row := m.rows[index]
	switch row.kind {
	case sessionRowGroup:
		return 1
	case sessionRowMore:
		return 2
	case sessionRowSession:
		return 2
	default:
		return 0
	}
}

func (m SessionsModel) clampScroll() SessionsModel {
	if len(m.rows) == 0 {
		m.cursor = 0
		m.offset = 0
		return m
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset >= len(m.rows) {
		m.offset = len(m.rows) - 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	available := m.rowViewportHeight()
	for m.offset < m.cursor && m.rowsHeight(m.offset, m.cursor) > available {
		m.offset++
	}
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	return m
}

func (m SessionsModel) rowsHeight(start, end int) int {
	height := 0
	for i := start; i <= end && i < len(m.rows); i++ {
		height += m.rowHeight(i)
	}
	return height
}

func (m SessionsModel) rowViewportHeight() int {
	height := m.height
	if m.showFilterLine() {
		height--
	}
	return max(height, 1)
}

func (m SessionsModel) showFilterLine() bool {
	return m.filtering || m.filter != ""
}

func (m SessionsModel) totalContentHeight() int {
	if len(m.rows) == 0 {
		return 1
	}
	return m.rowsHeight(0, len(m.rows)-1)
}

func (m SessionsModel) contentOffsetBeforeRow(index int) int {
	if index <= 0 || len(m.rows) == 0 {
		return 0
	}
	return m.rowsHeight(0, min(index, len(m.rows))-1)
}

func (m SessionsModel) renderScrollbar(viewportHeight, width int) []string {
	return renderVerticalScrollbar(viewportHeight, m.totalContentHeight(), m.contentOffsetBeforeRow(m.offset), verticalScrollbarOptions{
		Width:      width,
		Track:      "│",
		Thumb:      "┃",
		TrackStyle: sessionScrollbarTrackStyle,
		ThumbStyle: sessionScrollbarThumbStyle,
		AlignRight: true,
	})
}

func (m SessionsModel) renderRow(index, width int) []string {
	row := m.rows[index]
	selected := index == m.cursor && !m.filtering
	switch row.kind {
	case sessionRowGroup:
		group, ok := m.groupForRow(row)
		if !ok {
			return nil
		}
		active := !selected && m.groupHasActiveSession(group)
		return []string{renderSessionGroupHeader(group.cwd, len(group.sessions), width, selected, active, group.expanded)}
	case sessionRowSession:
		s, ok := m.sessionForRow(row)
		if !ok {
			return nil
		}
		active := s.ID == m.activeSessionID
		age := relativeAge(time.Now(), s.ModTime)
		lines := []string{renderSessionTitleLine(s.Title, age, width, selected, active)}
		subtitleLine := renderSessionReservedSubtitleLine(width, selected, active)
		if selected || active {
			if subtitle := sessionSubtitle(s); subtitle != "" {
				subtitleLine = renderSessionSubtitleLine(subtitle, age, width, selected, active)
			}
		}
		return append(lines, subtitleLine)
	case sessionRowMore:
		group, ok := m.groupForRow(row)
		if !ok {
			return nil
		}
		return []string{
			renderSessionMoreRow(len(group.sessions)-group.visibleCount, width, selected),
			renderSessionReservedSubtitleLine(width, selected, false),
		}
	default:
		return nil
	}
}

// projectDirLabel 返回 session group 头部展示的标签：只取路径最末的目录名。
// 空值与占位符原样返回。
func projectDirLabel(dir string) string {
	if dir == "" || dir == unknownProjectDir {
		return dir
	}
	return filepath.Base(filepath.Clean(dir))
}

func projectDirCountLabel(dir string, count int) string {
	label := projectDirLabel(dir)
	return fmt.Sprintf("%s (%d)", label, count)
}

func renderSessionMoreRow(remaining, width int, selected bool) string {
	if width <= 0 {
		return ""
	}
	text := fmt.Sprintf("  Load more (%d remaining)", max(remaining, 0))
	if selected {
		return sessionSelectedDescStyle.Render(padCell(truncateCell(text, width), width))
	}
	return sessionDescStyle.Render(truncateCell(text, width))
}

func renderSessionGroupHeader(dir string, count, width int, selected, active, expanded bool) string {
	if width <= 0 {
		return ""
	}
	twisty := "› "
	if expanded {
		twisty = "⌄ "
	}
	prefixWidth := lipgloss.Width(twisty)
	text := twisty
	if width > prefixWidth {
		text += truncatePrefixCell(projectDirCountLabel(dir, count), width-prefixWidth)
	}
	style := sessionGroupHeaderStyle
	if active {
		style = sessionActiveGroupHeaderStyle
	}
	if selected {
		style = sessionSelectedGroupHeaderStyle
	}
	return style.Render(truncateCell(text, width))
}

func renderSessionTitleLine(title, age string, width int, selected, active bool) string {
	if width <= 0 {
		return ""
	}

	ageWidth := lipgloss.Width(age)
	if width <= ageWidth {
		age = takeCellSuffix(age, width)
		if selected {
			return sessionSelectedAgeStyle.Render(age)
		}
		if active {
			return sessionActiveAgeStyle.Render(age)
		}
		return sessionAgeStyle.Render(age)
	}

	prefix := sessionRowPrefix(active)
	style := sessionTitleStyle
	ageStyle := sessionAgeStyle
	fillStyle := sessionNormalFillStyle
	if active {
		style = sessionActiveTitleStyle
		ageStyle = sessionActiveAgeStyle
		fillStyle = sessionActiveFillStyle
	}
	if selected {
		style = sessionSelectedTitleStyle
		ageStyle = sessionSelectedAgeStyle
		fillStyle = sessionSelectedFillStyle
	}

	availableLeft := width - ageWidth
	left := truncateCell(prefix, availableLeft)
	if titleWidth := availableLeft - lipgloss.Width(left) - 1; titleWidth > 0 {
		left += truncateCell(title, titleWidth)
	}
	padding := max(width-lipgloss.Width(left)-ageWidth, 0)
	return style.Render(left) + fillStyle.Render(strings.Repeat(" ", padding)) + ageStyle.Render(age)
}

func renderSessionReservedSubtitleLine(width int, selected, active bool) string {
	if width <= 0 {
		return ""
	}
	if selected {
		return sessionSelectedFillStyle.Render(strings.Repeat(" ", width))
	}
	if active {
		return sessionActiveFillStyle.Render(strings.Repeat(" ", width))
	}
	return strings.Repeat(" ", width)
}

func renderSessionSubtitleLine(subtitle, age string, width int, selected, active bool) string {
	if width <= 0 {
		return ""
	}
	ageWidth := lipgloss.Width(age)
	fillStyle := sessionActiveFillStyle
	descStyle := sessionActiveDescStyle
	if selected {
		fillStyle = sessionSelectedFillStyle
		descStyle = sessionSelectedDescStyle
	}
	if width <= ageWidth {
		return fillStyle.Render(strings.Repeat(" ", width))
	}
	leftWidth := width - ageWidth
	left := padCell(truncateCell(sessionRowPrefix(false)+subtitle, leftWidth), leftWidth)
	return descStyle.Render(left) + fillStyle.Render(strings.Repeat(" ", ageWidth))
}

func sessionRowPrefix(_ bool) string {
	return "  "
}

func sessionSubtitle(s session.Session) string {
	if s.LastPrompt != "" && s.LastPrompt != s.Title {
		return s.LastPrompt
	}
	return ""
}

func relativeAge(now, t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := now.Sub(t)
	if d < time.Minute {
		return "NOW"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dM", int(d/time.Minute))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dH", int(d/time.Hour))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%dD", int(d/(24*time.Hour)))
	}
	if d < 52*7*24*time.Hour {
		return fmt.Sprintf("%dW", int(d/(7*24*time.Hour)))
	}
	return fmt.Sprintf("%dY", int(d/(365*24*time.Hour)))
}
