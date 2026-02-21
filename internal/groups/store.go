package groups

import (
	"os"
	"path/filepath"

	"github.com/shnupta/herd/internal/store"
)

var defaultStore *store.Store

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	defaultStore = store.NewStore(filepath.Join(home, ".herd", "groups.json"))
	_ = defaultStore.Load()
}

// NewStore creates a group store backed by the given file path.
func NewStore(path string) *store.Store {
	s := store.NewStore(path)
	_ = s.Load()
	return s
}

// Get returns the custom group name for the given session key, or "" if not set.
func Get(key string) string { return defaultStore.Get(key) }

// Set assigns a custom group for the given session key and persists to disk.
// An empty group string deletes the assignment.
func Set(key, value string) error { return defaultStore.Set(key, value) }

// Delete removes the custom group assignment for the given key.
func Delete(key string) error { return defaultStore.Delete(key) }
