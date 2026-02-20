package tmux

import (
	"testing"
)

func TestIsClaudePane(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"claude", true},
		{"Claude", true},
		{"CLAUDE", true},
		{"2.1.47", true},
		{"10.0.0", true},
		{"bash", false},
		{"vim", false},
		{"1.2", false},     // only 2 parts
		{"1.2.3.4", false}, // 4 parts
		{"a.b.c", false},   // non-numeric segments
		{"", false},
	}
	for _, tt := range tests {
		if got := IsClaudePane(tt.cmd); got != tt.want {
			t.Errorf("IsClaudePane(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestParsePaneLineHappyPath(t *testing.T) {
	line := "%5\t$2\tmysession\t1\t0\t12345\tbash\t/home/user\t120\t40"
	p, ok := parsePaneLine(line)
	if !ok {
		t.Fatal("parsePaneLine returned ok=false for valid line")
	}
	if p.ID != "%5" {
		t.Errorf("ID = %q, want %%5", p.ID)
	}
	if p.SessionID != "$2" {
		t.Errorf("SessionID = %q, want $2", p.SessionID)
	}
	if p.SessionName != "mysession" {
		t.Errorf("SessionName = %q, want mysession", p.SessionName)
	}
	if p.WindowIndex != 1 {
		t.Errorf("WindowIndex = %d, want 1", p.WindowIndex)
	}
	if p.PaneIndex != 0 {
		t.Errorf("PaneIndex = %d, want 0", p.PaneIndex)
	}
	if p.PID != 12345 {
		t.Errorf("PID = %d, want 12345", p.PID)
	}
	if p.CurrentCmd != "bash" {
		t.Errorf("CurrentCmd = %q, want bash", p.CurrentCmd)
	}
	if p.CurrentPath != "/home/user" {
		t.Errorf("CurrentPath = %q, want /home/user", p.CurrentPath)
	}
	if p.Width != 120 {
		t.Errorf("Width = %d, want 120", p.Width)
	}
	if p.Height != 40 {
		t.Errorf("Height = %d, want 40", p.Height)
	}
}

func TestParsePaneLineTooFewFields(t *testing.T) {
	_, ok := parsePaneLine("only\ttwo\tfields")
	if ok {
		t.Error("parsePaneLine should return ok=false for too few fields")
	}

	_, ok = parsePaneLine("")
	if ok {
		t.Error("parsePaneLine should return ok=false for empty string")
	}
}

func TestParsePaneLineNonNumericFields(t *testing.T) {
	// Non-numeric PID — should still succeed, PID defaults to 0.
	line := "%5\t$2\tmysession\t1\t0\tnotanumber\tbash\t/home/user\t120\t40"
	p, ok := parsePaneLine(line)
	if !ok {
		t.Fatal("parsePaneLine should return ok=true even with non-numeric PID")
	}
	if p.PID != 0 {
		t.Errorf("PID = %d, want 0 for non-numeric input", p.PID)
	}
}
