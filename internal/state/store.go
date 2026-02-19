package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionState is written by the hook binary and read by the TUI.
type SessionState struct {
	SessionID   string    `json:"session_id"`
	TmuxPane    string    `json:"tmux_pane"`
	State       string    `json:"state"` // "working", "waiting", "idle", "plan_ready", "notifying"
	CurrentTool string    `json:"current_tool,omitempty"`
	ProjectPath string    `json:"project_path,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Dir returns the directory where state files are stored.
func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".herd", "sessions")
}

// Path returns the state file path for a given session ID.
func Path(sessionID string) string {
	return filepath.Join(Dir(), sessionID+".json")
}

// Write atomically writes the state for a session.
func Write(s SessionState) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Write to temp file then rename for atomicity.
	tmp := Path(s.SessionID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, Path(s.SessionID)); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ReadAll loads all session state files from the state directory.
func ReadAll() ([]SessionState, error) {
	entries, err := os.ReadDir(Dir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var states []SessionState
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(Dir(), e.Name()))
		if err != nil {
			continue
		}
		var s SessionState
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		states = append(states, s)
	}
	return states, nil
}
