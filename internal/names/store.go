package names

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store manages persistence of custom session labels.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string]string
}

// NewStore creates a new Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path, data: make(map[string]string)}
}

// Load reads the names from disk into the store.
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

// Get returns the custom label for the given key, or "" if not set.
func (s *Store) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key]
}

// Set assigns a custom label for the given key and persists to disk.
func (s *Store) Set(key, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = label
	return s.save()
}

// Delete removes the custom label for the given key and persists to disk.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
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

var defaultStore *Store

func init() {
	home, _ := os.UserHomeDir()
	defaultStore = NewStore(filepath.Join(home, ".herd", "names.json"))
	_ = defaultStore.Load()
}

// Get returns the custom label using the default store.
func Get(key string) string {
	return defaultStore.Get(key)
}

// Set assigns a custom label using the default store.
func Set(key, label string) error {
	return defaultStore.Set(key, label)
}

// Delete removes the custom label using the default store.
func Delete(key string) error {
	return defaultStore.Delete(key)
}
