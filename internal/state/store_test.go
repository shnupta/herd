package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreDir(t *testing.T) {
	store := NewStore("/tmp/test-sessions")
	if store.Dir() != "/tmp/test-sessions" {
		t.Errorf("Dir() = %q, want /tmp/test-sessions", store.Dir())
	}
}

func TestStorePath(t *testing.T) {
	store := NewStore("/tmp/sessions")
	want := "/tmp/sessions/abc123.json"
	if got := store.Path("abc123"); got != want {
		t.Errorf("Path(abc123) = %q, want %q", got, want)
	}
}

func TestStoreWriteCreatesFile(t *testing.T) {
	store := NewStore(t.TempDir())
	ss := SessionState{
		SessionID: "session-1",
		TmuxPane:  "%3",
		State:     "working",
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := os.Stat(store.Path("session-1")); err != nil {
		t.Errorf("state file not found after Write: %v", err)
	}
}

func TestStoreReadAllEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() on empty dir error: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("ReadAll() = %d states, want 0", len(states))
	}
}

func TestStoreReadAllNonexistentDir(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"))
	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() on nonexistent dir should not error: %v", err)
	}
	if states != nil {
		t.Errorf("ReadAll() = %v, want nil", states)
	}
}

func TestStoreWriteReadAllRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())

	ss := SessionState{
		SessionID:   "round-trip",
		TmuxPane:    "%7",
		State:       "waiting",
		CurrentTool: "Bash",
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("ReadAll() = %d states, want 1", len(states))
	}
	got := states[0]
	if got.SessionID != "round-trip" {
		t.Errorf("SessionID = %q, want round-trip", got.SessionID)
	}
	if got.State != "waiting" {
		t.Errorf("State = %q, want waiting", got.State)
	}
	if got.CurrentTool != "Bash" {
		t.Errorf("CurrentTool = %q, want Bash", got.CurrentTool)
	}
}

func TestStoreReadAllSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Write a valid state
	ss := SessionState{SessionID: "valid", State: "idle"}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Write an invalid JSON file
	os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not valid json}"), 0644)

	// Write a non-JSON file (should be ignored by extension check)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	// Create a directory named with .json extension (should be skipped)
	os.Mkdir(filepath.Join(dir, "subdir.json"), 0755)

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("ReadAll() = %d states, want 1 (invalid entries should be skipped)", len(states))
	}
	if len(states) > 0 && states[0].SessionID != "valid" {
		t.Errorf("states[0].SessionID = %q, want valid", states[0].SessionID)
	}
}
