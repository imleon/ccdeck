package main

import (
	"regexp"
	"testing"
	"time"
)

func TestParseArgsDefaultStartsDeck(t *testing.T) {
	cfg, err := parseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.mode != modeDeck {
		t.Fatalf("mode = %q, want deck", cfg.mode)
	}
	if cfg.groupName == "" {
		t.Fatal("default deck should generate a group name")
	}
}

func TestParseArgsDeckFlags(t *testing.T) {
	cfg, err := parseArgs([]string{"--projects-dir", "/tmp/projects", "--deck", "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.mode != modeDeck {
		t.Fatalf("mode = %q, want deck", cfg.mode)
	}
	if cfg.projectsDir != "/tmp/projects" || cfg.groupName != "dev" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseArgsRejectsRemovedAltScreenFlag(t *testing.T) {
	if _, err := parseArgs([]string{"--alt-screen=false"}); err == nil {
		t.Fatal("parseArgs should reject removed --alt-screen flag")
	}
}

func TestParseArgsInternalSessions(t *testing.T) {
	cfg, err := parseArgs([]string{string(modeInternalSessions), "--projects-dir", "/tmp/projects", "--group", "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.mode != modeInternalSessions || cfg.projectsDir != "/tmp/projects" || cfg.groupName != "dev" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseArgsInternalExplorerRequiresGroupAtValidation(t *testing.T) {
	cfg, err := parseArgs([]string{string(modeInternalExplorer)})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("explorer without --group should fail validation")
	}
}

func TestParseArgsInternalExplorerGroup(t *testing.T) {
	cfg, err := parseArgs([]string{string(modeInternalExplorer), "--group", "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.mode != modeInternalExplorer || cfg.groupName != "dev" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseArgsInternalFileRequiresGroupAtValidation(t *testing.T) {
	cfg, err := parseArgs([]string{string(modeInternalFile)})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("file without --group should fail validation")
	}
}

func TestParseArgsInternalClaudeRequiresGroupAtValidation(t *testing.T) {
	cfg, err := parseArgs([]string{string(modeInternalClaude)})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("claude host without --group should fail validation")
	}
}

func TestParseArgsRejectsRemovedPositionalArgs(t *testing.T) {
	cases := [][]string{
		{"workspace"},
		{"sessions"},
		{"explorer"},
		{"file"},
		{"unknown"},
		{"--deck", "dev", "extra"},
	}
	for _, args := range cases {
		if _, err := parseArgs(args); err == nil {
			t.Fatalf("parseArgs(%v) succeeded, want error", args)
		}
	}
}

func TestValidateConfigByMode(t *testing.T) {
	projects := t.TempDir()
	cases := []config{
		{mode: modeHelp},
		{mode: modeDeck, projectsDir: projects, groupName: "dev"},
		{mode: modeInternalSessions, projectsDir: projects, groupName: "dev"},
		{mode: modeInternalExplorer, groupName: "dev"},
		{mode: modeInternalFile, groupName: "dev"},
		{mode: modeInternalClaude, groupName: "dev"},
	}
	for _, cfg := range cases {
		if err := validateConfig(cfg); err != nil {
			t.Fatalf("validateConfig(%+v): %v", cfg, err)
		}
	}
}

func TestGenerateGroupName(t *testing.T) {
	got := generateGroupName(time.Date(2026, 6, 21, 14, 30, 25, 0, time.UTC), 12345)
	want := "ccdeck-20260621-143025-p12345"
	if got != want {
		t.Fatalf("group = %q, want %q", got, want)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(got) {
		t.Fatalf("group contains invalid chars: %q", got)
	}
}
