package diff

import (
	"testing"
)

func TestParseEmpty(t *testing.T) {
	d, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsEmpty() {
		t.Error("expected IsEmpty() == true for empty input")
	}
	if d.TotalFiles() != 0 {
		t.Errorf("TotalFiles() = %d, want 0", d.TotalFiles())
	}
}

func TestParseSingleFile(t *testing.T) {
	raw := "diff --git a/hello.go b/hello.go\n" +
		"--- a/hello.go\n" +
		"+++ b/hello.go\n" +
		"@@ -1,2 +1,2 @@\n" +
		" context\n" +
		"-removed\n" +
		"+added\n"

	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalFiles() != 1 {
		t.Fatalf("TotalFiles() = %d, want 1", d.TotalFiles())
	}

	f := d.Files[0]
	if f.GetFilePath() != "hello.go" {
		t.Errorf("GetFilePath() = %q, want hello.go", f.GetFilePath())
	}
	if f.GetFileName() != "hello.go" {
		t.Errorf("GetFileName() = %q, want hello.go", f.GetFileName())
	}
	if f.TotalLines() != 3 {
		t.Errorf("TotalLines() = %d, want 3", f.TotalLines())
	}

	lines := f.Hunks[0].Lines
	if lines[0].Type != LineContext || lines[0].Content != "context" {
		t.Errorf("line[0]: got type=%v content=%q, want context/context", lines[0].Type, lines[0].Content)
	}
	if lines[1].Type != LineRemoved || lines[1].Content != "removed" {
		t.Errorf("line[1]: got type=%v content=%q, want removed/removed", lines[1].Type, lines[1].Content)
	}
	if lines[2].Type != LineAdded || lines[2].Content != "added" {
		t.Errorf("line[2]: got type=%v content=%q, want added/added", lines[2].Type, lines[2].Content)
	}
}

func TestParseMultiFile(t *testing.T) {
	raw := "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n" +
		"diff --git a/b.go b/b.go\n" +
		"--- a/b.go\n" +
		"+++ b/b.go\n" +
		"@@ -1 +1 @@\n" +
		" context\n"

	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalFiles() != 2 {
		t.Errorf("TotalFiles() = %d, want 2", d.TotalFiles())
	}
	if d.Files[0].GetFilePath() != "a.go" {
		t.Errorf("Files[0] path = %q, want a.go", d.Files[0].GetFilePath())
	}
	if d.Files[1].GetFilePath() != "b.go" {
		t.Errorf("Files[1] path = %q, want b.go", d.Files[1].GetFilePath())
	}
}

func TestParseBinaryFile(t *testing.T) {
	raw := "diff --git a/img.png b/img.png\n" +
		"Binary files a/img.png and b/img.png differ\n"

	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalFiles() != 1 {
		t.Fatalf("TotalFiles() = %d, want 1", d.TotalFiles())
	}
	if !d.Files[0].Binary {
		t.Error("expected Binary == true")
	}
}

func TestParseNewFile(t *testing.T) {
	raw := "diff --git a/new.go b/new.go\n" +
		"--- /dev/null\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1 @@\n" +
		"+package main\n"

	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalFiles() != 1 {
		t.Fatalf("TotalFiles() = %d, want 1", d.TotalFiles())
	}
	if got := d.Files[0].GetFilePath(); got != "new.go" {
		t.Errorf("GetFilePath() = %q, want new.go", got)
	}
}

func TestParseDeletedFile(t *testing.T) {
	raw := "diff --git a/old.go b/old.go\n" +
		"--- a/old.go\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-package main\n"

	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalFiles() != 1 {
		t.Fatalf("TotalFiles() = %d, want 1", d.TotalFiles())
	}
	if got := d.Files[0].GetFilePath(); got == "" {
		t.Error("GetFilePath() returned empty string for deleted file")
	}
}

func TestIsEmpty(t *testing.T) {
	empty := &Diff{}
	if !empty.IsEmpty() {
		t.Error("expected IsEmpty() == true for zero-value Diff")
	}

	nonempty := &Diff{Files: []FileDiff{{}}}
	if nonempty.IsEmpty() {
		t.Error("expected IsEmpty() == false when Files is non-empty")
	}
}

func TestTotalFiles(t *testing.T) {
	d := &Diff{Files: []FileDiff{{}, {}, {}}}
	if d.TotalFiles() != 3 {
		t.Errorf("TotalFiles() = %d, want 3", d.TotalFiles())
	}
}
