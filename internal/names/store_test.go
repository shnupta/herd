package names

import (
	"path/filepath"
	"testing"
)

func TestStoreGetSetDelete(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "names.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got := store.Get("session:abc"); got != "" {
		t.Errorf("Get() on empty store = %q, want empty", got)
	}

	if err := store.Set("session:abc", "my label"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if got := store.Get("session:abc"); got != "my label" {
		t.Errorf("Get() = %q, want %q", got, "my label")
	}

	if err := store.Delete("session:abc"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if got := store.Get("session:abc"); got != "" {
		t.Errorf("Get() after Delete() = %q, want empty", got)
	}
}

func TestStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")

	s1 := NewStore(path)
	if err := s1.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := s1.Set("pane:%1", "alpha"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if err := s1.Set("session:xyz", "beta"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() on second store error: %v", err)
	}
	if got := s2.Get("pane:%1"); got != "alpha" {
		t.Errorf("Get(pane:%%1) = %q, want %q", got, "alpha")
	}
	if got := s2.Get("session:xyz"); got != "beta" {
		t.Errorf("Get(session:xyz) = %q, want %q", got, "beta")
	}
}

func TestStoreLoadNonexistent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "names.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() on nonexistent file error: %v", err)
	}
	if got := store.Get("anything"); got != "" {
		t.Errorf("Get() on empty store = %q, want empty", got)
	}
}

func TestStoreSaveCreatesDirectory(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "subdir", "nested", "names.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := store.Set("k", "v"); err != nil {
		t.Fatalf("Set() error when directory doesn't exist: %v", err)
	}
}
