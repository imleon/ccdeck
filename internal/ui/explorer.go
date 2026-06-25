package ui

import (
	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/gitstatus"
)

// explorerOpenFileMsg is emitted when the Explorer selects a file for the File panel.
type explorerOpenFileMsg struct {
	path string
}

// ExplorerModel is the product-level project explorer panel.
// It currently wraps a directory tree and can grow to include git/status views.
type ExplorerModel struct {
	tree TreeModel
}

func NewExplorer() ExplorerModel {
	return ExplorerModel{tree: NewTree()}
}

func (m ExplorerModel) Root() string {
	return m.tree.Root()
}

func (m ExplorerModel) SetRoot(dir string) ExplorerModel {
	m.tree = m.tree.SetRoot(dir)
	return m
}

func (m ExplorerModel) SetGitStatus(files map[string]gitstatus.Status, root string) ExplorerModel {
	m.tree = m.tree.SetGitStatus(files, root)
	return m
}

func (m ExplorerModel) SetGitStatusMap(status map[string]gitstatus.Status) ExplorerModel {
	m.tree = m.tree.SetGitStatusMap(status)
	return m
}

func (m ExplorerModel) VisibleGitRepoRoots() []string {
	return m.tree.VisibleGitRepoRoots()
}

func (m ExplorerModel) Refresh() ExplorerModel {
	m.tree = m.tree.Refresh()
	return m
}

func (m ExplorerModel) SetSize(w, h int) ExplorerModel {
	m.tree = m.tree.SetSize(w, h)
	return m
}

func (m ExplorerModel) Update(msg tea.Msg) (ExplorerModel, tea.Cmd) {
	tree, cmd := m.tree.Update(msg)
	m.tree = tree
	return m, wrapExplorerCmd(cmd)
}

func (m ExplorerModel) View(openedPath string) string {
	return m.tree.View(openedPath)
}

func wrapExplorerCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if selected, ok := msg.(treeSelectFileMsg); ok {
			return explorerOpenFileMsg{path: selected.path}
		}
		return msg
	}
}
