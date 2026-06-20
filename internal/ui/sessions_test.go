package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"cc-sidecar/internal/session"
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
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
	line := renderSessionTitleLine("a very long session title", "15D", 8, false)
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

func TestSessionSubtitleOmitsBranchAndDuplicateTitle(t *testing.T) {
	s := session.Session{
		Title:      "Fix footer",
		LastPrompt: "Investigate truncation",
		CWD:        "/data00/home/gaolei.veew/sourcecode/cc-sidecar",
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

	m, _ = m.Update(sessionsKey("down"))
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
	m := NewSessions([]session.Session{
		{ID: "one", CWD: "/repo/a", Title: "One"},
		{ID: "two", CWD: "/repo/a", Title: "Two"},
	}).SetSize(40, 6)

	if len(m.rows) != 3 {
		t.Fatalf("initial rows = %d, want 3", len(m.rows))
	}
	m, _ = m.Update(sessionsKey("left"))
	if len(m.rows) != 1 {
		t.Fatalf("collapsed rows = %d, want 1", len(m.rows))
	}
	if row, _ := m.currentRow(); row.kind != sessionRowGroup {
		t.Fatalf("cursor should stay on group header after collapse, got %+v", row)
	}

	m, _ = m.Update(sessionsKey("right"))
	if len(m.rows) != 3 {
		t.Fatalf("expanded rows = %d, want 3", len(m.rows))
	}
}

func TestSessionsLeftOnSessionMovesToHeader(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetSize(40, 4)
	m, _ = m.Update(sessionsKey("down"))
	if row, _ := m.currentRow(); row.kind != sessionRowSession {
		t.Fatalf("cursor should be on session before left, got %+v", row)
	}

	m, _ = m.Update(sessionsKey("left"))
	if row, _ := m.currentRow(); row.kind != sessionRowGroup {
		t.Fatalf("left on session should move to group header, got %+v", row)
	}
}

func TestSessionsEnterOnSessionEmitsChosenMsg(t *testing.T) {
	m := NewSessions([]session.Session{{ID: "one", CWD: "/repo/a", Title: "One"}}).SetSize(40, 4)
	m, _ = m.Update(sessionsKey("down"))

	_, cmd := m.Update(sessionsKey("enter"))
	if cmd == nil {
		t.Fatal("enter on session should emit command")
	}
	msg, ok := cmd().(sessionChosenMsg)
	if !ok {
		t.Fatalf("command emitted %T, want sessionChosenMsg", cmd())
	}
	if msg.id != "one" || msg.cwd != "/repo/a" {
		t.Fatalf("message = %+v, want id one cwd /repo/a", msg)
	}
}

func TestSessionsFilterKeepsHeaderForMatch(t *testing.T) {
	m := NewSessions([]session.Session{
		{ID: "one", CWD: "/repo/a", Title: "One", LastPrompt: "unique needle"},
		{ID: "two", CWD: "/repo/b", Title: "Two", LastPrompt: "other"},
	}).SetSize(60, 6)
	m.filter = "needle"
	m = m.rebuildRows()

	if len(m.rows) != 2 {
		t.Fatalf("filtered rows = %d, want header + matching session", len(m.rows))
	}
	if m.rows[0].kind != sessionRowGroup || m.rows[1].kind != sessionRowSession {
		t.Fatalf("filtered rows should keep group header then session, got %+v", m.rows)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "/repo/a") || !strings.Contains(view, "One") || strings.Contains(view, "Two") {
		t.Fatalf("filtered view should include matching group/session only, got %q", view)
	}
}
