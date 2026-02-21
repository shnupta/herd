package session

import (
	"testing"
	"time"
)

func TestKey(t *testing.T) {
	s := Session{ID: "abc-123", TmuxPane: "%1"}
	if got := s.Key(); got != "session:abc-123" {
		t.Errorf("Key() = %q, want session:abc-123", got)
	}

	s2 := Session{TmuxPane: "%5"}
	if got := s2.Key(); got != "pane:%5" {
		t.Errorf("Key() = %q, want pane:%%5", got)
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		path string
		pane string
		want string
	}{
		{"/home/user/dev/project", "%1", "dev/project"},
		{"/single", "%2", "single"},
		{"", "%3", "%3"},
	}
	for _, tt := range tests {
		s := Session{ProjectPath: tt.path, TmuxPane: tt.pane}
		if got := s.DisplayName(); got != tt.want {
			t.Errorf("DisplayName() with path=%q = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIdleForZero(t *testing.T) {
	s := Session{}
	if d := s.IdleFor(); d != 0 {
		t.Errorf("IdleFor() with zero UpdatedAt = %v, want 0", d)
	}
}

func TestIdleForNonZero(t *testing.T) {
	s := Session{UpdatedAt: time.Now().Add(-5 * time.Second)}
	d := s.IdleFor()
	if d < 5*time.Second || d > 10*time.Second {
		t.Errorf("IdleFor() = %v, expected ~5s", d)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateUnknown, "unknown"},
		{StateIdle, "idle"},
		{StateWorking, "working"},
		{StateWaiting, "waiting"},
		{StatePlanReady, "plan_ready"},
		{StateNotifying, "notifying"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		input string
		want  State
	}{
		{"working", StateWorking},
		{"waiting", StateWaiting},
		{"idle", StateIdle},
		{"plan_ready", StatePlanReady},
		{"notifying", StateNotifying},
		{"unknown", StateUnknown},
		{"", StateUnknown},
		{"garbage", StateUnknown},
		{"Working", StateUnknown}, // case-sensitive
		{"IDLE", StateUnknown},
	}
	for _, tt := range tests {
		if got := ParseState(tt.input); got != tt.want {
			t.Errorf("ParseState(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseStateRoundTrip(t *testing.T) {
	// Every State's String() should round-trip back through ParseState,
	// except StateUnknown whose String() is "unknown" (not a valid hook state).
	states := []State{StateIdle, StateWorking, StateWaiting, StatePlanReady, StateNotifying}
	for _, s := range states {
		got := ParseState(s.String())
		if got != s {
			t.Errorf("ParseState(%q) = %v, want %v", s.String(), got, s)
		}
	}
}

func TestDisplayNameRootPath(t *testing.T) {
	s := Session{ProjectPath: "/", TmuxPane: "%1"}
	got := s.DisplayName()
	// filepath.Base("/") = "/", parent is also "/" so we get just "/"
	if got == "" {
		t.Error("DisplayName should not return empty for root path")
	}
}

func TestKeyEmptyBoth(t *testing.T) {
	s := Session{}
	got := s.Key()
	if got != "pane:" {
		t.Errorf("Key() with empty fields = %q, want %q", got, "pane:")
	}
}
