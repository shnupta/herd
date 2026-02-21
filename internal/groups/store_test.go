package groups

import (
	"path/filepath"
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
