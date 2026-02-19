package diff

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Hunk represents a single diff hunk within a file.
type Hunk struct {
	OldStart int    // Starting line in old file
	OldCount int    // Number of lines in old file
	NewStart int    // Starting line in new file
	NewCount int    // Number of lines in new file
	Header   string // The @@ line
	Lines    []Line // The actual diff lines
}

// Line represents a single line in a diff.
type Line struct {
	Type    LineType
	Content string // Line content without the +/- prefix
	OldNum  int    // Line number in old file (0 if not applicable)
	NewNum  int    // Line number in new file (0 if not applicable)
}

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

// FileDiff represents the diff for a single file.
type FileDiff struct {
	OldPath string
	NewPath string
	Hunks   []Hunk
	Binary  bool
}

// Diff represents a complete git diff.
type Diff struct {
	Files []FileDiff
}

var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// Parse parses a unified diff string into a structured Diff.
func Parse(diffText string) (*Diff, error) {
	diff := &Diff{}
	scanner := bufio.NewScanner(strings.NewReader(diffText))

	var currentFile *FileDiff
	var currentHunk *Hunk
	var oldLineNum, newLineNum int

	for scanner.Scan() {
		line := scanner.Text()

		// New file diff starts with "diff --git"
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				diff.Files = append(diff.Files, *currentFile)
			}
			currentFile = &FileDiff{}
			currentHunk = nil

			// Parse paths from "diff --git a/path b/path"
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				currentFile.OldPath = strings.TrimPrefix(parts[2], "a/")
				currentFile.NewPath = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		// Check for binary file
		if strings.HasPrefix(line, "Binary files") {
			currentFile.Binary = true
			continue
		}

		// Parse --- and +++ lines for file paths
		if strings.HasPrefix(line, "--- ") {
			path := strings.TrimPrefix(line, "--- ")
			if path != "/dev/null" {
				currentFile.OldPath = strings.TrimPrefix(path, "a/")
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimPrefix(line, "+++ ")
			if path != "/dev/null" {
				currentFile.NewPath = strings.TrimPrefix(path, "b/")
			}
			continue
		}

		// Parse hunk header
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Header:   line,
			}
			oldLineNum = oldStart
			newLineNum = newStart
			continue
		}

		// Parse diff lines
		if currentHunk != nil && len(line) > 0 {
			diffLine := Line{}
			switch line[0] {
			case '+':
				diffLine.Type = LineAdded
				diffLine.Content = line[1:]
				diffLine.NewNum = newLineNum
				newLineNum++
			case '-':
				diffLine.Type = LineRemoved
				diffLine.Content = line[1:]
				diffLine.OldNum = oldLineNum
				oldLineNum++
			case ' ':
				diffLine.Type = LineContext
				diffLine.Content = line[1:]
				diffLine.OldNum = oldLineNum
				diffLine.NewNum = newLineNum
				oldLineNum++
				newLineNum++
			default:
				// Could be "\ No newline at end of file" or other
				continue
			}
			currentHunk.Lines = append(currentHunk.Lines, diffLine)
		}
	}

	// Don't forget the last file/hunk
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		diff.Files = append(diff.Files, *currentFile)
	}

	return diff, scanner.Err()
}

// GetGitDiff runs git diff in the specified directory and returns the output.
func GetGitDiff(dir string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Try without HEAD (for repos with no commits yet)
		cmd = exec.Command("git", "diff")
		cmd.Dir = dir
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(out), nil
}

// GetGitDiffCached runs git diff --cached in the specified directory.
func GetGitDiffCached(dir string) (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetGitRoot returns the git repository root for the given path.
func GetGitRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetFilePath returns the display path for a file diff.
func (f *FileDiff) GetFilePath() string {
	if f.NewPath != "" && f.NewPath != "/dev/null" {
		return f.NewPath
	}
	return f.OldPath
}

// GetFileName returns just the filename portion.
func (f *FileDiff) GetFileName() string {
	return filepath.Base(f.GetFilePath())
}

// TotalLines returns the total number of diff lines across all hunks.
func (f *FileDiff) TotalLines() int {
	total := 0
	for _, h := range f.Hunks {
		total += len(h.Lines)
	}
	return total
}

// IsEmpty returns true if the diff has no files.
func (d *Diff) IsEmpty() bool {
	return len(d.Files) == 0
}

// TotalFiles returns the number of files in the diff.
func (d *Diff) TotalFiles() int {
	return len(d.Files)
}
