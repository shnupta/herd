package domain

import (
	"testing"
)

func TestTruncateLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"truncates long line", "hello world", 5, "hello"},
		{"multiple lines", "hello world\nfoo bar baz", 5, "hello\nfoo b"},
		{"zero width returns input", "hello", 0, "hello"},
		{"empty string", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateLines(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("TruncateLines(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestCleanCapture(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no trailing blanks", "hello\nworld", "hello\nworld"},
		{"trailing blank lines", "hello\nworld\n\n\n", "hello\nworld"},
		{"trailing whitespace lines", "hello\n   \n  \n", "hello"},
		{"all blank", "\n\n\n", ""},
		{"empty string", "", ""},
		{"single line no newline", "hello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanCapture(tt.input)
			if got != tt.want {
				t.Errorf("CleanCapture(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
