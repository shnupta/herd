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
		s.GitBranch = gitBranch(p.CurrentPath)
		sessions = append(sessions, s)
	}
	return sessions, nil
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
