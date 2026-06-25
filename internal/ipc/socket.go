package ipc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const runtimeEnvVar = "CCDECK_RUNTIME_DIR"

type Role string

const (
	RoleSessions Role = "sessions"
	RoleExplorer Role = "explorer"
	RoleFile     Role = "file"
	RoleClaude   Role = "claude"
)

func RuntimeDir() (string, error) {
	return runtimeDir(os.Getenv, os.UserHomeDir, runtime.GOOS)
}

func runtimeDir(getenv func(string) string, homeDir func() (string, error), goos string) (string, error) {
	if dir := getenv(runtimeEnvVar); dir != "" {
		return dir, nil
	}
	if dir := getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "ccdeck"), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "ccdeck", "runtime"), nil
	}
	return filepath.Join(home, ".local", "ccdeck", "runtime"), nil
}

func SocketPath(groupName string, role Role, runtimeDirOverride string) (string, error) {
	if err := ValidateGroupName(groupName); err != nil {
		return "", err
	}
	if role != RoleSessions && role != RoleExplorer && role != RoleFile && role != RoleClaude {
		return "", fmt.Errorf("unknown ipc role: %s", role)
	}
	dir := runtimeDirOverride
	if dir == "" {
		var err error
		dir, err = RuntimeDir()
		if err != nil {
			return "", err
		}
	}
	hash := sha256.Sum256([]byte(groupName))
	name := hex.EncodeToString(hash[:])[:16] + "." + string(role) + ".sock"
	path := filepath.Join(dir, name)
	if len(path) >= 104 {
		return "", fmt.Errorf("ipc socket path too long: %s", path)
	}
	return path, nil
}

func ensureRuntimeDir(dir string) error {
	if dir == "" {
		return errors.New("empty runtime dir")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

func ValidateGroupName(name string) error {
	if name == "" {
		return errors.New("empty group name")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid group name: %s", name)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("invalid group name: %s", name)
	}
	return nil
}
