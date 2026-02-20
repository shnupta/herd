package session

import (
	"testing"

	"github.com/shnupta/herd/internal/tmux"
)

func noBranch(string) string { return "" }

func TestBuildSessionsEmpty(t *testing.T) {
	sessions := buildSessions(nil, noBranch)
	if sessions != nil {
		t.Errorf("buildSessions(nil) = %v, want nil", sessions)
	}
}

func TestBuildSessionsFiltersNonClaude(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "bash"},
		{ID: "%2", CurrentCmd: "vim"},
		{ID: "%3", CurrentCmd: "zsh"},
	}
	sessions := buildSessions(panes, noBranch)
	if len(sessions) != 0 {
		t.Errorf("buildSessions with non-claude panes = %d sessions, want 0", len(sessions))
	}
}

func TestBuildSessionsWithVersionString(t *testing.T) {
	panes := []tmux.Pane{
		{
			ID:          "%5",
			SessionName: "mysession",
			WindowIndex: 1,
			PaneIndex:   0,
			CurrentPath: "/home/user/project",
			CurrentCmd:  "2.1.47",
		},
	}
	sessions := buildSessions(panes, func(dir string) string {
		return "main"
	})

	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.TmuxPane != "%5" {
		t.Errorf("TmuxPane = %q, want %%5", s.TmuxPane)
	}
	if s.TmuxSession != "mysession" {
		t.Errorf("TmuxSession = %q, want mysession", s.TmuxSession)
	}
	if s.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath = %q, want /home/user/project", s.ProjectPath)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want main", s.GitBranch)
	}
	if s.State != StateUnknown {
		t.Errorf("State = %v, want StateUnknown", s.State)
	}
	if s.WindowIndex != 1 {
		t.Errorf("WindowIndex = %d, want 1", s.WindowIndex)
	}
	if s.PaneIndex != 0 {
		t.Errorf("PaneIndex = %d, want 0", s.PaneIndex)
	}
}

func TestBuildSessionsWithClaudeCommand(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%10", CurrentCmd: "claude", CurrentPath: "/work"},
		{ID: "%11", CurrentCmd: "bash", CurrentPath: "/work"},
	}
	sessions := buildSessions(panes, noBranch)
	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	if sessions[0].TmuxPane != "%10" {
		t.Errorf("TmuxPane = %q, want %%10", sessions[0].TmuxPane)
	}
}

func TestBuildSessionsMixedPanes(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "3.0.0"},
		{ID: "%2", CurrentCmd: "bash"},
		{ID: "%3", CurrentCmd: "claude"},
	}
	sessions := buildSessions(panes, noBranch)
	if len(sessions) != 2 {
		t.Errorf("buildSessions = %d sessions, want 2 (version + claude)", len(sessions))
	}
}
