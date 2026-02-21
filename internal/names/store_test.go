package names

import (
	"os"
	"path/filepath"
	"sync"
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

func TestStoreOverwriteKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "names.json"))
	_ = s.Set("k", "first")
	_ = s.Set("k", "second")
	if got := s.Get("k"); got != "second" {
		t.Fatalf("expected overwritten value \"second\", got %q", got)
	}
}

func TestStoreSetEmptyRemovesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")
	s := NewStore(path)
	_ = s.Set("sess-1", "my-name")
	_ = s.Set("sess-1", "") // should delete

	if got := s.Get("sess-1"); got != "" {
		t.Fatalf("expected key removed, got %q", got)
	}

	s2 := NewStore(path)
	if got := s2.Get("sess-1"); got != "" {
		t.Fatalf("expected key removed on disk, got %q", got)
	}
}

func TestStoreMultipleKeysAll(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "names.json"))
	_ = s.Set("a", "alpha")
	_ = s.Set("b", "beta")
	_ = s.Set("c", "gamma")

	all := s.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	if all["a"] != "alpha" || all["b"] != "beta" || all["c"] != "gamma" {
		t.Fatalf("unexpected All() result: %v", all)
	}
}

func TestStoreAllReturnsCopy(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "names.json"))
	_ = s.Set("k", "v")

	all := s.All()
	all["k"] = "mutated"

	if got := s.Get("k"); got != "v" {
		t.Fatalf("store mutated via All(): got %q, want \"v\"", got)
	}
}

func TestStoreDeleteNonexistentKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "names.json"))
	if err := s.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete non-existent key should succeed: %v", err)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := NewStore(filepath.Join(t.TempDir(), "names.json"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.Set("key", "value")
		}()
		go func() {
			defer wg.Done()
			_ = s.Get("key")
		}()
	}
	wg.Wait()
}

func TestStoreLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "names.json")
	if err := os.WriteFile(path, []byte(`{corrupt`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	// NewStore calls Load() which returns error, but NewStore ignores it.
	// Store should still be usable; Set should overwrite the malformed file.
	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("Set after malformed load should work: %v", err)
	}
}

func TestPackageLevelGetSetDelete(t *testing.T) {
	// Swap defaultStore to a temp-backed store for testing.
	orig := defaultStore
	defaultStore = NewStore(filepath.Join(t.TempDir(), "names.json"))
	t.Cleanup(func() { defaultStore = orig })

	if got := Get("x"); got != "" {
		t.Fatalf("Get on empty store = %q, want empty", got)
	}
	if err := Set("x", "my-label"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if got := Get("x"); got != "my-label" {
		t.Fatalf("Get() = %q, want \"my-label\"", got)
	}
	if err := Delete("x"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if got := Get("x"); got != "" {
		t.Fatalf("Get after Delete = %q, want empty", got)
	}
}
