// Package sidebar handles persistence of sidebar session state (pins, ordering).
package sidebar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// State represents the persisted sidebar state.
type State struct {
	// Pinned maps project path to pin order (lower = pinned earlier)
	Pinned map[string]int `json:"pinned"`
	// Order is the list of project paths in display order
	Order []string `json:"order"`
}

var (
	stateMu   sync.Mutex
	statePath string
)

func init() {
	home, _ := os.UserHomeDir()
	statePath = filepath.Join(home, ".herd", "sidebar.json")
}

// Load reads the sidebar state from disk.
// Returns empty state if file doesn't exist.
func Load() (*State, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Pinned: make(map[string]int),
				Order:  nil,
			}, nil
		}
		return nil, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Pinned == nil {
		s.Pinned = make(map[string]int)
	}
	return &s, nil
}

// Save writes the sidebar state to disk.
func Save(s *State) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0644)
}

// Cleanup removes entries for projects that are no longer active.
func (s *State) Cleanup(activeProjects map[string]bool) {
	// Clean pinned
	for project := range s.Pinned {
		if !activeProjects[project] {
			delete(s.Pinned, project)
		}
	}

	// Clean order
	var newOrder []string
	for _, project := range s.Order {
		if activeProjects[project] {
			newOrder = append(newOrder, project)
		}
	}
	s.Order = newOrder
}

// AddToOrder adds a project to the order list if not already present.
func (s *State) AddToOrder(project string) {
	for _, p := range s.Order {
		if p == project {
			return
		}
	}
	s.Order = append(s.Order, project)
}
