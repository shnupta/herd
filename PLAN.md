# herd — development plan

## Done

- [x] TUI with session list pane and live `capture-pane` viewport
- [x] Session discovery via `tmux list-panes`, detecting Claude processes by process name
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
- [x] Diff review mode (`d`) — review git diff with inline comments, submit feedback to agent
- [x] Filter/search (`/`) — fuzzy filter session list
- [x] Aggregate stats in header — shows count of sessions by state
- [x] New session (`n`) — project picker to launch claude in a new tmux window
- [x] Session grouping — opt-in custom groups (`g` key) with collapsible headers (`space`) and aggregate state dot
- [x] Session naming (`e`) — assign custom label, persistent via `~/.herd/names.json`
- [x] Pinning (`p`) — pin sessions to top; pin order persisted via `~/.herd/sidebar.json`
- [x] Reordering (`J`/`K`) — move sessions up/down in list, order persisted
- [x] Mouse support — wheel scroll and click-to-select in sidebar
- [x] Agent team auto-grouping — sessions belonging to a Claude Code agent team are
      automatically grouped under the team name by reading `~/.claude/teams/*/config.json`;
      custom group assignments still take precedence

## In progress / next

### Toggle session visibility
Hide/show individual sessions or whole groups from the sidebar.
Affects display only; session state still syncs in background.

### Worktrees (`w`)
For the selected session's project, list git worktrees.
Option to open an existing worktree in a new Claude session, or create a new
worktree + session in one step.
(Key binding exists; core functionality not yet implemented.)

### Zoom mode (`z`)
Hide the session sidebar entirely so the viewport fills the terminal.
Toggle back with `z`.

### Agent team pane resize
When viewing an agent team session, resize the tmux pane to match herd's viewport
dimensions so Claude's TUI renders at the correct width and `capture-pane` output
fits without truncation.

Proposed approach: `tmux resize-pane -t <pane-id> -x <viewport-width> -y <viewport-height>`
fired when the selected session changes and on `WindowSizeMsg`.

Open question: it's unclear how Claude Code lays out teammate panes (same window as the
lead, separate window, separate session). Resizing a pane adjusts neighbouring panes in
the same window, which could disrupt the user's tmux layout. Need to verify the window
topology before implementing to avoid unintended side-effects. Consider making this
opt-in behind a setting initially.

### Improve session discovery
Current approach scans `pane_current_command` for `"claude"` or a semver string (e.g.
`2.1.47`). This is fragile:
- Renaming a tmux window/pane does not affect `pane_current_command`, but if Claude
  ships under a different binary name it silently breaks.
- Any unrelated process named like a version string is a false positive.

Better approach: use the hook state files as the **primary** discovery source.
When hooks are installed, every Claude session writes `~/.herd/state/<session_id>.json`
containing its pane ID, project path, and session ID — ground truth with no process-name
guessing. The `tmux list-panes` scan stays as a **fallback** for sessions that started
before hooks were installed or where hooks are absent.

Discovery order:
1. Read all state files from `~/.herd/state/` → produces sessions with full metadata.
2. Cross-reference against live tmux panes (fast `list-panes`) to drop stale entries
   whose pane no longer exists.
3. Append any panes matched by process-name heuristic that aren't already in step 1
   (hooks-not-installed fallback).

### tmux integration — jump back to herd
When you press `t` to jump to a Claude pane you lose the ability to get back to herd
without knowing which window it lives in. Two parts:

1. **Herd registers itself on startup.** Write the herd pane ID to a well-known location
   (e.g. `~/.herd/herd_pane`) so external scripts and tmux bindings can find it.

2. **A tmux key binding to return.** `herd install` (or a separate `herd tmux-setup`)
   writes a binding into `~/.tmux.conf` (or sources a snippet) such as:
   ```
   bind-key H run-shell "tmux switch-client -t $(cat ~/.herd/herd_pane)"
   ```
   so `prefix+H` always snaps back to herd from anywhere in tmux.

3. **Herd cleans up on exit.** Remove `~/.herd/herd_pane` on shutdown so stale pane
   IDs don't cause errors after herd exits.

## Backlog

- Broadcast mode — send the same prompt / keystroke to multiple selected sessions simultaneously
- `y` yank — copy viewport content to system clipboard
- Scroll marker — dim badge on session list entry when new output has arrived while scrolled up
- Persistent session memory — remember which sessions were being monitored so herd can re-attach after a restart
- Configurable key bindings
- `herd new <path>` CLI shorthand to launch a session without opening the TUI
