package session

import (
	"testing"
	"time"

	"github.com/shnupta/herd/internal/state"
	"github.com/shnupta/herd/internal/tmux"
)

// noBranch is a branch lookup stub that always returns "".
func noBranch(string) string { return "" }

// branchStub returns the provided branch string for any directory.
func branchStub(branch string) func(string) string {
	return func(string) string { return branch }
}

// ---------------------------------------------------------------------------
// Helpers for constructing fake state/pane data
// ---------------------------------------------------------------------------

func makePane(id, sessionName, cmd, path string, windowIdx, paneIdx int) tmux.Pane {
	return tmux.Pane{
		ID:          id,
		SessionName: sessionName,
		WindowIndex: windowIdx,
		PaneIndex:   paneIdx,
		CurrentCmd:  cmd,
		CurrentPath: path,
	}
}

func makeStateEntry(sessionID, paneID, projectPath, stateStr string) state.SessionState {
	return state.SessionState{
		SessionID:   sessionID,
		TmuxPane:    paneID,
		ProjectPath: projectPath,
		State:       stateStr,
		UpdatedAt:   time.Now(),
	}
}

// ---------------------------------------------------------------------------
// discover() — primary path: all sessions from state files
// ---------------------------------------------------------------------------

func TestDiscoverAllFromStateFiles(t *testing.T) {
	stateEntries := []state.SessionState{
		makeStateEntry("sess-abc", "%1", "/home/user/project", "waiting"),
		makeStateEntry("sess-def", "%2", "/home/user/other", "working"),
	}
	panes := []tmux.Pane{
		makePane("%1", "mysession", "2.1.47", "/home/user/project", 0, 0),
		makePane("%2", "mysession", "claude", "/home/user/other", 1, 0),
	}

	readAll := func() ([]state.SessionState, error) { return stateEntries, nil }
	listPanes := func() ([]tmux.Pane, error) { return panes, nil }

	sessions, err := discover(readAll, listPanes, noBranch)
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("discover = %d sessions, want 2", len(sessions))
	}

	// Verify first session
	s0 := sessions[0]
	if s0.ID != "sess-abc" {
		t.Errorf("sessions[0].ID = %q, want sess-abc", s0.ID)
	}
	if s0.TmuxPane != "%1" {
		t.Errorf("sessions[0].TmuxPane = %q, want %%1", s0.TmuxPane)
	}
	if s0.State != StateWaiting {
		t.Errorf("sessions[0].State = %v, want StateWaiting", s0.State)
	}
	if s0.ProjectPath != "/home/user/project" {
		t.Errorf("sessions[0].ProjectPath = %q, want /home/user/project", s0.ProjectPath)
	}

	// Verify second session
	s1 := sessions[1]
	if s1.ID != "sess-def" {
		t.Errorf("sessions[1].ID = %q, want sess-def", s1.ID)
	}
	if s1.State != StateWorking {
		t.Errorf("sessions[1].State = %v, want StateWorking", s1.State)
	}
}

// ---------------------------------------------------------------------------
// discover() — stale state file dropped when pane is gone
// ---------------------------------------------------------------------------

func TestDiscoverStaleStateFileDropped(t *testing.T) {
	stateEntries := []state.SessionState{
		makeStateEntry("sess-alive", "%10", "/work/alive", "idle"),
		makeStateEntry("sess-dead", "%99", "/work/dead", "waiting"), // pane %99 doesn't exist
	}
	panes := []tmux.Pane{
		makePane("%10", "s1", "claude", "/work/alive", 0, 0),
		// pane %99 is intentionally absent
	}

	readAll := func() ([]state.SessionState, error) { return stateEntries, nil }
	listPanes := func() ([]tmux.Pane, error) { return panes, nil }

	sessions, err := discover(readAll, listPanes, noBranch)
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("discover = %d sessions, want 1 (stale should be dropped)", len(sessions))
	}
	if sessions[0].ID != "sess-alive" {
		t.Errorf("sessions[0].ID = %q, want sess-alive", sessions[0].ID)
	}
}

// ---------------------------------------------------------------------------
// discover() — fallback pane appended for sessions without state files
// ---------------------------------------------------------------------------

func TestDiscoverFallbackPaneAppended(t *testing.T) {
	// No state file entries — only the process-name heuristic applies.
	readAll := func() ([]state.SessionState, error) { return nil, nil }
	panes := []tmux.Pane{
		makePane("%5", "sess", "2.1.47", "/home/user/project", 1, 0),
		makePane("%6", "sess", "bash", "/home/user", 1, 1), // not Claude
	}
	listPanes := func() ([]tmux.Pane, error) { return panes, nil }

	sessions, err := discover(readAll, listPanes, branchStub("main"))
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("discover = %d sessions, want 1 (only the claude pane)", len(sessions))
	}
	s := sessions[0]
	if s.ID != "" {
		t.Errorf("ID = %q, want empty (no state file)", s.ID)
	}
	if s.TmuxPane != "%5" {
		t.Errorf("TmuxPane = %q, want %%5", s.TmuxPane)
	}
	if s.State != StateUnknown {
		t.Errorf("State = %v, want StateUnknown", s.State)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want main", s.GitBranch)
	}
}

// ---------------------------------------------------------------------------
// discover() — mixed scenario: some state-file sessions + fallback panes
// ---------------------------------------------------------------------------

func TestDiscoverMixedScenario(t *testing.T) {
	// %1 — covered by a state file (hooks installed)
	// %2 — no state file, but looks like Claude (pre-hook fallback)
	// %3 — not Claude at all (bash), should be excluded
	stateEntries := []state.SessionState{
		makeStateEntry("sess-hooked", "%1", "/work/hooked", "working"),
	}
	panes := []tmux.Pane{
		makePane("%1", "s1", "2.1.47", "/work/hooked", 0, 0),
		makePane("%2", "s1", "claude", "/work/legacy", 0, 1),
		makePane("%3", "s1", "bash", "/work/other", 0, 2),
	}

	readAll := func() ([]state.SessionState, error) { return stateEntries, nil }
	listPanes := func() ([]tmux.Pane, error) { return panes, nil }

	sessions, err := discover(readAll, listPanes, noBranch)
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("discover = %d sessions, want 2", len(sessions))
	}

	// First: from state file
	if sessions[0].ID != "sess-hooked" {
		t.Errorf("sessions[0].ID = %q, want sess-hooked", sessions[0].ID)
	}
	if sessions[0].State != StateWorking {
		t.Errorf("sessions[0].State = %v, want StateWorking", sessions[0].State)
	}

	// Second: fallback
	if sessions[1].ID != "" {
		t.Errorf("sessions[1].ID = %q, want empty (fallback)", sessions[1].ID)
	}
	if sessions[1].TmuxPane != "%2" {
		t.Errorf("sessions[1].TmuxPane = %q, want %%2", sessions[1].TmuxPane)
	}
	if sessions[1].State != StateUnknown {
		t.Errorf("sessions[1].State = %v, want StateUnknown", sessions[1].State)
	}
}

// ---------------------------------------------------------------------------
// discover() — empty inputs
// ---------------------------------------------------------------------------

func TestDiscoverEmpty(t *testing.T) {
	readAll := func() ([]state.SessionState, error) { return nil, nil }
	listPanes := func() ([]tmux.Pane, error) { return nil, nil }

	sessions, err := discover(readAll, listPanes, noBranch)
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("discover = %d sessions, want 0", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// discover() — state entry metadata propagated correctly
// ---------------------------------------------------------------------------

func TestDiscoverStateMetadataPropagated(t *testing.T) {
	now := time.Now().Add(-1 * time.Minute)
	stateEntries := []state.SessionState{
		{
			SessionID:   "sess-xyz",
			TmuxPane:    "%7",
			ProjectPath: "/my/project",
			State:       "plan_ready",
			CurrentTool: "Bash",
			UpdatedAt:   now,
		},
	}
	panes := []tmux.Pane{
		makePane("%7", "main-session", "2.1.47", "/my/project", 2, 3),
	}

	readAll := func() ([]state.SessionState, error) { return stateEntries, nil }
	listPanes := func() ([]tmux.Pane, error) { return panes, nil }

	sessions, err := discover(readAll, listPanes, branchStub("feature"))
	if err != nil {
		t.Fatalf("discover returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("discover = %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.ID != "sess-xyz" {
		t.Errorf("ID = %q, want sess-xyz", s.ID)
	}
	if s.TmuxSession != "main-session" {
		t.Errorf("TmuxSession = %q, want main-session", s.TmuxSession)
	}
	if s.WindowIndex != 2 {
		t.Errorf("WindowIndex = %d, want 2", s.WindowIndex)
	}
	if s.PaneIndex != 3 {
		t.Errorf("PaneIndex = %d, want 3", s.PaneIndex)
	}
	if s.State != StatePlanReady {
		t.Errorf("State = %v, want StatePlanReady", s.State)
	}
	if s.CurrentTool != "Bash" {
		t.Errorf("CurrentTool = %q, want Bash", s.CurrentTool)
	}
	if !s.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", s.UpdatedAt, now)
	}
	if s.GitBranch != "feature" {
		t.Errorf("GitBranch = %q, want feature", s.GitBranch)
	}
}

// ---------------------------------------------------------------------------
// stateFromString
// ---------------------------------------------------------------------------

func TestStateFromString(t *testing.T) {
	tests := []struct {
		input string
		want  State
	}{
		{"working", StateWorking},
		{"waiting", StateWaiting},
		{"idle", StateIdle},
		{"plan_ready", StatePlanReady},
		{"notifying", StateNotifying},
		{"", StateUnknown},
		{"unknown", StateUnknown},
		{"bogus", StateUnknown},
	}
	for _, tt := range tests {
		if got := stateFromString(tt.input); got != tt.want {
			t.Errorf("stateFromString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
