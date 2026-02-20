package sidebar

import (
	"path/filepath"
	"testing"
)

func TestAddToOrder(t *testing.T) {
	s := &State{Pinned: make(map[string]int)}

	// Add a new project
	s.AddToOrder("/proj/one")
	if len(s.Order) != 1 || s.Order[0] != "/proj/one" {
		t.Errorf("Order = %v, want [/proj/one]", s.Order)
	}

	// Add same project again — should be a no-op
	s.AddToOrder("/proj/one")
	if len(s.Order) != 1 {
		t.Errorf("Order length = %d, want 1 (duplicate should be ignored)", len(s.Order))
	}

	// Add a different project
	s.AddToOrder("/proj/two")
	if len(s.Order) != 2 {
		t.Errorf("Order length = %d, want 2", len(s.Order))
	}
}

func TestCleanup(t *testing.T) {
	s := &State{
		Pinned: map[string]int{"/active": 1, "/gone": 2},
		Order:  []string{"/active", "/gone", "/also-gone"},
	}

	active := map[string]bool{"/active": true}
	s.Cleanup(active)

	if _, ok := s.Pinned["/gone"]; ok {
		t.Error("Cleanup should remove /gone from Pinned")
	}
	if _, ok := s.Pinned["/active"]; !ok {
		t.Error("Cleanup should keep /active in Pinned")
	}
	if len(s.Order) != 1 || s.Order[0] != "/active" {
		t.Errorf("Order = %v, want [/active]", s.Order)
	}
}

func TestStoreLoadNonexistent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "sidebar.json"))

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if st == nil {
		t.Fatal("Load() returned nil state")
	}
	if st.Pinned == nil {
		t.Error("Load() should initialize Pinned map")
	}
	if len(st.Order) != 0 {
		t.Errorf("Order = %v, want empty", st.Order)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "sidebar.json"))

	original := &State{
		Pinned: map[string]int{"/proj/one": 1, "/proj/two": 2},
		Order:  []string{"/proj/one", "/proj/two"},
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Order) != 2 {
		t.Fatalf("Order length = %d, want 2", len(loaded.Order))
	}
	if loaded.Order[0] != "/proj/one" {
		t.Errorf("Order[0] = %q, want /proj/one", loaded.Order[0])
	}
	if loaded.Pinned["/proj/two"] != 2 {
		t.Errorf("Pinned[/proj/two] = %d, want 2", loaded.Pinned["/proj/two"])
	}
}

func TestStoreSaveCreatesDirectory(t *testing.T) {
	// Store path inside a subdirectory that doesn't exist yet.
	store := NewStore(filepath.Join(t.TempDir(), "subdir", "nested", "sidebar.json"))

	st := &State{Pinned: make(map[string]int)}
	if err := store.Save(st); err != nil {
		t.Fatalf("Save() error when directory doesn't exist: %v", err)
	}
}
