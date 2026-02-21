package session

import (
	"path/filepath"
	"time"
)

// State represents the current activity state of a Claude session.
type State int

const (
	StateUnknown  State = iota // no hook data yet
	StateIdle                  // no recent activity
	StateWorking               // tool is executing
	StateWaiting               // Claude finished, waiting for user input
	StatePlanReady             // ExitPlanMode was called, plan awaits approval
	StateNotifying             // Claude sent a notification
)

func (s State) String() string {
	switch s {
	case StateWorking:
		return "working"
	case StateWaiting:
		return "waiting"
	case StateIdle:
		return "idle"
	case StatePlanReady:
		return "plan_ready"
	case StateNotifying:
		return "notifying"
	default:
		return "unknown"
	}
}

// Session represents a running Claude Code instance.
type Session struct {
	// Identity
	ID          string // Claude session UUID (from hooks), empty until first hook fires
	TmuxPane    string // tmux pane ID, e.g. "%12"
	TmuxSession string // tmux session name, e.g. "2"
	WindowIndex int
	PaneIndex   int

	// Context
	ProjectPath string
	GitRoot     string // absolute path to git repo root; empty if not a git repo
	GitBranch   string

	// State
	State       State
	CurrentTool string // set when State == StateWorking
	UpdatedAt   time.Time
}

// Key returns a unique identifier for the session, suitable for pinning/ordering.
// Uses Claude session ID if available (from hooks), otherwise falls back to pane ID.
func (s Session) Key() string {
	if s.ID != "" {
		return "session:" + s.ID
	}
	return "pane:" + s.TmuxPane
}

// DisplayName returns a short human-readable label for the session.
func (s Session) DisplayName() string {
	if s.ProjectPath == "" {
		return s.TmuxPane
	}
	// Use the last two path components for context, e.g. "dev/porter"
	base := filepath.Base(s.ProjectPath)
	parent := filepath.Base(filepath.Dir(s.ProjectPath))
	if parent != "" && parent != "." && parent != "/" {
		return parent + "/" + base
	}
	return base
}

// IdleFor returns how long the session has been in its current state.
func (s Session) IdleFor() time.Duration {
	if s.UpdatedAt.IsZero() {
		return 0
	}
	return time.Since(s.UpdatedAt)
}

// ParseState parses a hook-written state string into a State value.
func ParseState(s string) State {
	switch s {
	case "working":
		return StateWorking
	case "waiting":
		return StateWaiting
	case "plan_ready":
		return StatePlanReady
	case "notifying":
		return StateNotifying
	case "idle":
		return StateIdle
	default:
		return StateUnknown
	}
}

