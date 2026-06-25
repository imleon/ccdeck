package ipc

import "time"

const (
	ProtocolVersion = 1

	TypeSetRoot      = "set_root"
	TypeOpenFile     = "open_file"
	TypeActivateChat = "activate_chat"
	TypeClearFile    = "clear_file"

	SourceSessions = "sessions"
	SourceExplorer = "explorer"
)

// Event is the JSON envelope sent over a ccdeck Unix socket link.
type Event struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	Source    string `json:"source,omitempty"`
	Path      string `json:"path"`
	Root      string `json:"root,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	TS        string `json:"ts,omitempty"`
}

func NewSetRoot(path, sessionID string) Event {
	return Event{
		V:         ProtocolVersion,
		Type:      TypeSetRoot,
		Source:    SourceSessions,
		Path:      path,
		SessionID: sessionID,
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func NewOpenFile(path, root string) Event {
	return Event{
		V:      ProtocolVersion,
		Type:   TypeOpenFile,
		Source: SourceExplorer,
		Path:   path,
		Root:   root,
		TS:     time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func NewActivateChat(sessionID, cwd string) Event {
	return Event{
		V:         ProtocolVersion,
		Type:      TypeActivateChat,
		Source:    SourceSessions,
		Path:      cwd,
		SessionID: sessionID,
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func NewClearFile(root string) Event {
	return Event{
		V:      ProtocolVersion,
		Type:   TypeClearFile,
		Source: SourceSessions,
		Root:   root,
		TS:     time.Now().UTC().Format(time.RFC3339Nano),
	}
}
