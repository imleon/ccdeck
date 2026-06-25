package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"ccdeck/internal/deck"
	"ccdeck/internal/ipc"
	"ccdeck/internal/session"
	"ccdeck/internal/ui"
)

type runMode string

const (
	modeHelp             runMode = "help"
	modeDeck             runMode = "deck"
	modeInternalSessions runMode = "__sessions"
	modeInternalExplorer runMode = "__explorer"
	modeInternalFile     runMode = "__file"
	modeInternalClaude   runMode = deck.ClaudeHostModeMarker
)

type config struct {
	mode        runMode
	projectsDir string
	groupName   string
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(os.Stdout)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.mode == modeHelp {
		printUsage(os.Stdout)
		return
	}
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	switch cfg.mode {
	case modeDeck:
		err = runDeck(cfg)
	case modeInternalSessions:
		err = runSessions(cfg)
	case modeInternalExplorer:
		err = runExplorer(cfg)
	case modeInternalFile:
		err = runFile(cfg)
	case modeInternalClaude:
		err = runClaudeHost(cfg)
	default:
		err = fmt.Errorf("未知模式: %s", cfg.mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDeck(cfg config) error {
	if err := deck.EnsureZellij(); err != nil {
		return err
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件失败: %w", err)
	}
	layout := deck.BuildLayout(
		execPath,
		cfg.groupName,
		cfg.projectsDir,
		string(modeInternalSessions),
		string(modeInternalExplorer),
		string(modeInternalFile),
		string(modeInternalClaude),
	)
	layoutFile, err := os.CreateTemp("", "ccdeck-layout-*.kdl")
	if err != nil {
		return fmt.Errorf("创建 zellij layout 文件失败: %w", err)
	}
	defer os.Remove(layoutFile.Name())
	if _, err := layoutFile.WriteString(layout); err != nil {
		layoutFile.Close()
		return fmt.Errorf("写入 zellij layout 文件失败: %w", err)
	}
	if err := layoutFile.Close(); err != nil {
		return fmt.Errorf("关闭 zellij layout 文件失败: %w", err)
	}
	return deck.StartSession(context.Background(), cfg.groupName, layoutFile.Name())
}

func runSessions(cfg config) error {
	presence, err := ipc.Listen(cfg.groupName, ipc.RoleSessions, "")
	if err != nil {
		return err
	}
	defer presence.Close()

	source := session.NewFilesystemSessionSource(cfg.projectsDir)
	metas, err := source.List()
	if err != nil {
		return fmt.Errorf("扫描会话失败: %w", err)
	}

	sender := ipc.Sender{GroupName: cfg.groupName}
	p := tea.NewProgram(ui.NewSessionsApp(metas, ui.SessionsAppOptions{
		SessionSource: source,
		GroupName:     cfg.groupName,
		SetRootSender: sender,
		ClaudeSender:  sender,
		IPCPresence:   presence,
	}))
	_, err = p.Run()
	return err
}

func runExplorer(cfg config) error {
	listener, err := ipc.Listen(cfg.groupName, ipc.RoleExplorer, "")
	if err != nil {
		return err
	}
	defer listener.Close()

	sender := ipc.Sender{GroupName: cfg.groupName}
	p := tea.NewProgram(ui.NewExplorerApp(ui.ExplorerAppOptions{
		GroupName:      cfg.groupName,
		IPCListener:    listener,
		OpenFileSender: sender,
		FileSender:     sender,
	}))
	_, err = p.Run()
	return err
}

func runFile(cfg config) error {
	listener, err := ipc.Listen(cfg.groupName, ipc.RoleFile, "")
	if err != nil {
		return err
	}
	defer listener.Close()

	p := tea.NewProgram(ui.NewFileApp(ui.FileAppOptions{
		GroupName:   cfg.groupName,
		IPCListener: listener,
	}))
	_, err = p.Run()
	return err
}

func runClaudeHost(cfg config) error {
	listener, err := ipc.Listen(cfg.groupName, ipc.RoleClaude, "")
	if err != nil {
		return err
	}
	defer listener.Close()

	p := tea.NewProgram(ui.NewClaudeApp(ui.ClaudeAppOptions{
		GroupName:   cfg.groupName,
		IPCListener: listener,
	}))
	_, err = p.Run()
	return err
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		mode:        modeDeck,
		projectsDir: session.ProjectsDir(),
	}
	if len(args) == 0 {
		cfg.groupName = generateGroupName(time.Now(), os.Getpid())
		return cfg, nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		return config{mode: modeHelp}, nil
	case string(modeInternalSessions):
		cfg.mode = modeInternalSessions
		return cfg, parseSessionsArgs(args[1:], &cfg)
	case string(modeInternalExplorer):
		cfg.mode = modeInternalExplorer
		return cfg, parseExplorerArgs(args[1:], &cfg)
	case string(modeInternalFile):
		cfg.mode = modeInternalFile
		return cfg, parseFileArgs(args[1:], &cfg)
	case string(modeInternalClaude):
		cfg.mode = modeInternalClaude
		return cfg, parseClaudeHostArgs(args[1:], &cfg)
	default:
		if err := parseDeckArgs(args, &cfg); err != nil {
			return cfg, err
		}
		if cfg.groupName == "" {
			cfg.groupName = generateGroupName(time.Now(), os.Getpid())
		}
		return cfg, nil
	}
}

func parseDeckArgs(args []string, cfg *config) error {
	fs := newFlagSet("ccdeck")
	fs.StringVar(&cfg.projectsDir, "projects-dir", cfg.projectsDir, "Claude Code projects directory")
	fs.StringVar(&cfg.groupName, "deck", "", "runtime name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("ccdeck 不接受位置参数: %v", fs.Args())
	}
	return nil
}

func parseSessionsArgs(args []string, cfg *config) error {
	fs := newFlagSet(string(modeInternalSessions))
	fs.StringVar(&cfg.projectsDir, "projects-dir", cfg.projectsDir, "Claude Code projects directory")
	fs.StringVar(&cfg.groupName, "group", "", "runtime name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("sessions 不接受位置参数: %v", fs.Args())
	}
	return nil
}

func parseExplorerArgs(args []string, cfg *config) error {
	fs := newFlagSet(string(modeInternalExplorer))
	fs.StringVar(&cfg.groupName, "group", "", "runtime name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("explorer 不接受位置参数: %v", fs.Args())
	}
	return nil
}

func parseFileArgs(args []string, cfg *config) error {
	fs := newFlagSet(string(modeInternalFile))
	fs.StringVar(&cfg.groupName, "group", "", "runtime name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("file 不接受位置参数: %v", fs.Args())
	}
	return nil
}

func parseClaudeHostArgs(args []string, cfg *config) error {
	fs := newFlagSet(string(modeInternalClaude))
	fs.StringVar(&cfg.groupName, "group", "", "runtime name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("claude-host 不接受位置参数: %v", fs.Args())
	}
	return nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func generateGroupName(now time.Time, pid int) string {
	return fmt.Sprintf("ccdeck-%s-p%d", now.Format("20060102-150405"), pid)
}

func validateConfig(cfg config) error {
	switch cfg.mode {
	case modeHelp:
		return nil
	case modeDeck, modeInternalSessions:
		if err := validateDir("projects-dir", cfg.projectsDir); err != nil {
			return err
		}
		return ipc.ValidateGroupName(cfg.groupName)
	case modeInternalExplorer, modeInternalFile, modeInternalClaude:
		if cfg.groupName == "" {
			return fmt.Errorf("%s 模式需要 --group", cfg.mode)
		}
		return ipc.ValidateGroupName(cfg.groupName)
	default:
		return fmt.Errorf("未知模式: %s", cfg.mode)
	}
}

func validateDir(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s 不可访问: %s: %w", name, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录: %s", name, path)
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `ccdeck

Usage:
  ccdeck [--projects-dir DIR] [--deck NAME]
      Start the ccdeck zellij runtime.

Examples:
  ccdeck
  ccdeck --projects-dir "$HOME/.claude/projects"
  ccdeck --deck dev

ccdeck is the only public entrypoint. It starts zellij directly and boots four
linked panes for sessions, Claude command preview, file, and explorer. The
internal __sessions / __explorer / __file / __claude-host modes are runtime
implementation details, not user-facing commands. Alternate-screen is always on.
`)
}
