package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store is a thread-safe key-value store backed by a JSON file.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string]string
}

// NewStore creates a new Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path, data: make(map[string]string)}
}

// Load reads the store contents from disk.
// Returns nil if the file doesn't exist (treated as empty).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]string)
			return nil
		}
		return err
	}

	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if m == nil {
		m = make(map[string]string)
	}
	s.data = m
	return nil
}

// Get returns the value for the given key, or "" if not set.
func (s *Store) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key]
}

// Set assigns a value for the given key and persists to disk.
// An empty value deletes the key.
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value == "" {
		delete(s.data, key)
	} else {
		s.data[key] = value
	}
	return s.save()
}

// Delete removes the given key and persists to disk.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return s.save()
}

// All returns a copy of all key-value pairs.
func (s *Store) All() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]string, len(s.data))
	for k, v := range s.data {
		cp[k] = v
	}
	return cp
}

// save writes the current data to disk. Caller must hold mu.
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0644)
}
