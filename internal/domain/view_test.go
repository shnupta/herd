package domain

import (
	"testing"

	"github.com/shnupta/herd/internal/session"
)

func TestBuildViewItems_Ungrouped(t *testing.T) {
	sessions := []PreGroupedSession{
		{Session: session.Session{TmuxPane: "%1"}},
		{Session: session.Session{TmuxPane: "%2"}},
	}

	items := BuildViewItems(sessions, nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.IsHeader {
			t.Error("ungrouped sessions should not have headers")
		}
	}
	if items[0].SessionIdx != 0 || items[1].SessionIdx != 1 {
		t.Error("session indices wrong")
	}
}

func TestBuildViewItems_Grouped(t *testing.T) {
	sessions := []PreGroupedSession{
		{Session: session.Session{TmuxPane: "%1", State: session.StateIdle}, GroupKey: "g:web", GroupName: "web"},
		{Session: session.Session{TmuxPane: "%2", State: session.StateWorking}, GroupKey: "g:web", GroupName: "web"},
	}

	items := BuildViewItems(sessions, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 items (1 header + 2 sessions), got %d", len(items))
	}
	if !items[0].IsHeader {
		t.Error("first item should be a header")
	}
	if items[0].GroupKey != "g:web" {
		t.Errorf("expected group key g:web, got %s", items[0].GroupKey)
	}
	if items[0].GroupName != "web" {
		t.Errorf("expected group name web, got %s", items[0].GroupName)
	}
	if items[0].Count != 2 {
		t.Errorf("expected count 2, got %d", items[0].Count)
	}
	if items[0].AggState != session.StateWorking {
		t.Errorf("expected aggregate state Working, got %v", items[0].AggState)
	}
	if items[0].SessionIdx != -1 {
		t.Errorf("header SessionIdx should be -1, got %d", items[0].SessionIdx)
	}
}

func TestBuildViewItems_Collapsed(t *testing.T) {
	sessions := []PreGroupedSession{
		{Session: session.Session{TmuxPane: "%1"}, GroupKey: "g:api", GroupName: "api"},
		{Session: session.Session{TmuxPane: "%2"}, GroupKey: "g:api", GroupName: "api"},
	}
	collapsed := map[string]bool{"g:api": true}

	items := BuildViewItems(sessions, collapsed)

	if len(items) != 1 {
		t.Fatalf("expected 1 item (header only), got %d", len(items))
	}
	if !items[0].IsHeader {
		t.Error("expected header")
	}
}

func TestBuildViewItems_MixedGroupedAndUngrouped(t *testing.T) {
	sessions := []PreGroupedSession{
		{Session: session.Session{TmuxPane: "%1"}},
		{Session: session.Session{TmuxPane: "%2"}, GroupKey: "g:web", GroupName: "web"},
		{Session: session.Session{TmuxPane: "%3"}, GroupKey: "g:web", GroupName: "web"},
		{Session: session.Session{TmuxPane: "%4"}},
	}

	items := BuildViewItems(sessions, nil)

	// Expected: flat(%1), header(web), session(%2), session(%3), flat(%4)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	if items[0].IsHeader {
		t.Error("first item should be ungrouped session")
	}
	if !items[1].IsHeader {
		t.Error("second item should be group header")
	}
	if items[4].IsHeader {
		t.Error("last item should be ungrouped session")
	}
}

func TestBuildViewItems_Empty(t *testing.T) {
	items := BuildViewItems(nil, nil)
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestWorstState(t *testing.T) {
	tests := []struct {
		name   string
		states []session.State
		want   session.State
	}{
		{"empty", nil, session.StateUnknown},
		{"single idle", []session.State{session.StateIdle}, session.StateIdle},
		{"working wins", []session.State{session.StateIdle, session.StateWorking, session.StateWaiting}, session.StateWorking},
		{"waiting beats idle", []session.State{session.StateIdle, session.StateWaiting}, session.StateWaiting},
		{"plan_ready beats notifying", []session.State{session.StateNotifying, session.StatePlanReady}, session.StatePlanReady},
		{"notifying beats idle", []session.State{session.StateIdle, session.StateNotifying}, session.StateNotifying},
		{"unknown is lowest", []session.State{session.StateUnknown, session.StateIdle}, session.StateIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorstState(tt.states)
			if got != tt.want {
				t.Errorf("WorstState() = %v, want %v", got, tt.want)
			}
		})
	}
}
