package review

import (
	"strings"
	"testing"

	"github.com/shnupta/herd/internal/diff"
)

func TestAddComment(t *testing.T) {
	r := NewReview("session1", "/project")
	r.AddComment("file.go", 10, 0, 2, "fix this")

	if !r.HasComments() {
		t.Error("HasComments() = false, want true after AddComment")
	}
	if len(r.Comments) != 1 {
		t.Fatalf("len(Comments) = %d, want 1", len(r.Comments))
	}
	c := r.Comments[0]
	if c.FilePath != "file.go" || c.LineNum != 10 || c.HunkIndex != 0 || c.LineIndex != 2 || c.Text != "fix this" {
		t.Errorf("comment fields mismatch: %+v", c)
	}
}

func TestRemoveComment(t *testing.T) {
	r := NewReview("session1", "/project")
	r.AddComment("file.go", 10, 0, 2, "first")
	r.AddComment("file.go", 20, 0, 5, "second")

	r.RemoveComment(0)
	if len(r.Comments) != 1 {
		t.Fatalf("len(Comments) = %d, want 1 after remove", len(r.Comments))
	}
	if r.Comments[0].Text != "second" {
		t.Errorf("remaining comment text = %q, want second", r.Comments[0].Text)
	}
}

func TestRemoveCommentOutOfBounds(t *testing.T) {
	r := NewReview("session1", "/project")
	r.AddComment("file.go", 10, 0, 2, "keep")

	r.RemoveComment(5)  // past end — no-op
	r.RemoveComment(-1) // negative — no-op

	if len(r.Comments) != 1 {
		t.Errorf("len(Comments) = %d, want 1 (out-of-bounds remove should be no-op)", len(r.Comments))
	}
}

func TestGetCommentsForFile(t *testing.T) {
	r := NewReview("session1", "/project")
	r.AddComment("a.go", 1, 0, 0, "for a")
	r.AddComment("b.go", 2, 0, 1, "for b")
	r.AddComment("a.go", 3, 0, 2, "for a again")

	aComments := r.GetCommentsForFile("a.go")
	if len(aComments) != 2 {
		t.Errorf("GetCommentsForFile(a.go) = %d comments, want 2", len(aComments))
	}

	bComments := r.GetCommentsForFile("b.go")
	if len(bComments) != 1 {
		t.Errorf("GetCommentsForFile(b.go) = %d comments, want 1", len(bComments))
	}

	noneComments := r.GetCommentsForFile("nothere.go")
	if len(noneComments) != 0 {
		t.Errorf("GetCommentsForFile(nothere.go) = %d comments, want 0", len(noneComments))
	}
}

func TestGetCommentForLine(t *testing.T) {
	r := NewReview("session1", "/project")
	r.AddComment("file.go", 10, 1, 3, "specific comment")

	c := r.GetCommentForLine("file.go", 1, 3)
	if c == nil {
		t.Fatal("GetCommentForLine returned nil, want comment")
	}
	if c.Text != "specific comment" {
		t.Errorf("Text = %q, want specific comment", c.Text)
	}

	// Wrong hunk index
	if c2 := r.GetCommentForLine("file.go", 2, 3); c2 != nil {
		t.Error("GetCommentForLine with wrong hunk should return nil")
	}

	// Wrong line index
	if c3 := r.GetCommentForLine("file.go", 1, 4); c3 != nil {
		t.Error("GetCommentForLine with wrong line should return nil")
	}
}

func TestFormatFeedbackEmpty(t *testing.T) {
	r := NewReview("session1", "/project")
	d := &diff.Diff{}
	if fb := r.FormatFeedback(d); fb != "" {
		t.Errorf("FormatFeedback with no comments = %q, want empty string", fb)
	}
}

func TestFormatFeedbackWithComments(t *testing.T) {
	raw := "diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,2 +1,2 @@\n" +
		" context\n" +
		"-old\n" +
		"+new\n"

	d, err := diff.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReview("session1", "/project")
	// Comment on the "+new" line: hunkIndex=0, lineIndex=2
	r.AddComment("main.go", 2, 0, 2, "review comment")

	fb := r.FormatFeedback(d)
	if fb == "" {
		t.Error("FormatFeedback returned empty string, want non-empty")
	}
	if !strings.Contains(fb, "main.go") {
		t.Errorf("FormatFeedback missing file path, got: %q", fb)
	}
	if !strings.Contains(fb, "review comment") {
		t.Errorf("FormatFeedback missing comment text, got: %q", fb)
	}
	if !strings.Contains(fb, "Please address this feedback.") {
		t.Errorf("FormatFeedback missing closing text, got: %q", fb)
	}
}

func TestStorageSaveLoadDeleteExists(t *testing.T) {
	dir := t.TempDir()
	storage := NewStorage(dir)

	r := NewReview("test-session", "/myproject")
	r.AddComment("main.go", 5, 0, 1, "test comment")

	// Save
	if err := storage.Save(r); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Exists
	if !storage.Exists("test-session") {
		t.Error("Exists() = false after Save, want true")
	}

	// Load
	loaded, err := storage.Load("test-session")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want test-session", loaded.SessionID)
	}
	if len(loaded.Comments) != 1 || loaded.Comments[0].Text != "test comment" {
		t.Errorf("Comments mismatch: %v", loaded.Comments)
	}

	// Delete
	if err := storage.Delete("test-session"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if storage.Exists("test-session") {
		t.Error("Exists() = true after Delete, want false")
	}
}

func TestStorageLoadNonexistent(t *testing.T) {
	storage := NewStorage(t.TempDir())
	_, err := storage.Load("nonexistent")
	if err == nil {
		t.Error("Load() of nonexistent session should return error")
	}
}
