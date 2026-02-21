package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcherFiresOnCreate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	ss := SessionState{
		SessionID: "create-test",
		TmuxPane:  "%1",
		State:     "working",
		UpdatedAt: time.Now(),
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	select {
	case got := <-w.Events:
		if got.SessionID != "create-test" {
			t.Errorf("SessionID = %q, want create-test", got.SessionID)
		}
		if got.State != "working" {
			t.Errorf("State = %q, want working", got.State)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for watcher event on create")
	}
}

func TestWatcherFiresOnModify(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create the file before starting the watcher.
	ss := SessionState{
		SessionID: "modify-test",
		TmuxPane:  "%2",
		State:     "working",
		UpdatedAt: time.Now(),
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	// Modify the file.
	ss.State = "waiting"
	ss.UpdatedAt = time.Now()
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	select {
	case got := <-w.Events:
		if got.SessionID != "modify-test" {
			t.Errorf("SessionID = %q, want modify-test", got.SessionID)
		}
		if got.State != "waiting" {
			t.Errorf("State = %q, want waiting", got.State)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for watcher event on modify")
	}
}

func TestWatcherIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	// Write a .tmp file — should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "session.tmp"), []byte("tmp"), 0o644); err != nil {
		t.Fatalf("WriteFile(.tmp) error: %v", err)
	}
	// Write a .txt file — should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("txt"), 0o644); err != nil {
		t.Fatalf("WriteFile(.txt) error: %v", err)
	}

	// Now write a real JSON state file to prove the watcher is working.
	ss := SessionState{
		SessionID: "real-session",
		State:     "idle",
		UpdatedAt: time.Now(),
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	select {
	case got := <-w.Events:
		if got.SessionID != "real-session" {
			t.Errorf("SessionID = %q, want real-session", got.SessionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for watcher event")
	}
}

func TestWatcherIgnoresMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	// Write malformed JSON to a .json file — should not crash, no event emitted.
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not valid}"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Write a valid state to prove the watcher didn't crash.
	ss := SessionState{
		SessionID: "after-bad",
		State:     "working",
		UpdatedAt: time.Now(),
	}
	if err := store.Write(ss); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	select {
	case got := <-w.Events:
		if got.SessionID != "after-bad" {
			t.Errorf("SessionID = %q, want after-bad", got.SessionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out — watcher may have crashed on malformed JSON")
	}
}

func TestWatcherMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	for i, id := range []string{"s1", "s2", "s3"} {
		ss := SessionState{
			SessionID: id,
			State:     "working",
			UpdatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		if err := store.Write(ss); err != nil {
			t.Fatalf("Write(%s) error: %v", id, err)
		}
		// Small delay so fsnotify emits distinct events.
		time.Sleep(50 * time.Millisecond)
	}

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for len(seen) < 3 {
		select {
		case got := <-w.Events:
			seen[got.SessionID] = true
		case <-timeout:
			t.Fatalf("timed out, only saw %d/3 sessions: %v", len(seen), seen)
		}
	}

	for _, id := range []string{"s1", "s2", "s3"} {
		if !seen[id] {
			t.Errorf("missing event for session %q", id)
		}
	}
}

func TestWatcherCloseStopsLoop(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}

	// Close should return without blocking.
	done := make(chan struct{})
	go func() {
		w.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success — Close returned.
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked for too long")
	}

	// Events channel should be closed after loop exits.
	for {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return // channel closed, success
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Events channel not closed after Close()")
		}
	}
}

func TestWatcherConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	w, err := NewWatcherForStore(store)
	if err != nil {
		t.Fatalf("NewWatcherForStore() error: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	// Write files concurrently.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ss := SessionState{
				SessionID: "concurrent-" + string(rune('a'+n)),
				State:     "working",
				UpdatedAt: time.Now(),
			}
			store.Write(ss)
		}(i)
	}
	wg.Wait()

	// Just verify we get at least one event without crashing.
	select {
	case <-w.Events:
		// Got an event, good.
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for any concurrent event")
	}
}
