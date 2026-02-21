package groups

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGetSetDeleteRoundtrip(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))
	if err := s.Set("sess-1", "backend"); err != nil {
		t.Fatal(err)
	}
	if got := s.Get("sess-1"); got != "backend" {
		t.Fatalf("expected \"backend\", got %q", got)
	}
	if err := s.Delete("sess-1"); err != nil {
		t.Fatal(err)
	}
	if got := s.Get("sess-1"); got != "" {
		t.Fatalf("expected empty after delete, got %q", got)
	}
}

func TestPersistenceAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	s1 := NewStore(path)
	if err := s1.Set("sess-1", "infra"); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(path) // new instance, same file
	if got := s2.Get("sess-1"); got != "infra" {
		t.Fatalf("expected \"infra\" from new instance, got %q", got)
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope.json"))
	// NewStore calls Load internally; should not panic or error
	if got := s.Get("any"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSetEmptyRemovesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	s := NewStore(path)
	_ = s.Set("sess-1", "frontend")
	_ = s.Set("sess-1", "") // should delete

	if got := s.Get("sess-1"); got != "" {
		t.Fatalf("expected key removed, got %q", got)
	}

	// Verify persisted
	s2 := NewStore(path)
	if got := s2.Get("sess-1"); got != "" {
		t.Fatalf("expected key removed on disk, got %q", got)
	}
}

func TestOverwriteKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))
	_ = s.Set("sess-1", "backend")
	_ = s.Set("sess-1", "frontend")
	if got := s.Get("sess-1"); got != "frontend" {
		t.Fatalf("expected overwritten value \"frontend\", got %q", got)
	}
}

func TestMultipleKeys(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))
	_ = s.Set("a", "group-a")
	_ = s.Set("b", "group-b")
	_ = s.Set("c", "group-c")

	all := s.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	if all["a"] != "group-a" || all["b"] != "group-b" || all["c"] != "group-c" {
		t.Fatalf("unexpected All() result: %v", all)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))
	_ = s.Set("k", "v")

	all := s.All()
	all["k"] = "mutated"
	all["new"] = "injected"

	if got := s.Get("k"); got != "v" {
		t.Fatalf("store mutated via All(): got %q, want \"v\"", got)
	}
	if got := s.Get("new"); got != "" {
		t.Fatalf("store mutated via All(): got %q for injected key", got)
	}
}

func TestDeleteNonexistentKey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))
	if err := s.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete non-existent key should succeed: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := NewStore(filepath.Join(t.TempDir(), "groups.json"))

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

func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "groups.json")
	if err := os.WriteFile(path, []byte(`{not valid`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	// NewStore calls Load() internally which returns an error,
	// but NewStore ignores it. The store should still be usable.
	// Set should overwrite the malformed file.
	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("Set after malformed load should work: %v", err)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "deep", "nested", "groups.json"))
	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("Set should create missing parent directories: %v", err)
	}
}

func TestPackageLevelGetSetDelete(t *testing.T) {
	// Swap defaultStore to a temp-backed store for testing.
	orig := defaultStore
	defaultStore = NewStore(filepath.Join(t.TempDir(), "groups.json"))
	t.Cleanup(func() { defaultStore = orig })

	if got := Get("x"); got != "" {
		t.Fatalf("Get on empty store = %q, want empty", got)
	}
	if err := Set("x", "mygroup"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if got := Get("x"); got != "mygroup" {
		t.Fatalf("Get() = %q, want \"mygroup\"", got)
	}
	if err := Delete("x"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if got := Get("x"); got != "" {
		t.Fatalf("Get after Delete = %q, want empty", got)
	}
}
