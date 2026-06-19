package ui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"cc-session/internal/session"
)

// sessionChosenMsg is emitted when Enter is pressed on a selected session.
type sessionChosenMsg struct {
	id  string
	cwd string
}

// sessionItem adapts session.Session to bubbles/list.Item.
type sessionItem struct {
	s session.Session
}

func (i sessionItem) Title() string {
	return i.s.Title
}

func (i sessionItem) Description() string {
	when := i.s.ModTime.Format("01-02 15:04")
	cwd := i.s.CWD
	if cwd == "" {
		cwd = "?"
	}
	branch := i.s.GitBranch
	if branch == "" {
		branch = "-"
	}
	last := i.s.LastPrompt
	if last != "" && last != i.s.Title {
		last = " · " + last
	} else {
		last = ""
	}
	return fmt.Sprintf("%s · %s · %s%s", when, cwd, branch, last)
}

func (i sessionItem) FilterValue() string {
	return i.s.Title + " " + i.s.LastPrompt + " " + i.s.CWD + " " + i.s.GitBranch
}

// SessionsModel is the left panel: a searchable list of Claude Code sessions.
type SessionsModel struct {
	list list.Model
}

func NewSessions(sessions []session.Session) SessionsModel {
	items := make([]list.Item, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, sessionItem{s: s})
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(focusedBorderColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(focusedBorderColor)

	l := list.New(items, delegate, 0, 0)
	l.Title = "Claude Code Sessions"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	return SessionsModel{list: l}
}

func (m SessionsModel) SetSize(w, h int) SessionsModel {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	m.list.SetSize(w, h)
	return m
}

func (m SessionsModel) Selected() (session.Session, bool) {
	item, ok := m.list.SelectedItem().(sessionItem)
	return item.s, ok
}

func (m SessionsModel) CurrentCWD() string {
	if s, ok := m.Selected(); ok {
		return s.CWD
	}
	return ""
}

// Count returns the total number of sessions in the list.
func (m SessionsModel) Count() int {
	return len(m.list.Items())
}

// SelectedTitle returns the current selected session title for status display.
func (m SessionsModel) SelectedTitle() string {
	if s, ok := m.Selected(); ok {
		return s.Title
	}
	return ""
}

func (m SessionsModel) Update(msg tea.Msg) (SessionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "enter" && m.list.FilterState() != list.Filtering {
			if s, ok := m.Selected(); ok {
				return m, func() tea.Msg { return sessionChosenMsg{id: s.ID, cwd: s.CWD} }
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m SessionsModel) View() string {
	return m.list.View()
}
