package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"cc-sidecar/internal/session"
	"cc-sidecar/internal/ui"
)

type config struct {
	projectsDir string
	initialRoot string
	altScreen   bool
}

func main() {
	cfg := parseFlags()
	if err := validateDir("projects-dir", cfg.projectsDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.initialRoot != "" {
		if err := validateDir("root", cfg.initialRoot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	source := session.NewFilesystemSessionSource(cfg.projectsDir)
	metas, err := source.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "扫描会话失败:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewRoot(metas, ui.Options{
		InitialRoot:   cfg.initialRoot,
		AltScreen:     cfg.altScreen,
		SessionSource: source,
	}))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{altScreen: true}
	flag.StringVar(&cfg.projectsDir, "projects-dir", session.ProjectsDir(), "Claude Code projects directory")
	flag.StringVar(&cfg.initialRoot, "root", "", "initial directory for the tree panel")
	flag.BoolVar(&cfg.altScreen, "alt-screen", true, "render in the terminal alternate screen")
	noAltScreen := flag.Bool("no-alt-screen", false, "disable alternate screen rendering")
	flag.Parse()
	if *noAltScreen {
		cfg.altScreen = false
	}
	return cfg
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
