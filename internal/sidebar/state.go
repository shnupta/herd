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

// Store manages sidebar state persistence for a specific file path.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a new Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads the sidebar state from disk.
// Returns empty state if file doesn't exist.
func (s *Store) Load() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Pinned: make(map[string]int),
				Order:  nil,
			}, nil
		}
		return nil, err
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Pinned == nil {
		st.Pinned = make(map[string]int)
	}
	return &st, nil
}

// Save writes the sidebar state to disk.
func (s *Store) Save(st *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

var defaultStore *Store

func init() {
	home, _ := os.UserHomeDir()
	defaultStore = NewStore(filepath.Join(home, ".herd", "sidebar.json"))
}

// Load reads the sidebar state from disk using the default store.
func Load() (*State, error) {
	return defaultStore.Load()
}

// Save writes the sidebar state to disk using the default store.
func Save(s *State) error {
	return defaultStore.Save(s)
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
