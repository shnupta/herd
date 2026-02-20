package session

import (
	"os/exec"
	"strings"
	"time"

	"github.com/shnupta/herd/internal/tmux"
)

// Discover scans all tmux panes and returns sessions for any that are running Claude.
func Discover() ([]Session, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return nil, err
	}
	return buildSessions(panes, gitBranch), nil
}

// buildSessions converts tmux panes to Sessions using the provided branch lookup function.
func buildSessions(panes []tmux.Pane, branchFn func(string) string) []Session {
	var sessions []Session
	for _, p := range panes {
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
	return sessions
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
