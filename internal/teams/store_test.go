package teams

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTeamConfig(t *testing.T, dir, teamName string, team Team) {
	t.Helper()
	teamDir := filepath.Join(dir, teamName)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(team)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(teamDir, "config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingDirectory(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nonexistent"))
	if err := s.Load(); err != nil {
		t.Fatalf("Load on missing dir should succeed, got: %v", err)
	}
	if got := s.TeamForSession("%1", ""); got != "" {
		t.Fatalf("expected empty team, got %q", got)
	}
}

func TestLoadEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load on empty dir failed: %v", err)
	}
	if got := s.TeamForSession("%1", ""); got != "" {
		t.Fatalf("expected empty team, got %q", got)
	}
}

func TestLoadValidTeam(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "alpha", Team{
		Name:          "alpha",
		LeadSessionID: "lead-1",
		Members: []Member{
			{AgentID: "a1", Name: "worker-1", TmuxPaneID: "%5"},
			{AgentID: "a2", Name: "worker-2", SessionID: "sess-w2"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got := s.TeamForSession("", "lead-1"); got != "alpha" {
		t.Fatalf("expected alpha for lead session, got %q", got)
	}
}

func TestLoadSkipsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	// Create a team dir with invalid JSON
	badDir := filepath.Join(dir, "bad-team")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "config.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also create a valid team
	writeTeamConfig(t, dir, "good-team", Team{
		Name:          "good-team",
		LeadSessionID: "lead-good",
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load should succeed despite malformed config: %v", err)
	}
	// The good team should still be loaded
	if got := s.TeamForSession("", "lead-good"); got != "good-team" {
		t.Fatalf("expected good-team, got %q", got)
	}
}

func TestLoadSkipsMissingConfigJSON(t *testing.T) {
	dir := t.TempDir()
	// Create a team dir with no config.json
	if err := os.MkdirAll(filepath.Join(dir, "empty-team"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load should succeed despite missing config.json: %v", err)
	}
}

func TestLoadSkipsFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file (not a directory) in the teams dir
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load should skip non-directory entries: %v", err)
	}
}

func TestLoadMultipleTeams(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "alpha", Team{
		Name:          "alpha",
		LeadSessionID: "lead-a",
		Members: []Member{
			{AgentID: "a1", Name: "w1", TmuxPaneID: "%10"},
		},
	})
	writeTeamConfig(t, dir, "beta", Team{
		Name:          "beta",
		LeadSessionID: "lead-b",
		Members: []Member{
			{AgentID: "b1", Name: "w2", SessionID: "sess-b1"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := s.TeamForSession("", "lead-a"); got != "alpha" {
		t.Fatalf("expected alpha, got %q", got)
	}
	if got := s.TeamForSession("", "lead-b"); got != "beta" {
		t.Fatalf("expected beta, got %q", got)
	}
}

func TestTeamForSessionByPaneID(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "myteam", Team{
		Name: "myteam",
		Members: []Member{
			{AgentID: "a1", Name: "worker", TmuxPaneID: "%42"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if got := s.TeamForSession("%42", ""); got != "myteam" {
		t.Fatalf("expected myteam via pane ID, got %q", got)
	}
}

func TestTeamForSessionByMemberSessionID(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "myteam", Team{
		Name: "myteam",
		Members: []Member{
			{AgentID: "a1", Name: "worker", SessionID: "member-sess"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if got := s.TeamForSession("", "member-sess"); got != "myteam" {
		t.Fatalf("expected myteam via member session ID, got %q", got)
	}
}

func TestTeamForSessionNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "myteam", Team{
		Name:          "myteam",
		LeadSessionID: "lead-1",
		Members: []Member{
			{AgentID: "a1", Name: "worker", TmuxPaneID: "%5", SessionID: "sess-5"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if got := s.TeamForSession("%99", "unknown-sess"); got != "" {
		t.Fatalf("expected empty for non-matching session, got %q", got)
	}
}

func TestTeamForSessionEmptyInputs(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "myteam", Team{
		Name:          "myteam",
		LeadSessionID: "lead-1",
		Members: []Member{
			{AgentID: "a1", Name: "worker", TmuxPaneID: "%5"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	// Both empty — should not match anything
	if got := s.TeamForSession("", ""); got != "" {
		t.Fatalf("expected empty for empty inputs, got %q", got)
	}
}

func TestTeamForSessionPaneIDPriority(t *testing.T) {
	dir := t.TempDir()
	// Member has both pane ID and session ID
	writeTeamConfig(t, dir, "myteam", Team{
		Name: "myteam",
		Members: []Member{
			{AgentID: "a1", Name: "worker", TmuxPaneID: "%7", SessionID: "sess-7"},
		},
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	// Match by pane ID even if session ID doesn't match
	if got := s.TeamForSession("%7", "wrong-sess"); got != "myteam" {
		t.Fatalf("expected myteam via pane ID, got %q", got)
	}
}

func TestReloadClearsOldTeams(t *testing.T) {
	dir := t.TempDir()
	writeTeamConfig(t, dir, "first", Team{
		Name:          "first",
		LeadSessionID: "lead-first",
	})

	s := NewStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if got := s.TeamForSession("", "lead-first"); got != "first" {
		t.Fatalf("expected first, got %q", got)
	}

	// Remove the first team, add a different one
	os.RemoveAll(filepath.Join(dir, "first"))
	writeTeamConfig(t, dir, "second", Team{
		Name:          "second",
		LeadSessionID: "lead-second",
	})

	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if got := s.TeamForSession("", "lead-first"); got != "" {
		t.Fatalf("expected old team gone after reload, got %q", got)
	}
	if got := s.TeamForSession("", "lead-second"); got != "second" {
		t.Fatalf("expected second, got %q", got)
	}
}
