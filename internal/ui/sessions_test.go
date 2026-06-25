package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"ccdeck/internal/session"
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
}

func makeSessionsWithPrompts(count int, cwd string) []session.Session {
	sessions := makeSessions(count, cwd)
	for i := range sessions {
		sessions[i].LastPrompt = "Prompt " + sessions[i].ID
	}
	return sessions
}

func makeSessions(count int, cwd string) []session.Session {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	sessions := make([]session.Session, 0, count)
	for i := range count {
		sessions = append(sessions, session.Session{
			ID:      fmt.Sprintf("s%02d", i),
			CWD:     cwd,
			Title:   fmt.Sprintf("Session %02d", i),
			ModTime: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	return sessions
}

func sessionsKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	code := rune(0)
	if len(runes) > 0 {
		code = runes[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func TestGroupSessionGroupsByCWD(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	sessions := []session.Session{
		{ID: "a-new", CWD: "/repo/a", ModTime: now},
		{ID: "b", CWD: "/repo/b", ModTime: now.Add(-time.Hour)},
		{ID: "a-old", CWD: "/repo/a", ModTime: now.Add(-2 * time.Hour)},
		{ID: "c", CWD: "/repo/c", ModTime: now.Add(-3 * time.Hour)},
	}

	groups := groupSessionGroupsByCWD(sessions)
	got := make([]string, 0)
	for _, g := range groups {
		for _, s := range g.sessions {
			got = append(got, s.ID)
		}
	}

	want := []string{"a-new", "a-old", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("grouped session order = %v, want %v", got, want)
	}
	if !groups[0].expanded {
		t.Fatal("groups should default to expanded")
	}
}

func TestRelativeAge(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{"now", now.Add(-30 * time.Second), "NOW"},
		{"minutes", now.Add(-10 * time.Minute), "10M"},
		{"hours", now.Add(-2 * time.Hour), "2H"},
		{"days", now.Add(-24 * time.Hour), "1D"},
		{"weeks", now.Add(-8 * 24 * time.Hour), "1W"},
		{"future", now.Add(time.Hour), "NOW"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relativeAge(now, tt.time); got != tt.want {
				t.Fatalf("relativeAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderSessionTitleLineKeepsAgeWhenNarrow(t *testing.T) {
	line := renderSessionTitleLine("a very long session title", "15D", 8, false, false)
	plain := stripANSI(line)

	if width := lipgloss.Width(plain); width > 8 {
		t.Fatalf("title line width = %d, want <= 8: %q", width, plain)
	}
	if !strings.HasSuffix(plain, "15D") {
		t.Fatalf("title line should keep full age at right edge, got %q", plain)
	}
	if strings.Contains(plain, "15…") || strings.Contains(plain, "15...") {
		t.Fatalf("age should not be truncated, got %q", plain)
	}
}

func TestRenderSessionSubtitleLineReservesAgeColumn(t *testing.T) {
	line := renderSessionSubtitleLine("a very long selected session subtitle", "15D", 14)
	plain := stripANSI(line)

	if width := lipgloss.Width(plain); width != 14 {
		t.Fatalf("subtitle line width = %d, want 14: %q", width, plain)
	}
	if !strings.HasSuffix(plain, "   ") {
		t.Fatalf("subtitle line should reserve age width at right edge, got %q", plain)
	}
	if strings.Contains(plain, "15D") {
		t.Fatalf("subtitle line should not render age text, got %q", plain)
	}
}

func TestRenderActiveSessionTitleLineKeepsAgeWhenNarrow(t *testing.T) {
	line := renderSessionTitleLine("a very long session title", "15D", 8, false, true)
	plain := stripANSI(line)

	if width := lipgloss.Width(plain); width > 8 {
		t.Fatalf("active title line width = %d, want <= 8: %q", width, plain)
	}
	if !strings.HasPrefix(plain, "│") {
		t.Fatalf("active title line should render left guide, got %q", plain)
	}
	if !strings.HasSuffix(plain, "15D") {
		t.Fatalf("active title line should keep full age at right edge, got %q", plain)
	}
}

func TestRenderReservedSubtitleLineUsesPlainSpaces(t *testing.T) {
	got := renderSessionReservedSubtitleLine(14, false)
	want := strings.Repeat(" ", 14)
	if got != want {
		t.Fatalf("reserved subtitle line = %q, want %q", got, want)
	}
}

func TestRenderReservedSubtitleLineHighlightsSelectedRow(t *testing.T) {
	line := renderSessionReservedSubtitleLine(14, true)
	if lipgloss.Width(stripANSI(line)) != 14 {
		t.Fatalf("reserved subtitle line width = %d, want 14", lipgloss.Width(stripANSI(line)))
	}
	if line == stripANSI(line) {
		t.Fatalf("selected reserved subtitle line should be styled, got %q", line)
	}
}

func TestLoadMoreReservedLineHighlightsSelectedRow(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a"))
	m.cursor = len(m.rows) - 1
	lines := m.renderRow(m.cursor, 40)
	if len(lines) != 2 {
		t.Fatalf("load-more rendered lines = %d, want 2", len(lines))
	}
	if lines[0] == stripANSI(lines[0]) {
		t.Fatalf("selected load-more title line should be styled across the row, got %q", lines[0])
	}
	if lines[1] == stripANSI(lines[1]) {
		t.Fatalf("selected load-more reserved line should be styled, got %q", lines[1])
	}
}

func TestRenderSessionSubtitleLineOmitsActiveGuide(t *testing.T) {
	line := renderSessionSubtitleLine("a very long selected session subtitle", "15D", 14)
	plain := stripANSI(line)

	if width := lipgloss.Width(plain); width != 14 {
		t.Fatalf("subtitle line width = %d, want 14: %q", width, plain)
	}
	if strings.HasPrefix(plain, "│") {
		t.Fatalf("subtitle line should not render active left guide, got %q", plain)
	}
	if strings.Contains(plain, "15D") {
		t.Fatalf("subtitle line should not render age text, got %q", plain)
	}
}

func TestRenderSessionGroupHeaderShowsCount(t *testing.T) {
	line := renderSessionGroupHeader("/repo/project", 3, 40, false, false, true)
	plain := stripANSI(line)

	if !strings.Contains(plain, "project (3)") {
		t.Fatalf("group header should show basename and count, got %q", plain)
	}
}

func TestNewSessionsExpandsOnlyFirstProjectAndSelectsFirstSession(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	m := NewSessions([]session.Session{
		{ID: "old-a", CWD: "/repo/a", Title: "Old A", ModTime: now.Add(-2 * time.Hour)},
		{ID: "new-b", CWD: "/repo/b", Title: "New B", ModTime: now},
		{ID: "old-b", CWD: "/repo/b", Title: "Old B", ModTime: now.Add(-time.Hour)},
	})

	if len(m.groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(m.groups))
	}
	if !m.groups[0].expanded || m.groups[1].expanded {
		t.Fatalf("initial expansion = [%v %v], want first expanded only", m.groups[0].expanded, m.groups[1].expanded)
	}
	row, ok := m.currentRow()
	if !ok || row.kind != sessionRowSession {
		t.Fatalf("initial cursor row = %+v, want first session row", row)
	}
	selected, ok := m.Selected()
	if !ok || selected.ID != "new-b" {
		t.Fatalf("selected = %+v, %v; want new-b", selected, ok)
	}
	if m.offset != 0 {
		t.Fatalf("initial offset = %d, want project header visible at top", m.offset)
	}
}

func TestSessionActiveAndSelectedStylePrecedence(t *testing.T) {
	selected := renderSessionTitleLine("Title", "1H", 24, true, false)
	active := renderSessionTitleLine("Title", "1H", 24, false, true)
	both := renderSessionTitleLine("Title", "1H", 24, true, true)

	if selected == active {
		t.Fatal("selected and active title styles should differ")
	}
	if stripANSI(both) == stripANSI(selected) || !strings.HasPrefix(stripANSI(both), "│") {
		t.Fatalf("selected active row should keep selected styling with active guide, got %q", stripANSI(both))
	}

	selectedGroup := renderSessionGroupHeader("/repo/project", 1, 24, true, false, true)
	activeGroup := renderSessionGroupHeader("/repo/project", 1, 24, false, true, true)
	bothGroup := renderSessionGroupHeader("/repo/project", 1, 24, true, true, true)
	if selectedGroup == activeGroup {
		t.Fatal("selected and active group styles should differ")
	}
	if strings.Contains(activeGroup, "\x1b[4") || strings.Contains(activeGroup, ";4m") {
		t.Fatalf("active group style should not underline, got %q", activeGroup)
	}
	if bothGroup != selectedGroup {
		t.Fatal("selected group style should win when a group is both selected and active")
	}
}

func TestSessionsLoadMoreRowForLargeGroup(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a"))

	if len(m.rows) != defaultSessionGroupPageSize+2 {
		t.Fatalf("rows = %d, want header + page + more", len(m.rows))
	}
	last := m.rows[len(m.rows)-1]
	if last.kind != sessionRowMore {
		t.Fatalf("last row = %+v, want load-more row", last)
	}
	if m.rowHeight(len(m.rows)-1) != 2 {
		t.Fatalf("load-more row height = %d, want 2", m.rowHeight(len(m.rows)-1))
	}
	if lines := m.renderRow(len(m.rows)-1, 40); len(lines) != 2 {
		t.Fatalf("load-more rendered lines = %d, want 2", len(lines))
	}
}

func TestSessionsActivateLoadMoreExpandsNextPageAndKeepsMoreSelected(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize*2+5, "/repo/a"))
	m.cursor = len(m.rows) - 1

	m, cmd := m.Update(sessionsKey("enter"))
	if cmd != nil {
		t.Fatal("load-more should not emit session activation command")
	}
	if m.groups[0].visibleCount != defaultSessionGroupPageSize*2 {
		t.Fatalf("visibleCount = %d, want two pages visible", m.groups[0].visibleCount)
	}
	if row, _ := m.currentRow(); row.kind != sessionRowMore {
		t.Fatalf("load-more should remain selected when more sessions remain, got %+v", row)
	}

	m, cmd = m.Update(sessionsKey("right"))
	if cmd != nil {
		t.Fatal("load-more right key should not emit session activation command")
	}
	if m.groups[0].visibleCount != defaultSessionGroupPageSize*2+5 {
		t.Fatalf("visibleCount = %d, want all sessions visible", m.groups[0].visibleCount)
	}
	if got := m.rows[len(m.rows)-1].kind; got == sessionRowMore {
		t.Fatal("load-more row should disappear after all sessions are visible")
	}
}

func TestSessionsFilterBypassesLoadMore(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a")).SetSize(80, 8)
	m.filter = "Session"
	m = m.rebuildRows()

	if len(m.rows) != defaultSessionGroupPageSize+6 {
		t.Fatalf("filtered rows = %d, want header + all sessions", len(m.rows))
	}
	for _, row := range m.rows {
		if row.kind == sessionRowMore {
			t.Fatal("filter should not render load-more row")
		}
	}
}

func TestSessionsLoadMoreSelectionSurvivesRefresh(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a"))
	m.cursor = len(m.rows) - 1

	m = m.SetSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a"))
	row, ok := m.currentRow()
	if !ok || row.kind != sessionRowMore {
		t.Fatalf("selected row after refresh = %+v, %v; want load-more row", row, ok)
	}
}

func TestSessionsVisibleCountSurvivesRefresh(t *testing.T) {
	m := NewSessions(makeSessions(defaultSessionGroupPageSize+5, "/repo/a"))
	m.cursor = len(m.rows) - 1
	m, _ = m.Update(sessionsKey("enter"))

	m = m.SetSessions(makeSessions(defaultSessionGroupPageSize+3, "/repo/a"))
	if m.groups[0].visibleCount != defaultSessionGroupPageSize+3 {
		t.Fatalf("visibleCount = %d, want clamped refreshed count", m.groups[0].visibleCount)
	}
}

func TestSessionsViewRendersScrollbarWhenContentOverflows(t *testing.T) {
	m := NewSessions(makeSessions(8, "/repo/a")).SetSize(24, 4)
	view := m.View()
	plain := stripANSI(view)
	lines := strings.Split(plain, "\n")

	if len(lines) == 0 {
		t.Fatal("view should render lines")
	}
	for _, line := range lines {
		if width := lipgloss.Width(line); width > 24 {
			t.Fatalf("line width = %d, want <= 24: %q", width, line)
		}
	}
	if !strings.Contains(plain, "┃") {
		t.Fatalf("overflowing view should render scrollbar thumb, got %q", plain)
	}
}

func TestSessionsViewKeepsScrollbarGutterWhenContentFits(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetSize(24, 4)
	plain := stripANSI(m.View())
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 {
		t.Fatal("view should render lines")
	}
	for _, line := range lines {
		if width := lipgloss.Width(line); width != 24 {
			t.Fatalf("line width = %d, want stable gutter width 24: %q", width, line)
		}
	}
}

func TestSessionsRowsReserveSubtitleLine(t *testing.T) {
	m := NewSessions(makeSessionsWithPrompts(2, "/repo/a")).SetSize(40, 6)
	for i := 1; i <= 2; i++ {
		if m.rowHeight(i) != 2 || len(m.renderRow(i, 40)) != 2 {
			t.Fatalf("session row %d should reserve subtitle line, height=%d lines=%d", i, m.rowHeight(i), len(m.renderRow(i, 40)))
		}
	}

	m = NewSessions([]session.Session{{ID: "same", CWD: "/repo/a", Title: "Same", LastPrompt: "Same"}}).SetSize(40, 4)
	if m.rowHeight(1) != 2 || len(m.renderRow(1, 40)) != 2 {
		t.Fatalf("selected row with duplicate subtitle should still reserve subtitle line, height=%d lines=%d", m.rowHeight(1), len(m.renderRow(1, 40)))
	}
}

func TestSessionsResizeKeepsProjectHeaderAboveSelectedSession(t *testing.T) {
	m := NewSessions(makeSessionsWithPrompts(2, "/repo/a")).SetSize(40, 8)
	m, _ = m.Update(sessionsKey("down"))
	m = m.SetSize(40, 5)

	if m.offset != 0 {
		t.Fatalf("offset after resize = %d, want project header visible", m.offset)
	}
	plain := stripANSI(m.View())
	lines := strings.Split(plain, "\n")
	if len(lines) < 4 {
		t.Fatalf("rendered lines = %d, want at least 4: %q", len(lines), plain)
	}
	wantOrder := []string{"a (2)", "Session 00", "", "Session 01", "Prompt s01"}
	for i, want := range wantOrder {
		if want == "" {
			if strings.TrimSpace(lines[i]) != "" {
				t.Fatalf("line %d = %q, want reserved blank subtitle line; full view: %q", i, lines[i], plain)
			}
			continue
		}
		if !strings.Contains(lines[i], want) {
			t.Fatalf("line %d = %q, want to contain %q; full view: %q", i, lines[i], want, plain)
		}
	}
}

func TestSessionsActiveSessionSurvivesRefreshOnlyWhenPresent(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetActiveSession("one")

	m = m.SetSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One refreshed"}})
	if m.activeSessionID != "one" {
		t.Fatalf("active session should survive refresh when present, got %q", m.activeSessionID)
	}

	m = m.SetSessions([]session.Session{{ID: "two", CWD: "/repo/a", Title: "Two"}})
	if m.activeSessionID != "" {
		t.Fatalf("active session should clear when missing after refresh, got %q", m.activeSessionID)
	}
}

func TestSessionSubtitleOmitsBranchAndDuplicateTitle(t *testing.T) {
	s := session.Session{
		Title:      "Fix footer",
		LastPrompt: "Investigate truncation",
		CWD:        "/data00/home/gaolei.veew/sourcecode/ccdeck",
		GitBranch:  "HEAD",
		ModTime:    time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
	}

	subtitle := sessionSubtitle(s)
	if subtitle != "Investigate truncation" {
		t.Fatalf("subtitle = %q, want last prompt only", subtitle)
	}
	if strings.Contains(subtitle, "HEAD") || strings.Contains(subtitle, s.CWD) || strings.Contains(subtitle, "06-20") {
		t.Fatalf("subtitle should not include branch/cwd/date, got %q", subtitle)
	}

	s.LastPrompt = s.Title
	if got := sessionSubtitle(s); got != "" {
		t.Fatalf("duplicate title prompt should be hidden, got %q", got)
	}
}

func TestSessionsViewShowsSubtitleOnlyForSelectedSession(t *testing.T) {
	m := NewSessions([]session.Session{
		{ID: "one", Title: "One", LastPrompt: "prompt one", CWD: "/repo/a", ModTime: time.Now()},
		{ID: "two", Title: "Two", LastPrompt: "prompt two", CWD: "/repo/a", ModTime: time.Now().Add(-time.Hour)},
	}).SetSize(40, 8)

	view := stripANSI(m.View())
	if !strings.Contains(view, "prompt one") {
		t.Fatalf("selected session subtitle should be visible, got %q", view)
	}
	if strings.Contains(view, "prompt two") {
		t.Fatalf("unselected session subtitle should be hidden, got %q", view)
	}

	m, _ = m.Update(sessionsKey("down"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "prompt two") {
		t.Fatalf("new selected session subtitle should be visible, got %q", view)
	}
	if strings.Contains(view, "prompt one") {
		t.Fatalf("previously selected subtitle should be hidden, got %q", view)
	}
}

func TestSessionsHeaderCollapseExpand(t *testing.T) {
	total := defaultSessionGroupPageSize*2 + 5
	m := NewSessions(makeSessions(total, "/repo/a")).SetSize(40, 6)

	if len(m.rows) != defaultSessionGroupPageSize+2 {
		t.Fatalf("initial rows = %d, want header + page + more", len(m.rows))
	}
	m.cursor = len(m.rows) - 1
	m, _ = m.Update(sessionsKey("enter"))
	if m.groups[0].visibleCount != defaultSessionGroupPageSize*2 {
		t.Fatalf("visibleCount after load more = %d, want %d", m.groups[0].visibleCount, defaultSessionGroupPageSize*2)
	}

	m.cursor = 0
	m, _ = m.Update(sessionsKey("left"))
	if len(m.rows) != 1 {
		t.Fatalf("collapsed rows = %d, want 1", len(m.rows))
	}
	if m.groups[0].visibleCount != defaultSessionGroupPageSize {
		t.Fatalf("visibleCount after collapse = %d, want reset to %d", m.groups[0].visibleCount, defaultSessionGroupPageSize)
	}
	if row, _ := m.currentRow(); row.kind != sessionRowGroup {
		t.Fatalf("cursor should stay on group header after collapse, got %+v", row)
	}

	m, _ = m.Update(sessionsKey("right"))
	if m.groups[0].visibleCount != defaultSessionGroupPageSize {
		t.Fatalf("visibleCount after expand = %d, want reset to %d", m.groups[0].visibleCount, defaultSessionGroupPageSize)
	}
	if len(m.rows) != defaultSessionGroupPageSize+2 {
		t.Fatalf("expanded rows = %d, want header + page + more", len(m.rows))
	}
}

func TestSessionsLeftOnSessionMovesToHeader(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetSize(40, 4)
	if row, _ := m.currentRow(); row.kind != sessionRowSession {
		t.Fatalf("cursor should be on session before left, got %+v", row)
	}

	m, _ = m.Update(sessionsKey("left"))
	if row, _ := m.currentRow(); row.kind != sessionRowGroup {
		t.Fatalf("left on session should move to group header, got %+v", row)
	}
}

func TestSessionsEnterOnSessionEmitsChosenMsg(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a/sub", ProjectDir: "/repo/a", Title: "One"}}).SetSize(40, 4)
	_, cmd := m.Update(sessionsKey("enter"))
	if cmd == nil {
		t.Fatal("enter on session should emit command")
	}
	msg, ok := cmd().(sessionChosenMsg)
	if !ok {
		t.Fatalf("command emitted %T, want sessionChosenMsg", cmd())
	}
	if msg.id != "one" || msg.cwd != "/repo/a/sub" || msg.projectDir != "/repo/a" {
		t.Fatalf("message = %+v, want id one cwd /repo/a/sub projectDir /repo/a", msg)
	}
}

func TestSessionsEnterOnSessionFallsBackToCWDForExplorerRoot(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetSize(40, 4)
	_, cmd := m.Update(sessionsKey("enter"))
	if cmd == nil {
		t.Fatal("enter on session should emit command")
	}
	msg := cmd().(sessionChosenMsg)
	if msg.projectDir != "/repo/a" {
		t.Fatalf("projectDir = %q, want cwd fallback /repo/a", msg.projectDir)
	}
}

func TestSessionsFilterKeepsHeaderForMatch(t *testing.T) {
	m := NewSessions([]session.Session{
		{ID: "one", CWD: "/repo/a", Title: "One", LastPrompt: "unique needle"},
		{ID: "two", CWD: "/repo/b", Title: "Two", LastPrompt: "other"},
	}).SetSize(60, 6)
	m.filter = "needle"
	m = m.rebuildRows()
	m.cursor = 0
	m.offset = 0

	if len(m.rows) != 2 {
		t.Fatalf("filtered rows = %d, want header + matching session", len(m.rows))
	}
	if m.rows[0].kind != sessionRowGroup || m.rows[1].kind != sessionRowSession {
		t.Fatalf("filtered rows should keep group header then session, got %+v", m.rows)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "a") || !strings.Contains(view, "One") || strings.Contains(view, "Two") {
		t.Fatalf("filtered view should include matching group/session only, got %q", view)
	}
}
