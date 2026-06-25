package ui

import tea "charm.land/bubbletea/v2"

// FileModel is the product-level selected-file panel.
// It currently delegates rendering and scrolling to ViewerModel.
type FileModel struct {
	viewer ViewerModel
}

func NewFile() FileModel {
	return FileModel{viewer: NewViewer()}
}

func (m FileModel) Path() string {
	return m.viewer.Path()
}

func (m FileModel) Status() string {
	return m.viewer.Status()
}

func (m FileModel) WrapStatus() string {
	return m.viewer.WrapStatus()
}

func (m FileModel) SetSize(w, h int) FileModel {
	m.viewer = m.viewer.SetSize(w, h)
	return m
}

func (m FileModel) LoadPath(path string) (FileModel, tea.Cmd) {
	viewer, cmd := m.viewer.LoadFile(path)
	m.viewer = viewer
	return m, cmd
}

func (m FileModel) Refresh() (FileModel, tea.Cmd) {
	viewer, cmd := m.viewer.Refresh()
	m.viewer = viewer
	return m, cmd
}

func (m FileModel) Clear() FileModel {
	viewer := NewViewer()
	viewer = viewer.SetSize(m.viewer.vp.Width(), m.viewer.vp.Height())
	viewer.vp.SoftWrap = m.viewer.vp.SoftWrap
	m.viewer = viewer
	return m
}

func (m FileModel) Update(msg tea.Msg) (FileModel, tea.Cmd) {
	viewer, cmd := m.viewer.Update(msg)
	m.viewer = viewer
	return m, cmd
}

func (m FileModel) View() string {
	return m.viewer.View()
}
