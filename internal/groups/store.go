package groups

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store manages persistence of custom session group assignments.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string]string
}

// NewStore creates a new Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path, data: make(map[string]string)}
}

// Load reads the group assignments from disk.
// Returns nil if the file doesn't exist (treated as empty).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]string)
			return nil
		}
		return err
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m == nil {
		m = make(map[string]string)
	}
	s.data = m
	return nil
}

// Get returns the custom group name for the given session key, or "" if not set.
func (s *Store) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key]
}

// Set assigns a custom group for the given session key and persists to disk.
// An empty group string deletes the assignment (reverts to auto-detection).
func (s *Store) Set(key, group string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if group == "" {
		delete(s.data, key)
	} else {
		s.data[key] = group
	}
	return s.save()
}

// save writes the current data to disk. Caller must hold mu.
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
