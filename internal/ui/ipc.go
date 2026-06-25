package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/ipc"
)

type ipcSetRootMsg struct {
	path      string
	sessionID string
}

type ipcOpenFileMsg struct {
	path string
}

type ipcActivateChatMsg struct {
	sessionID string
	cwd       string
}

type ipcClearFileMsg struct {
	root string
}

type ipcReceiveErrorMsg struct {
	err error
}

type ipcSendResultMsg struct {
	target    string
	eventType string
	err       error
}

func waitIPCCmd(listener *ipc.Listener) tea.Cmd {
	if listener == nil {
		return nil
	}
	return func() tea.Msg {
		ev, err := listener.AcceptEvent()
		if err != nil {
			return ipcReceiveErrorMsg{err: err}
		}
		switch ev.Type {
		case ipc.TypeSetRoot:
			return ipcSetRootMsg{path: ev.Path, sessionID: ev.SessionID}
		case ipc.TypeOpenFile:
			return ipcOpenFileMsg{path: ev.Path}
		case ipc.TypeActivateChat:
			return ipcActivateChatMsg{sessionID: ev.SessionID, cwd: ev.Path}
		case ipc.TypeClearFile:
			return ipcClearFileMsg{root: ev.Root}
		default:
			return ipcReceiveErrorMsg{err: fmt.Errorf("unknown ipc event: %s", ev.Type)}
		}
	}
}

func sendSetRootCmd(sender ipc.Sender, path, sessionID string) tea.Cmd {
	if sender.GroupName == "" {
		return nil
	}
	if path == "" {
		return func() tea.Msg {
			return ipcSendResultMsg{target: "explorer", eventType: ipc.TypeSetRoot, err: errors.New("session has no project dir")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout(sender))
		defer cancel()
		err := sender.SendToExplorer(ctx, ipc.NewSetRoot(path, sessionID))
		return ipcSendResultMsg{target: "explorer", eventType: ipc.TypeSetRoot, err: err}
	}
}

func sendOpenFileCmd(sender ipc.Sender, path, root string) tea.Cmd {
	if sender.GroupName == "" || path == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout(sender))
		defer cancel()
		err := sender.SendToFile(ctx, ipc.NewOpenFile(path, root))
		return ipcSendResultMsg{target: "file", eventType: ipc.TypeOpenFile, err: err}
	}
}

func sendActivateChatCmd(sender ipc.Sender, sessionID, cwd string) tea.Cmd {
	if sender.GroupName == "" || sessionID == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout(sender))
		defer cancel()
		err := sender.SendToClaude(ctx, ipc.NewActivateChat(sessionID, cwd))
		return ipcSendResultMsg{target: "claude", eventType: ipc.TypeActivateChat, err: err}
	}
}

func sendClearFileCmd(sender ipc.Sender, root string) tea.Cmd {
	if sender.GroupName == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout(sender))
		defer cancel()
		err := sender.SendToFile(ctx, ipc.NewClearFile(root))
		return ipcSendResultMsg{target: "file", eventType: ipc.TypeClearFile, err: err}
	}
}

func sendTimeout(sender ipc.Sender) time.Duration {
	if sender.Timeout > 0 {
		return sender.Timeout
	}
	return 250 * time.Millisecond
}
