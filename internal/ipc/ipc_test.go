package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuntimeDirSelection(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		goos string
		want string
	}{
		{name: "explicit", env: map[string]string{runtimeEnvVar: "/run/custom", "XDG_RUNTIME_DIR": "/run/user/1"}, goos: "linux", want: "/run/custom"},
		{name: "xdg", env: map[string]string{"XDG_RUNTIME_DIR": "/run/user/1"}, goos: "linux", want: "/run/user/1/ccdeck"},
		{name: "darwin", env: map[string]string{}, goos: "darwin", want: "/home/me/Library/Application Support/ccdeck/runtime"},
		{name: "linux fallback", env: map[string]string{}, goos: "linux", want: "/home/me/.local/ccdeck/runtime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtimeDir(func(k string) string { return tt.env[k] }, func() (string, error) { return "/home/me", nil }, tt.goos)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("runtimeDir = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSocketPathAndGroupValidation(t *testing.T) {
	dir := t.TempDir()
	pathSessions, err := SocketPath("dev", RoleSessions, dir)
	if err != nil {
		t.Fatal(err)
	}
	pathExplorer, err := SocketPath("dev", RoleExplorer, dir)
	if err != nil {
		t.Fatal(err)
	}
	pathFile, err := SocketPath("dev", RoleFile, dir)
	if err != nil {
		t.Fatal(err)
	}
	if pathSessions == pathExplorer || pathExplorer == pathFile || pathSessions == pathFile {
		t.Fatalf("role paths should differ: %q %q %q", pathSessions, pathExplorer, pathFile)
	}
	if filepath.Dir(pathExplorer) != dir {
		t.Fatalf("socket dir = %q, want %q", filepath.Dir(pathExplorer), dir)
	}
	for _, name := range []string{"", "../x", "/tmp/x", "bad name"} {
		if _, err := SocketPath(name, RoleExplorer, dir); err == nil {
			t.Fatalf("SocketPath(%q) succeeded, want error", name)
		}
	}
}

func TestListenSendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	listener, err := Listen("dev", RoleExplorer, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	sender := Sender{GroupName: "dev", RuntimeDir: dir, Timeout: time.Second}
	sent := NewSetRoot("/repo", "abc")
	done := make(chan Event, 1)
	errs := make(chan error, 1)
	go func() {
		ev, err := listener.AcceptEvent()
		if err != nil {
			errs <- err
			return
		}
		done <- ev
	}()
	if err := sender.SendToExplorer(context.Background(), sent); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errs:
		t.Fatal(err)
	case got := <-done:
		if got.Type != TypeSetRoot || got.Path != "/repo" || got.SessionID != "abc" {
			t.Fatalf("event = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestListenRejectsRunningReceiver(t *testing.T) {
	dir := t.TempDir()
	listener, err := Listen("dev", RoleFile, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if _, err := Listen("dev", RoleFile, dir); err == nil {
		t.Fatal("second listener should fail")
	}
}

func TestListenRemovesStaleSocket(t *testing.T) {
	dir := t.TempDir()
	path, err := SocketPath("dev", RoleExplorer, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := Listen("dev", RoleExplorer, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if listener.Path() != path {
		t.Fatalf("path = %q, want %q", listener.Path(), path)
	}
}

func TestEnsureRuntimeDirMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runtime")
	if err := ensureRuntimeDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("mode = %o, want 700", got)
	}
}
