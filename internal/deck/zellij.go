package deck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	ClaudeHostModeMarker  = "__claude-host"
	ClaudePaneTitlePrefix = "ccdeck-claude:"
)

// EnsureZellij verifies that zellij is available in PATH.
func EnsureZellij() error {
	if _, err := exec.LookPath("zellij"); err != nil {
		return fmt.Errorf("未找到 zellij，请先安装后再运行 ccdeck: %w", err)
	}
	return nil
}

// BuildLayout returns a zellij layout that boots the sessions / Claude host /
// file / explorer panes for one ccdeck runtime.
func BuildLayout(executablePath, deckID, projectsDir string, sessionsMode, explorerMode, fileMode, claudeHostMode string) string {
	sessionsArgs := []string{sessionsMode, "--group", deckID, "--projects-dir", projectsDir}
	explorerArgs := []string{explorerMode, "--group", deckID}
	fileArgs := []string{fileMode, "--group", deckID}
	claudeHostArgs := []string{claudeHostMode, "--group", deckID}

	return fmt.Sprintf(`layout {
    pane size=1 borderless=true {
        plugin location="tab-bar"
    }
    pane split_direction="vertical" {
        pane size="15%%" name="Project" command=%s {
            %s
        }
        pane size="35%%" name="Claude Code" command=%s {
            %s
        }
        pane size="35%%" name="File" command=%s {
            %s
        }
        pane size="15%%" name="Explorer" command=%s {
            %s
        }
    }
    pane size=1 borderless=true {
        plugin location="status-bar"
    }
}
`,
		kdlQuote(executablePath), argsBlock(sessionsArgs),
		kdlQuote(executablePath), argsBlock(claudeHostArgs),
		kdlQuote(executablePath), argsBlock(fileArgs),
		kdlQuote(executablePath), argsBlock(explorerArgs),
	)
}

// StartSession replaces the current interactive flow with a zellij session using
// the supplied layout.
func StartSession(ctx context.Context, sessionName, layoutPath string) error {
	cmd := exec.CommandContext(ctx, "zellij", "-s", sessionName, "-n", layoutPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// LocalZellijPaneManager controls Claude panes from inside a running zellij pane.
// It relies on the current process already being attached to the target session.
type LocalZellijPaneManager struct {
	runner commandRunner
}

func NewLocalZellijPaneManager() *LocalZellijPaneManager {
	return &LocalZellijPaneManager{runner: execRunner{}}
}

func ClaudePaneTitle(sessionID string) string {
	return ClaudePaneTitlePrefix + sessionID
}

func (m *LocalZellijPaneManager) EnsureClaudePane(ctx context.Context, sessionID, cwd string) (string, error) {
	panes, err := m.listPanes(ctx)
	if err != nil {
		return "", err
	}
	paneTitle := ClaudePaneTitle(sessionID)
	if pane, ok := findPaneByTitle(panes, paneTitle); ok {
		if err := m.focusPane(ctx, pane.ID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已切换到 Claude pane: %s", sessionID), nil
	}

	args := []string{"run", "--stacked", "--name", paneTitle}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, "--", "claude", "--resume", sessionID)
	if _, err := m.runner.Run(ctx, "zellij", args...); err != nil {
		return "", fmt.Errorf("启动 Claude pane 失败: %w", err)
	}
	return fmt.Sprintf("已打开 Claude pane: %s", sessionID), nil
}

type paneInfo struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	PaneCommand string `json:"pane_command"`
	IsPlugin    bool   `json:"is_plugin"`
}

func (m *LocalZellijPaneManager) listPanes(ctx context.Context) ([]paneInfo, error) {
	out, err := m.runner.Run(ctx, "zellij", "action", "list-panes", "--json", "--all", "--tab", "--state", "--command")
	if err != nil {
		return nil, fmt.Errorf("读取 zellij panes 失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var panes []paneInfo
	if err := json.Unmarshal(out, &panes); err != nil {
		return nil, fmt.Errorf("解析 zellij panes 失败: %w", err)
	}
	return panes, nil
}

func (m *LocalZellijPaneManager) focusPane(ctx context.Context, paneID int) error {
	out, err := m.runner.Run(ctx, "zellij", "action", "focus-pane-id", strconv.Itoa(paneID))
	if err != nil {
		return fmt.Errorf("聚焦 pane %d 失败: %w: %s", paneID, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func findPaneByTitle(panes []paneInfo, title string) (paneInfo, bool) {
	for _, pane := range panes {
		if pane.IsPlugin {
			continue
		}
		if pane.Title == title {
			return pane, true
		}
	}
	return paneInfo{}, false
}

func findClaudeHostPane(panes []paneInfo) (paneInfo, bool) {
	for _, pane := range panes {
		if pane.IsPlugin {
			continue
		}
		if strings.Contains(pane.PaneCommand, ClaudeHostModeMarker) {
			return pane, true
		}
	}
	return paneInfo{}, false
}

func argsBlock(args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, "args")
	for _, arg := range args {
		parts = append(parts, kdlQuote(arg))
	}
	return strings.Join(parts, " ")
}

func kdlQuote(s string) string {
	return strconv.Quote(s)
}
