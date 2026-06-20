package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"cc-sidecar/internal/session"
)

const unknownProjectDir = "(unknown project)"

// sessionChosenMsg is emitted when Enter is pressed on a selected session.
type sessionChosenMsg struct {
	id  string
	cwd string
}

type sessionGroup struct {
	cwd      string
	sessions []session.Session
	expanded bool
}

type sessionRowKind int

const (
	sessionRowGroup sessionRowKind = iota
	sessionRowSession
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
}

func NewSessions(sessions []session.Session) SessionsModel {
	m := SessionsModel{groups: groupSessionGroupsByCWD(sessions)}
	return m.rebuildRows()
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
		case sessionRowGroup:
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

// SetSessions replaces the session data while preserving filter, expanded
// groups, and the current selection when the same session/group still exists.
func (m SessionsModel) SetSessions(sessions []session.Session) SessionsModel {
	oldOffset := m.offset
	oldCursor := m.cursor
	expanded := make(map[string]bool, len(m.groups))
	for _, g := range m.groups {
		expanded[g.cwd] = g.expanded
	}

	selectedSessionKey := ""
	selectedGroupCWD := ""
	if row, ok := m.currentRow(); ok {
		switch row.kind {
		case sessionRowSession:
			if s, ok := m.sessionForRow(row); ok {
				selectedSessionKey = sessionKey(s)
				selectedGroupCWD = sessionProjectDir(s)
			}
		case sessionRowGroup:
			if g, ok := m.groupForRow(row); ok {
				selectedGroupCWD = g.cwd
			}
		}
	}

	m.groups = groupSessionGroupsByCWD(sessions)
	for i := range m.groups {
		if wasExpanded, ok := expanded[m.groups[i].cwd]; ok {
			m.groups[i].expanded = wasExpanded
		}
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
		case sessionRowGroup:
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

	lines := make([]string, 0, m.height)
	available := m.height
	if m.showFilterLine() {
		filter := "/" + m.filter
		if !m.filtering && m.filter != "" {
			filter = "filter: " + m.filter
		}
		lines = append(lines, sessionFilterStyle.Render(truncateCell(filter, m.width)))
		available--
	}
	if available <= 0 {
		return strings.Join(lines, "\n")
	}

	if len(m.rows) == 0 {
		text := "  (no sessions)"
		if m.filter != "" {
			text = "  (no matches)"
		}
		lines = append(lines, truncateCell(text, m.width))
		return strings.Join(lines, "\n")
	}

	used := 0
	for i := m.offset; i < len(m.rows); i++ {
		rowLines := m.renderRow(i)
		if len(rowLines) == 0 {
			continue
		}
		if used+len(rowLines) > available {
			break
		}
		lines = append(lines, rowLines...)
		used += len(rowLines)
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
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
	case sessionRowSession:
		m.cursor = m.groupHeaderIndex(row.groupIndex)
	}
	return m.clampScroll()
}

func (m SessionsModel) moveRight() (SessionsModel, tea.Cmd) {
	row, ok := m.currentRow()
	if !ok {
		return m.clampScroll(), nil
	}
	if row.kind == sessionRowGroup {
		m.groups[row.groupIndex].expanded = true
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
		return m.clampScroll(), nil
	}
	return m.openCurrentSession()
}

func (m SessionsModel) activate() (SessionsModel, tea.Cmd) {
	row, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	if row.kind == sessionRowGroup {
		m.groups[row.groupIndex].expanded = !m.groups[row.groupIndex].expanded
		m = m.rebuildRows()
		m.cursor = m.groupHeaderIndex(row.groupIndex)
		return m.clampScroll(), nil
	}
	return m.openCurrentSession()
}

func (m SessionsModel) openCurrentSession() (SessionsModel, tea.Cmd) {
	if s, ok := m.Selected(); ok {
		return m, func() tea.Msg { return sessionChosenMsg{id: s.ID, cwd: s.CWD} }
	}
	return m, nil
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
		groups = append(groups, sessionGroup{cwd: key, sessions: groupSessions, expanded: true})
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
		if query != "" || group.expanded {
			for _, sessionIndex := range matchedSessions {
				rows = append(rows, sessionRow{kind: sessionRowSession, groupIndex: groupIndex, sessionIndex: sessionIndex})
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

func (m SessionsModel) groupHeaderIndex(groupIndex int) int {
	for i, row := range m.rows {
		if row.kind == sessionRowGroup && row.groupIndex == groupIndex {
			return i
		}
	}
	return m.cursor
}

func (m SessionsModel) rowHeight(index int) int {
	if index < 0 || index >= len(m.rows) {
		return 0
	}
	row := m.rows[index]
	if row.kind == sessionRowGroup {
		return 1
	}
	return 2
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

func (m SessionsModel) renderRow(index int) []string {
	row := m.rows[index]
	selected := index == m.cursor && !m.filtering
	switch row.kind {
	case sessionRowGroup:
		group, ok := m.groupForRow(row)
		if !ok {
			return nil
		}
		return []string{renderSessionGroupHeader(group.cwd, m.width, selected, group.expanded)}
	case sessionRowSession:
		s, ok := m.sessionForRow(row)
		if !ok {
			return nil
		}
		lines := []string{renderSessionTitleLine(s.Title, relativeAge(time.Now(), s.ModTime), m.width, selected)}
		subtitleLine := ""
		if selected {
			if subtitle := sessionSubtitle(s); subtitle != "" {
				subtitleLine = renderSessionSubtitleLine(subtitle, m.width)
			}
		}
		return append(lines, subtitleLine)
	default:
		return nil
	}
}

func renderSessionGroupHeader(dir string, width int, selected, expanded bool) string {
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
		text += truncateMiddleCell(dir, width-prefixWidth)
	}
	style := sessionGroupHeaderStyle
	if selected {
		style = sessionSelectedGroupHeaderStyle
	}
	return style.Render(truncateCell(text, width))
}

func renderSessionTitleLine(title, age string, width int, selected bool) string {
	if width <= 0 {
		return ""
	}

	ageWidth := lipgloss.Width(age)
	if width <= ageWidth {
		age = takeCellSuffix(age, width)
		if selected {
			return sessionSelectedAgeStyle.Render(age)
		}
		return sessionAgeStyle.Render(age)
	}

	prefix := "  "
	style := sessionTitleStyle
	ageStyle := sessionAgeStyle
	fillStyle := lipgloss.NewStyle().Inline(true)
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

func renderSessionSubtitleLine(subtitle string, width int) string {
	if width <= 0 {
		return ""
	}
	line := padCell(truncateCell("  "+subtitle, width), width)
	return sessionSelectedDescStyle.Render(line)
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
