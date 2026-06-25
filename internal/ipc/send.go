package ipc

import (
	"context"
	"encoding/json"
	"net"
	"time"
)

const defaultSendTimeout = 200 * time.Millisecond

type Sender struct {
	GroupName  string
	RuntimeDir string
	Timeout    time.Duration
}

func (s Sender) SendToExplorer(ctx context.Context, ev Event) error {
	return s.send(ctx, RoleExplorer, ev)
}

func (s Sender) SendToFile(ctx context.Context, ev Event) error {
	return s.send(ctx, RoleFile, ev)
}

func (s Sender) SendToClaude(ctx context.Context, ev Event) error {
	return s.send(ctx, RoleClaude, ev)
}

func (s Sender) send(ctx context.Context, role Role, ev Event) error {
	path, err := SocketPath(s.GroupName, role, s.RuntimeDir)
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultSendTimeout
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return err
	}
	defer conn.Close()
	return json.NewEncoder(conn).Encode(ev)
}
