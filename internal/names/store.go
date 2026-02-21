package names

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
	defaultStore = store.NewStore(filepath.Join(home, ".herd", "names.json"))
	_ = defaultStore.Load()
}

// NewStore creates a names store backed by the given file path.
func NewStore(path string) *store.Store {
	s := store.NewStore(path)
	_ = s.Load()
	return s
}

// Get returns the custom label for the given key, or "" if not set.
func Get(key string) string { return defaultStore.Get(key) }

// Set assigns a custom label for the given key and persists to disk.
func Set(key, label string) error { return defaultStore.Set(key, label) }

// Delete removes the custom label for the given key.
func Delete(key string) error { return defaultStore.Delete(key) }
