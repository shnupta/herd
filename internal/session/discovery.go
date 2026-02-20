package session

import (
	"os/exec"
	"strings"
	"time"

	"github.com/shnupta/herd/internal/state"
	"github.com/shnupta/herd/internal/tmux"
)

// Discover returns all live Claude Code sessions using a three-step approach:
//
//  1. Read hook state files (~/.herd/sessions/*.json) — sessions with full
//     metadata including real Claude session IDs.
//  2. Cross-reference against live tmux panes; drop any state-file entry whose
//     pane no longer exists (stale).
//  3. Append panes that pass the IsClaudePane heuristic but aren't already
//     covered by step 1 (fallback for sessions started before hooks were
//     installed).
func Discover() ([]Session, error) {
	return discover(state.ReadAll, tmux.ListPanes, gitBranch)
}

// discover is the testable core of Discover. It accepts injectable functions
// so tests can supply fake state, fake panes, and a no-op branch lookup.
func discover(
	readAll func() ([]state.SessionState, error),
	listPanes func() ([]tmux.Pane, error),
	branchFn func(string) string,
) ([]Session, error) {
	// Step 1: load hook-tracked sessions.
	stateEntries, err := readAll()
	if err != nil {
		return nil, err
	}

	// Step 2: load live tmux panes and build an O(1) lookup set.
	panes, err := listPanes()
	if err != nil {
		return nil, err
	}

	liveByID := make(map[string]tmux.Pane, len(panes))
	for _, p := range panes {
		liveByID[p.ID] = p
	}

	// Convert surviving state entries to Sessions.
	var sessions []Session
	coveredPanes := make(map[string]struct{}, len(stateEntries))

	for _, ss := range stateEntries {
		pane, alive := liveByID[ss.TmuxPane]
		if !alive {
			// Pane is gone — state file is stale; skip.
			continue
		}

		s := Session{
			ID:          ss.SessionID,
			TmuxPane:    ss.TmuxPane,
			TmuxSession: pane.SessionName,
			WindowIndex: pane.WindowIndex,
			PaneIndex:   pane.PaneIndex,
			ProjectPath: ss.ProjectPath,
			State:       stateFromString(ss.State),
			CurrentTool: ss.CurrentTool,
			UpdatedAt:   ss.UpdatedAt,
		}
		s.GitBranch = branchFn(ss.ProjectPath)

		sessions = append(sessions, s)
		coveredPanes[ss.TmuxPane] = struct{}{}
	}

	// Step 3: fallback — panes matching the process-name heuristic that aren't
	// already covered by a state file entry.
	for _, p := range panes {
		if _, covered := coveredPanes[p.ID]; covered {
			continue
		}
		if !tmux.IsClaudePane(p.CurrentCmd) {
			continue
		}
		s := Session{
			TmuxPane:    p.ID,
			TmuxSession: p.SessionName,
			WindowIndex: p.WindowIndex,
			PaneIndex:   p.PaneIndex,
			ProjectPath: p.CurrentPath,
			State:       StateUnknown,
			UpdatedAt:   time.Now(),
		}
		s.GitBranch = branchFn(p.CurrentPath)
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// stateFromString converts a state string (as stored in the JSON state file)
// to the typed State constant used by Session.
func stateFromString(s string) State {
	switch s {
	case "working":
		return StateWorking
	case "waiting":
		return StateWaiting
	case "idle":
		return StateIdle
	case "plan_ready":
		return StatePlanReady
	case "notifying":
		return StateNotifying
	default:
		return StateUnknown
	}
}

// gitBranch returns the current git branch for the given directory, or empty string.
func gitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "" // detached HEAD
	}
	return branch
}
