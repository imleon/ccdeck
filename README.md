# ccdeck

`ccdeck` is a zellij-first terminal UI for browsing local Claude Code sessions,
project files, and selected file contents without pushing browsing output into a
Claude Code conversation.

`ccdeck` is the only public entrypoint. Running it starts zellij directly and
boots four coordinated panes inside one runtime:

- `sessions`: browse local Claude Code sessions
- `claude`: preview the Claude command for the selected session without running it
- `file`: preview the file selected in explorer
- `explorer`: browse the selected session's working tree

Selecting a session updates the linked panes automatically. The current
implementation keeps the browser sidecar separate from the Claude Code
conversation, while using zellij as the runtime shell.

## Build

This project uses Go 1.26+ and Bubble Tea v2.

```bash
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go test ./...
/usr/local/go/bin/go build -buildvcs=false -o ccdeck .
```

`-buildvcs=false` is used because this directory may not be inside a complete
Git checkout, and recent Go versions try to stamp VCS metadata by default.

## Run

Start the default deck:

```bash
./ccdeck
./ccdeck --projects-dir "$HOME/.claude/projects"
./ccdeck --deck dev
```

This launches zellij directly and boots one coordinated ccdeck runtime.

### Flags

- `--projects-dir <dir>`: override the Claude Code projects directory. Defaults
  to `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects`.
- `--deck <name>`: explicitly name the runtime. If omitted, ccdeck generates
  one automatically.

Alternate-screen is always enabled for the Bubble Tea panes.

## Key bindings

Sessions:

- `↑` / `↓` or `j` / `k`: move selection
- `/`: filter sessions
- `Enter`: activate the selected session and sync Claude / explorer / file panes

Explorer:

- `↑` / `↓` or `j` / `k`: move selection
- `Enter` / `l` / `→`: expand a directory, or select a file and notify file pane
- `h` / `←`: collapse the current directory; if it is already collapsed, move to
  its parent directory
- Uses fixed-width Nerd Font icons for folders and common file types. If icons
  render as squares, switch the terminal font to a Nerd Font.

File:

- Uses Bubble Tea's viewport key bindings for scrolling.
- `w`: toggle soft wrap.
- `r`: reload the current file.

Claude pane:

- Waits for session activation from sessions pane.
- Records the Claude command preview for the selected session without running it.

## Current scope

This tool intentionally keeps the browsing sidecar separate from the Claude Code
conversation. It does not:

- inject transcript raw content into Claude Code conversation context
- proxy Claude Code terminal interaction inside Bubble Tea
- persist deck state beyond the current zellij runtime
- automatically interpret explorer/file output as conversation input

The current implementation already starts zellij and links panes automatically.
Internal pane modes remain implementation details behind the `ccdeck` entrypoint.
