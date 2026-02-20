package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shnupta/herd/internal/diff"
)

// Comment represents a review comment on a specific location.
type Comment struct {
	FilePath  string    `json:"file_path"`
	LineNum   int       `json:"line_num"`    // Line number in the new file
	HunkIndex int       `json:"hunk_index"`  // Which hunk this comment is on
	LineIndex int       `json:"line_index"`  // Index within the hunk's lines
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Review represents a complete review session.
type Review struct {
	SessionID string    `json:"session_id"`
	ProjectPath string  `json:"project_path"`
	Comments  []Comment `json:"comments"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewReview creates a new review for the given session.
func NewReview(sessionID, projectPath string) *Review {
	now := time.Now()
	return &Review{
		SessionID:   sessionID,
		ProjectPath: projectPath,
		Comments:    []Comment{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddComment adds a comment to the review.
func (r *Review) AddComment(filePath string, lineNum, hunkIndex, lineIndex int, text string) {
	r.Comments = append(r.Comments, Comment{
		FilePath:  filePath,
		LineNum:   lineNum,
		HunkIndex: hunkIndex,
		LineIndex: lineIndex,
		Text:      text,
		CreatedAt: time.Now(),
	})
	r.UpdatedAt = time.Now()
}

// RemoveComment removes the comment at the given index.
func (r *Review) RemoveComment(index int) {
	if index >= 0 && index < len(r.Comments) {
		r.Comments = append(r.Comments[:index], r.Comments[index+1:]...)
		r.UpdatedAt = time.Now()
	}
}

// GetCommentsForFile returns all comments for a specific file.
func (r *Review) GetCommentsForFile(filePath string) []Comment {
	var comments []Comment
	for _, c := range r.Comments {
		if c.FilePath == filePath {
			comments = append(comments, c)
		}
	}
	return comments
}

// GetCommentForLine returns the comment at a specific location, if any.
func (r *Review) GetCommentForLine(filePath string, hunkIndex, lineIndex int) *Comment {
	for i := range r.Comments {
		c := &r.Comments[i]
		if c.FilePath == filePath && c.HunkIndex == hunkIndex && c.LineIndex == lineIndex {
			return c
		}
	}
	return nil
}

// HasComments returns true if there are any comments.
func (r *Review) HasComments() bool {
	return len(r.Comments) > 0
}

// FormatFeedback formats the review as feedback text to send to the agent.
func (r *Review) FormatFeedback(d *diff.Diff) string {
	if len(r.Comments) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Review of your recent changes:\n\n")

	// Group comments by file
	commentsByFile := make(map[string][]Comment)
	for _, c := range r.Comments {
		commentsByFile[c.FilePath] = append(commentsByFile[c.FilePath], c)
	}

	for _, file := range d.Files {
		filePath := file.GetFilePath()
		comments, ok := commentsByFile[filePath]
		if !ok {
			continue
		}

		for _, comment := range comments {
			// Find the relevant lines from the diff
			if comment.HunkIndex < len(file.Hunks) {
				hunk := file.Hunks[comment.HunkIndex]
				
				// Get context: the commented line and a few around it
				startIdx := comment.LineIndex
				if startIdx < 0 {
					startIdx = 0
				}
				endIdx := comment.LineIndex + 1
				if endIdx > len(hunk.Lines) {
					endIdx = len(hunk.Lines)
				}

				sb.WriteString(fmt.Sprintf("%s:%d\n", filePath, comment.LineNum))
				for i := startIdx; i < endIdx; i++ {
					line := hunk.Lines[i]
					prefix := " "
					switch line.Type {
					case diff.LineAdded:
						prefix = "+"
					case diff.LineRemoved:
						prefix = "-"
					}
					sb.WriteString(fmt.Sprintf("> %s%s\n", prefix, line.Content))
				}
				sb.WriteString(fmt.Sprintf("Comment: %s\n\n", comment.Text))
			}
		}
	}

	sb.WriteString("Please address this feedback.")
	return sb.String()
}

// Storage paths

func reviewDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".herd", "reviews")
}

func reviewPath(sessionID string) string {
	return filepath.Join(reviewDir(), sessionID+".json")
}

// Save persists the review to disk.
func (r *Review) Save() error {
	dir := reviewDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(reviewPath(r.SessionID), data, 0o644)
}

// Load loads a review from disk.
func Load(sessionID string) (*Review, error) {
	data, err := os.ReadFile(reviewPath(sessionID))
	if err != nil {
		return nil, err
	}

	var r Review
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

// Delete removes a saved review from disk.
func Delete(sessionID string) error {
	return os.Remove(reviewPath(sessionID))
}

// Exists checks if a saved review exists for the session.
func Exists(sessionID string) bool {
	_, err := os.Stat(reviewPath(sessionID))
	return err == nil
}
