package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Listener struct {
	groupName string
	role      Role
	path      string
	ln        net.Listener
}

func Listen(groupName string, role Role, runtimeDirOverride string) (*Listener, error) {
	path, err := SocketPath(groupName, role, runtimeDirOverride)
	if err != nil {
		return nil, err
	}
	if err := ensureRuntimeDir(runtimeDirOverrideOrParent(path, runtimeDirOverride)); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		if conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond); dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("group receiver already running: %s/%s", groupName, role)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	return &Listener{groupName: groupName, role: role, path: path, ln: ln}, nil
}

func runtimeDirOverrideOrParent(path, runtimeDirOverride string) string {
	if runtimeDirOverride != "" {
		return runtimeDirOverride
	}
	return dirName(path)
}

func dirName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			if i == 0 {
				return string(path[0])
			}
			return path[:i]
		}
	}
	return "."
}

func (l *Listener) AcceptEvent() (Event, error) {
	conn, err := l.ln.Accept()
	if err != nil {
		return Event{}, err
	}
	defer conn.Close()
	var ev Event
	if err := json.NewDecoder(conn).Decode(&ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}

func (l *Listener) Close() error {
	var closeErr error
	if l.ln != nil {
		closeErr = l.ln.Close()
	}
	if l.path != "" {
		if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (l *Listener) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
