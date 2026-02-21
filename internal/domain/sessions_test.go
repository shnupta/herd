package domain

import (
	"testing"
	"time"

	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/state"
)

func TestMergeSessions_MatchByID(t *testing.T) {
	sessions := []session.Session{
		{ID: "abc", TmuxPane: "%1", State: session.StateUnknown},
		{ID: "def", TmuxPane: "%2", State: session.StateUnknown},
	}
	now := time.Now()
	updates := []state.SessionState{
		{SessionID: "abc", TmuxPane: "%99", State: "working", CurrentTool: "Bash", UpdatedAt: now},
	}

	result := MergeSessions(sessions, updates)

	if result[0].State != session.StateWorking {
		t.Errorf("expected StateWorking, got %v", result[0].State)
	}
	if result[0].CurrentTool != "Bash" {
		t.Errorf("expected CurrentTool=Bash, got %q", result[0].CurrentTool)
	}
	if result[0].UpdatedAt != now {
		t.Errorf("expected UpdatedAt to be set")
	}
	// Second session should be unchanged
	if result[1].State != session.StateUnknown {
		t.Errorf("expected second session unchanged, got %v", result[1].State)
	}
}

func TestMergeSessions_MatchByPane(t *testing.T) {
	sessions := []session.Session{
		{ID: "", TmuxPane: "%5", State: session.StateUnknown},
	}
	updates := []state.SessionState{
		{SessionID: "new-id", TmuxPane: "%5", State: "idle"},
	}

	result := MergeSessions(sessions, updates)

	if result[0].ID != "new-id" {
		t.Errorf("expected ID to be set to new-id, got %q", result[0].ID)
	}
	if result[0].State != session.StateIdle {
		t.Errorf("expected StateIdle, got %v", result[0].State)
	}
}

func TestMergeSessions_NoMatch(t *testing.T) {
	sessions := []session.Session{
		{ID: "abc", TmuxPane: "%1", State: session.StateIdle},
	}
	updates := []state.SessionState{
		{SessionID: "zzz", TmuxPane: "%99", State: "working"},
	}

	result := MergeSessions(sessions, updates)

	if result[0].State != session.StateIdle {
		t.Errorf("expected session unchanged, got %v", result[0].State)
	}
}

func TestMergeSessions_DoesNotMutateInput(t *testing.T) {
	sessions := []session.Session{
		{ID: "", TmuxPane: "%1", State: session.StateUnknown},
	}
	updates := []state.SessionState{
		{SessionID: "new", TmuxPane: "%1", State: "working"},
	}

	_ = MergeSessions(sessions, updates)

	if sessions[0].State != session.StateUnknown {
		t.Errorf("original slice was mutated")
	}
}

func TestSortSessions_PinnedFirst(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1"},
		{TmuxPane: "%2"},
		{TmuxPane: "%3"},
	}
	pinned := map[string]int{
		"pane:%3": 1,
	}

	result := SortSessions(sessions, pinned, nil)

	if result[0].TmuxPane != "%3" {
		t.Errorf("expected pinned session first, got %s", result[0].TmuxPane)
	}
}

func TestSortSessions_PinCounterOrder(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1"},
		{TmuxPane: "%2"},
		{TmuxPane: "%3"},
	}
	pinned := map[string]int{
		"pane:%2": 5,
		"pane:%3": 2,
	}

	result := SortSessions(sessions, pinned, nil)

	if result[0].TmuxPane != "%3" {
		t.Errorf("expected pane %%3 first (lower pin counter), got %s", result[0].TmuxPane)
	}
	if result[1].TmuxPane != "%2" {
		t.Errorf("expected pane %%2 second, got %s", result[1].TmuxPane)
	}
}

func TestSortSessions_SavedOrder(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1"},
		{TmuxPane: "%2"},
		{TmuxPane: "%3"},
	}
	savedOrder := []string{"pane:%3", "pane:%1", "pane:%2"}

	result := SortSessions(sessions, nil, savedOrder)

	if result[0].TmuxPane != "%3" {
		t.Errorf("expected pane %%3 first per saved order, got %s", result[0].TmuxPane)
	}
	if result[1].TmuxPane != "%1" {
		t.Errorf("expected pane %%1 second, got %s", result[1].TmuxPane)
	}
	if result[2].TmuxPane != "%2" {
		t.Errorf("expected pane %%2 third, got %s", result[2].TmuxPane)
	}
}

func TestSortSessions_SingleElement(t *testing.T) {
	sessions := []session.Session{{TmuxPane: "%1"}}
	result := SortSessions(sessions, nil, nil)
	if len(result) != 1 || result[0].TmuxPane != "%1" {
		t.Errorf("single element should be returned as-is")
	}
}
