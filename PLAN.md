# herd — development plan

## Done

- [x] TUI with session list pane and live `capture-pane` viewport
- [x] Session discovery via `tmux list-panes`, detecting Claude processes
- [x] Status indicators: working / waiting / plan_ready / notifying / idle
- [x] Hook installation into `~/.claude/settings.json`
- [x] Hook handler updating per-session state files
- [x] fsnotify watcher pushing hook events into the TUI event loop
- [x] Insert mode (`i` / `ctrl+h`) — forward keystrokes to Claude pane
- [x] Jump to pane (`t`) — resize-window restore + tmux switch-client
- [x] Kill session (`x`) — tmux kill-pane + remove from list
- [x] Pane width sync — resize-window to viewport width so capture output wraps correctly
- [x] Auto-refresh session list every 3 s (picks up new/dead sessions automatically)
- [x] Scroll % indicator anchored to top-right of output header
- [x] Distinct insert-mode status bar (pastel amber)
- [x] `--help` CLI output

## In progress / next

### Session grouping
Group the session sidebar by project (git root or project path).
Collapsible groups with a toggle key (e.g. `space`).
Show per-group aggregate state (e.g. amber dot if any session in the group is working).

### Toggle session visibility
Hide/show individual sessions or whole groups from the sidebar.

### Session naming
Assign a custom label to a session that persists across herd restarts.
Stored in `~/.herd/names.json`, keyed by Claude session ID falling back to pane ID.
Edit with a prompt overlay (e.g. `e` key).

### Filter / search (`/`)
Type to fuzzy-filter the session list.
Matches on project name, git branch, session label, pane ID.
`esc` clears the filter.

### New session (`n`)
Project picker modal: list recently used directories (from existing sessions +
a configurable search path), select one, launch `claude` in a new tmux window.

### Worktrees (`w`)
For the selected session's project, list git worktrees.
Option to open an existing worktree in a new Claude session, or create a new
worktree + session in one step.

### Aggregate stats in header
Show a compact summary in the top bar, e.g.:
`herd  ·  myproject  [main]  ·  3 working  2 waiting`

### Zoom mode (`z`)
Hide the session sidebar entirely so the viewport fills the terminal.
Toggle back with `z`.

## Backlog

- Broadcast mode — send the same prompt / keystroke to multiple selected sessions simultaneously
- `y` yank — copy viewport content to system clipboard
- Scroll marker — dim badge on session list entry when new output has arrived while scrolled up
- Persistent session memory — remember which sessions were being monitored so herd can re-attach after a restart
- Mouse scroll in session list
- Configurable key bindings
- `herd new <path>` CLI shorthand to launch a session without opening the TUI
