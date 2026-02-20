# herd

A TUI for managing multiple [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions in tmux.

![herd demo](https://github.com/shnupta/herd/raw/main/demo.gif)

## Install

```bash
go install github.com/shnupta/herd@latest
```

Requires:
- Go 1.21+
- tmux
- Claude Code CLI (`claude`)

## Usage

Start herd inside a tmux session:

```bash
herd
```

On first run, install the Claude hooks to enable status tracking:

```
I (capital i) — install hooks
```

## Features

### Session Management
- **Live viewport** — see Claude's output in real-time without switching panes
- **Session list** — all Claude sessions across tmux, with status indicators
- **Auto-discovery** — new sessions appear automatically, dead ones disappear
- **Status tracking** — working / waiting / idle / plan_ready via Claude hooks

### Navigation & Control
| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate session list |
| `J/K` | Move session up/down (reorder) |
| `p` | Pin/unpin session to top |
| `/` | Filter sessions |
| `i` | Insert mode (type into Claude) |
| `ctrl+h` | Exit insert mode |
| `t` | Jump to pane (switch tmux focus) |
| `n` | New session (project picker) |
| `x` | Kill session |
| `d` | Diff review mode |
| `r` | Refresh session list |
| `I` | Install Claude hooks |
| `q` | Quit |

### Diff Review
Press `d` to review uncommitted changes in the selected session's project. Add inline comments and submit feedback directly to the Claude session.

### Persistence
Session pins and ordering are saved to `~/.herd/sidebar.json` and restored on restart.

## Configuration

Create `~/.herd/config.json`:

```json
{
  "project_dirs": [
    "~/code",
    "~/work",
    "~/projects"
  ],
  "dangerously_skip_permissions": true
}
```

### Options

| Field | Description | Default |
|-------|-------------|---------|
| `project_dirs` | Directories to scan for projects in the new session picker | `["~"]` |
| `dangerously_skip_permissions` | Launch Claude with `--dangerously-skip-permissions` flag | `false` |

## How It Works

1. **Session discovery**: Scans `tmux list-panes` for processes named `claude` or matching a semver pattern (e.g., `2.1.47`)
2. **Status tracking**: Claude hooks write state to `~/.herd/state/` which herd watches via fsnotify
3. **Live capture**: Polls `tmux capture-pane` to show Claude's output in the viewport

## License

MIT
