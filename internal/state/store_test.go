package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestStoreWriteCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "sessions")
	store := NewStore(dir)
	ss := SessionState{SessionID: "nested-write", State: "idle"}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := os.Stat(store.Path("nested-write")); err != nil {
		t.Errorf("state file not found after Write to nested dir: %v", err)
	}
}

func TestStoreWriteOverwrite(t *testing.T) {
	store := NewStore(t.TempDir())
	ss := SessionState{SessionID: "overwrite", State: "working", UpdatedAt: time.Now()}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	ss.State = "waiting"
	ss.CurrentTool = ""
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() overwrite error: %v", err)
	}

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("ReadAll() = %d states, want 1", len(states))
	}
	if states[0].State != "waiting" {
		t.Errorf("State = %q, want waiting after overwrite", states[0].State)
	}
}

func TestStoreWritePreservesAllFields(t *testing.T) {
	store := NewStore(t.TempDir())
	now := time.Now().Truncate(time.Second)
	ss := SessionState{
		SessionID:   "full-fields",
		TmuxPane:    "%42",
		State:       "plan_ready",
		CurrentTool: "ExitPlanMode",
		ProjectPath: "/home/user/project",
		UpdatedAt:   now,
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("ReadAll() = %d, want 1", len(states))
	}
	got := states[0]
	if got.TmuxPane != "%42" {
		t.Errorf("TmuxPane = %q, want %%42", got.TmuxPane)
	}
	if got.CurrentTool != "ExitPlanMode" {
		t.Errorf("CurrentTool = %q, want ExitPlanMode", got.CurrentTool)
	}
	if got.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath = %q, want /home/user/project", got.ProjectPath)
	}
}

func TestStoreReadAllMultiple(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"a", "b", "c"} {
		if err := store.Write(SessionState{SessionID: id, State: "working"}); err != nil {
			t.Fatalf("Write(%s) error: %v", id, err)
		}
	}

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 3 {
		t.Fatalf("ReadAll() = %d, want 3", len(states))
	}
	ids := map[string]bool{}
	for _, s := range states {
		ids[s.SessionID] = true
	}
	for _, id := range []string{"a", "b", "c"} {
		if !ids[id] {
			t.Errorf("missing session %q", id)
		}
	}
}

func TestStoreReadAllSkipsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.Write(SessionState{SessionID: "readable", State: "idle"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	unreadable := filepath.Join(dir, "noperm.json")
	if err := os.WriteFile(unreadable, []byte(`{"session_id":"hidden"}`), 0o000); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	states, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("ReadAll() = %d, want 1 (unreadable file skipped)", len(states))
	}
}
