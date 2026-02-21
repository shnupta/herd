package domain

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// TruncateLines truncates each line of s to at most maxWidth runes.
// Uses ANSI-aware truncation so escape codes don't corrupt the layout.
func TruncateLines(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, maxWidth, "")
	}
	return strings.Join(lines, "\n")
}

// CleanCapture removes trailing blank lines from a capture string.
func CleanCapture(s string) string {
	lines := strings.Split(s, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[:end], "\n")
}
