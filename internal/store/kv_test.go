package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadNonexistentFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "missing.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("Load from nonexistent file should succeed, got: %v", err)
	}
	if got := s.Get("any"); got != "" {
		t.Fatalf("expected empty string for missing key, got %q", got)
	}
}

func TestLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte(`{"a":"1","b":"2"}`), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got := s.Get("a"); got != "1" {
		t.Fatalf("expected \"1\", got %q", got)
	}
	if got := s.Get("b"); got != "2" {
		t.Fatalf("expected \"2\", got %q", got)
	}
}

func TestGetExistingKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "data.json"))
	_ = s.Set("key", "value")

	if got := s.Get("key"); got != "value" {
		t.Fatalf("expected \"value\", got %q", got)
	}
}

func TestGetMissingKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "data.json"))
	if got := s.Get("nope"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSetPersistReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := NewStore(path)
	if err := s.Set("x", "42"); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if got := s2.Get("x"); got != "42" {
		t.Fatalf("expected \"42\" after reload, got %q", got)
	}
}

func TestSetEmptyDeletesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := NewStore(path)
	_ = s.Set("k", "v")
	_ = s.Set("k", "")

	if got := s.Get("k"); got != "" {
		t.Fatalf("expected key deleted, got %q", got)
	}

	// Verify persisted
	s2 := NewStore(path)
	_ = s2.Load()
	if got := s2.Get("k"); got != "" {
		t.Fatalf("expected key deleted on disk, got %q", got)
	}
}

func TestDeletePersistReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := NewStore(path)
	_ = s.Set("a", "1")
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if got := s.Get("a"); got != "" {
		t.Fatalf("expected empty after delete, got %q", got)
	}

	s2 := NewStore(path)
	_ = s2.Load()
	if got := s2.Get("a"); got != "" {
		t.Fatalf("expected empty after reload, got %q", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := NewStore(filepath.Join(t.TempDir(), "data.json"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		key := "key"
		go func() {
			defer wg.Done()
			_ = s.Set(key, "value")
		}()
		go func() {
			defer wg.Done()
			_ = s.Get(key)
		}()
	}
	wg.Wait()
}

func TestAllReturnsCopy(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "data.json"))
	_ = s.Set("a", "1")
	_ = s.Set("b", "2")

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	// Mutating the returned map should not affect the store
	all["a"] = "modified"
	all["c"] = "new"

	if got := s.Get("a"); got != "1" {
		t.Fatalf("store mutated via All() copy: got %q, want \"1\"", got)
	}
	if got := s.Get("c"); got != "" {
		t.Fatalf("store mutated via All() copy: got %q for new key", got)
	}
}
