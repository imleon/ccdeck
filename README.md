# cc-session

`cc-session` is a terminal UI for browsing local Claude Code sessions and their
working directories without putting session lists or file browsing output into a
Claude Code conversation.

It runs as a separate process from Claude Code:

- left panel: Claude Code sessions
- middle panel: directory tree for the selected session's working directory
- right panel: syntax-highlighted file viewer with line numbers

It does **not** automatically resume sessions, control tmux, or embed Claude
Code. When you select a session, it only shows the command you can run yourself.

## Build

This project uses Go 1.26+ and Bubble Tea v2.

```bash
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go test ./...
/usr/local/go/bin/go build -buildvcs=false -o cc-session .
```

`-buildvcs=false` is used because this directory may not be inside a complete
Git checkout, and recent Go versions try to stamp VCS metadata by default.

## Run

```bash
./cc-session
```

Useful flags:

```bash
./cc-session --help
./cc-session --projects-dir "$HOME/.claude/projects"
./cc-session --root /data00/home/gaolei.veew/sourcecode/cc-session
./cc-session --no-alt-screen
```

### Flags

- `--projects-dir <dir>`: override the Claude Code projects directory. Defaults
  to `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects`.
- `--root <dir>`: set the initial directory shown in the tree panel. If omitted,
  the tree follows the currently highlighted session's working directory.
- `--no-alt-screen`: disable alternate-screen rendering, useful for debugging or
  capturing output.
- `--alt-screen`: explicitly enable alternate-screen rendering. Enabled by
  default.

## Key bindings

Global:

- `Tab`: focus next panel
- `Shift+Tab`: focus previous panel
- `?`: toggle help
- `q` / `Ctrl+C`: quit

Sessions panel:

- `↑` / `↓` or `j` / `k`: move selection
- `/`: filter sessions
- `Enter`: show `cd <cwd> && claude --resume <session-id>` in the status line

Tree panel:

- `↑` / `↓` or `j` / `k`: move selection
- `Enter` / `l` / `→`: expand a directory or open a file in the viewer
- `h` / `←`: collapse a directory
- Uses fixed-width Nerd Font icons for folders and common file types. If icons render as squares, switch the terminal font to a Nerd Font; ASCII fallback support is kept in code for a future flag.

Viewer panel:

- Uses Bubble Tea's viewport key bindings for scrolling.

## Current scope

This tool is intentionally a standalone browser. It does not:

- auto-run `claude --resume`
- send commands into an existing Claude Code session
- manage tmux panes
- embed Claude Code inside the TUI
- send session-list output into a Claude Code conversation

Those integrations can be added later as a separate phase.
