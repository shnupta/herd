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

	// Per-call caches so panes in the same directory don't spawn redundant git processes.
	branchCache := make(map[string]string)
	rootCache := make(map[string]string)

	cachedBranch := func(dir string) string {
		if v, ok := branchCache[dir]; ok {
			return v
		}
		v := gitBranch(dir)
		branchCache[dir] = v
		return v
	}
	cachedRoot := func(dir string) string {
		if v, ok := rootCache[dir]; ok {
			return v
		}
		v := gitRoot(dir)
		rootCache[dir] = v
		return v
	}

	return buildSessions(panes, cachedBranch, cachedRoot), nil
}

// buildSessions converts tmux panes to Sessions using the provided lookup functions.
func buildSessions(panes []tmux.Pane, branchFn func(string) string, rootFn func(string) string) []Session {
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
		s.GitRoot = rootFn(p.CurrentPath)
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

// gitRoot returns the absolute path to the git repository root for the given
// directory, or empty string if the directory is not inside a git repository.
func gitRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
