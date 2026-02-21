package teams

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Member represents a single agent in a team.
type Member struct {
	AgentID   string `json:"agentId"`
	Name      string `json:"name"`
	AgentType string `json:"agentType"`
	// TmuxPaneID is populated when the member is running in split-pane mode.
	TmuxPaneID string `json:"tmuxPaneId"`
	// SessionID may be populated for non-lead members in future Claude Code versions.
	SessionID string `json:"sessionId"`
}

// Team represents a Claude Code agent team parsed from its config file.
type Team struct {
	Name          string   `json:"name"`
	LeadSessionID string   `json:"leadSessionId"`
	Members       []Member `json:"members"`
}

// Store reads team configs from ~/.claude/teams/ and answers membership queries.
type Store struct {
	dir   string
	teams []Team
}

// NewStore creates a Store backed by the given directory (typically ~/.claude/teams).
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Load (re)reads all team config files from disk.
// Silently ignores missing directories or malformed configs.
func (s *Store) Load() error {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		s.teams = nil
		return nil
	}
	if err != nil {
		return err
	}

	var teams []Team
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name(), "config.json"))
		if err != nil {
			continue
		}
		var t Team
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		teams = append(teams, t)
	}
	s.teams = teams
	return nil
}

// TeamForSession returns the team name for the given session, or "" if not found.
// Matching priority: tmux pane ID on any member, then Claude session ID on lead/members.
func (s *Store) TeamForSession(paneID, sessionID string) string {
	for _, t := range s.teams {
		// Match lead by Claude session ID (pane ID is usually empty for the lead).
		if sessionID != "" && t.LeadSessionID == sessionID {
			return t.Name
		}
		for _, m := range t.Members {
			if paneID != "" && m.TmuxPaneID != "" && m.TmuxPaneID == paneID {
				return t.Name
			}
			if sessionID != "" && m.SessionID != "" && m.SessionID == sessionID {
				return t.Name
			}
		}
	}
	return ""
}
