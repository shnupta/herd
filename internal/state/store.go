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

// Store manages session state files in a directory.
type Store struct {
	dir string
}

// NewStore creates a new Store for the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Dir returns the directory where state files are stored.
func (s *Store) Dir() string {
	return s.dir
}

// Path returns the state file path for a given session ID.
func (s *Store) Path(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".json")
}

// Write atomically writes the state for a session.
func (s *Store) Write(ss SessionState) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data, err := json.Marshal(ss)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Write to temp file then rename for atomicity.
	tmp := s.Path(ss.SessionID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.Path(ss.SessionID)); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ReadAll loads all session state files from the state directory.
func (s *Store) ReadAll() ([]SessionState, error) {
	entries, err := os.ReadDir(s.dir)
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
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var ss SessionState
		if err := json.Unmarshal(data, &ss); err != nil {
			continue
		}
		states = append(states, ss)
	}
	return states, nil
}

var defaultStore *Store

func init() {
	home, _ := os.UserHomeDir()
	defaultStore = NewStore(filepath.Join(home, ".herd", "sessions"))
}

// Dir returns the directory where state files are stored.
func Dir() string { return defaultStore.Dir() }

// Path returns the state file path for a given session ID.
func Path(sessionID string) string { return defaultStore.Path(sessionID) }

// Write atomically writes the state for a session.
func Write(ss SessionState) error { return defaultStore.Write(ss) }

// ReadAll loads all session state files from the state directory.
func ReadAll() ([]SessionState, error) { return defaultStore.ReadAll() }
