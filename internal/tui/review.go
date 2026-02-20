package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shnupta/herd/internal/diff"
	"github.com/shnupta/herd/internal/review"
)

// ReviewModel is the bubbletea model for diff review.
type ReviewModel struct {
	diff        *diff.Diff
	review      *review.Review
	projectPath string
	sessionID   string

	// Navigation
	fileIndex int // Current file
	hunkIndex int // Current hunk within file
	lineIndex int // Current line within hunk

	// Dimensions
	width  int
	height int

	// Components
	viewport viewport.Model
	textarea textarea.Model

	// State
	ready        bool
	commenting   bool // True when entering a comment
	submitted    bool // True when review was submitted
	cancelled    bool // True when review was cancelled
	feedbackText string // The formatted feedback to send

	// Flattened view of all lines for easier navigation
	flatLines []flatLine
	flatIndex int
}

type flatLine struct {
	fileIndex int
	hunkIndex int
	lineIndex int
	file      *diff.FileDiff
	hunk      *diff.Hunk
	line      *diff.Line
	isHeader  bool // True for hunk headers
}

// ReviewKeyMap defines the key bindings for the review UI.
type ReviewKeyMap struct {
	Up        key.Binding
	Down      key.Binding
	NextHunk  key.Binding
	PrevHunk  key.Binding
	NextFile  key.Binding
	PrevFile  key.Binding
	Comment   key.Binding
	Delete    key.Binding
	Submit    key.Binding
	Pause     key.Binding
	Quit      key.Binding
}

var reviewKeys = ReviewKeyMap{
	Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	NextHunk:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next hunk")),
	PrevHunk:  key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev hunk")),
	NextFile:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "next file")),
	PrevFile:  key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "prev file")),
	Comment:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment/edit")),
	Delete:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete comment")),
	Submit:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "submit")),
	Pause:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pause")),
	Quit:      key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "cancel")),
}

// Styles for the review UI
var (
	reviewHeaderStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	reviewFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Bold(true)

	reviewHunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	reviewAddedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981"))

	reviewRemovedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444"))

	reviewContextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF"))

	reviewSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#374151"))

	reviewCommentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Italic(true)

	reviewHelpStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#6B7280")).
			Padding(0, 1)

	reviewCommentInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#F59E0B")).
				Padding(0, 1)
)

// NewReviewModel creates a new review model.
func NewReviewModel(d *diff.Diff, sessionID, projectPath string) ReviewModel {
	ta := textarea.New()
	ta.Placeholder = "Enter your comment..."
	ta.CharLimit = 500
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.Focus()

	// Try to load existing review or create new one
	r, err := review.Load(sessionID)
	if err != nil {
		r = review.NewReview(sessionID, projectPath)
	}

	m := ReviewModel{
		diff:        d,
		review:      r,
		sessionID:   sessionID,
		projectPath: projectPath,
		textarea:    ta,
	}

	m.buildFlatLines()
	return m
}

func (m *ReviewModel) buildFlatLines() {
	m.flatLines = nil
	for fi, file := range m.diff.Files {
		for hi, hunk := range file.Hunks {
			// Add hunk header as a line
			m.flatLines = append(m.flatLines, flatLine{
				fileIndex: fi,
				hunkIndex: hi,
				lineIndex: -1,
				file:      &m.diff.Files[fi],
				hunk:      &m.diff.Files[fi].Hunks[hi],
				isHeader:  true,
			})
			// Add each line in the hunk
			for li := range hunk.Lines {
				m.flatLines = append(m.flatLines, flatLine{
					fileIndex: fi,
					hunkIndex: hi,
					lineIndex: li,
					file:      &m.diff.Files[fi],
					hunk:      &m.diff.Files[fi].Hunks[hi],
					line:      &m.diff.Files[fi].Hunks[hi].Lines[li],
				})
			}
		}
	}
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpHeight := m.height - 4 // header + help
		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}
		m.updateViewportContent()

	case tea.KeyMsg:
		if m.commenting {
			switch msg.String() {
			case "esc":
				m.commenting = false
				m.textarea.Reset()
			case "enter":
				if strings.TrimSpace(m.textarea.Value()) != "" {
					m.addCommentAtCursor()
				}
				m.commenting = false
				m.textarea.Reset()
				m.updateViewportContent()
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, reviewKeys.Quit):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, reviewKeys.Up):
			if m.flatIndex > 0 {
				m.flatIndex--
				m.updateViewportContent()
				m.ensureVisible()
			}

		case key.Matches(msg, reviewKeys.Down):
			if m.flatIndex < len(m.flatLines)-1 {
				m.flatIndex++
				m.updateViewportContent()
				m.ensureVisible()
			}

		case key.Matches(msg, reviewKeys.NextHunk):
			m.jumpToNextHunk()
			m.updateViewportContent()
			m.ensureVisible()

		case key.Matches(msg, reviewKeys.PrevHunk):
			m.jumpToPrevHunk()
			m.updateViewportContent()
			m.ensureVisible()

		case key.Matches(msg, reviewKeys.NextFile):
			m.jumpToNextFile()
			m.updateViewportContent()
			m.ensureVisible()

		case key.Matches(msg, reviewKeys.PrevFile):
			m.jumpToPrevFile()
			m.updateViewportContent()
			m.ensureVisible()

		case key.Matches(msg, reviewKeys.Comment):
			if len(m.flatLines) > 0 && !m.flatLines[m.flatIndex].isHeader {
				m.commenting = true
				// Pre-fill with existing comment if any (for editing)
				fl := m.flatLines[m.flatIndex]
				if c := m.review.GetCommentForLine(fl.file.GetFilePath(), fl.hunkIndex, fl.lineIndex); c != nil {
					m.textarea.SetValue(c.Text)
				}
			}

		case key.Matches(msg, reviewKeys.Delete):
			// Delete comment at current line
			if len(m.flatLines) > 0 && !m.flatLines[m.flatIndex].isHeader {
				fl := m.flatLines[m.flatIndex]
				filePath := fl.file.GetFilePath()
				// Find and remove the comment
				for i, c := range m.review.Comments {
					if c.FilePath == filePath && c.HunkIndex == fl.hunkIndex && c.LineIndex == fl.lineIndex {
						m.review.RemoveComment(i)
						m.updateViewportContent()
						break
					}
				}
			}

		case key.Matches(msg, reviewKeys.Submit):
			if m.review.HasComments() {
				m.feedbackText = m.review.FormatFeedback(m.diff)
				m.submitted = true
				_ = review.Delete(m.sessionID) // Clean up saved review
			}
			return m, nil

		case key.Matches(msg, reviewKeys.Pause):
			_ = m.review.Save()
			m.cancelled = true
			return m, nil
		}
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ReviewModel) addCommentAtCursor() {
	if m.flatIndex >= len(m.flatLines) {
		return
	}
	fl := m.flatLines[m.flatIndex]
	if fl.isHeader || fl.line == nil {
		return
	}

	filePath := fl.file.GetFilePath()
	lineNum := fl.line.NewNum
	if lineNum == 0 {
		lineNum = fl.line.OldNum
	}

	// Remove existing comment at this location first
	for i, c := range m.review.Comments {
		if c.FilePath == filePath && c.HunkIndex == fl.hunkIndex && c.LineIndex == fl.lineIndex {
			m.review.RemoveComment(i)
			break
		}
	}

	text := strings.TrimSpace(m.textarea.Value())
	if text != "" {
		m.review.AddComment(filePath, lineNum, fl.hunkIndex, fl.lineIndex, text)
	}
}

func (m *ReviewModel) jumpToNextHunk() {
	if len(m.flatLines) == 0 {
		return
	}
	currentFile := m.flatLines[m.flatIndex].fileIndex
	currentHunk := m.flatLines[m.flatIndex].hunkIndex

	for i := m.flatIndex + 1; i < len(m.flatLines); i++ {
		fl := m.flatLines[i]
		if fl.fileIndex != currentFile || fl.hunkIndex != currentHunk {
			m.flatIndex = i
			return
		}
	}
}

func (m *ReviewModel) jumpToPrevHunk() {
	if len(m.flatLines) == 0 || m.flatIndex == 0 {
		return
	}
	currentFile := m.flatLines[m.flatIndex].fileIndex
	currentHunk := m.flatLines[m.flatIndex].hunkIndex

	// First, go back to start of current hunk
	for m.flatIndex > 0 {
		fl := m.flatLines[m.flatIndex-1]
		if fl.fileIndex != currentFile || fl.hunkIndex != currentHunk {
			break
		}
		m.flatIndex--
	}

	// Then go to previous hunk
	if m.flatIndex > 0 {
		m.flatIndex--
		currentFile = m.flatLines[m.flatIndex].fileIndex
		currentHunk = m.flatLines[m.flatIndex].hunkIndex
		// Go to start of that hunk
		for m.flatIndex > 0 {
			fl := m.flatLines[m.flatIndex-1]
			if fl.fileIndex != currentFile || fl.hunkIndex != currentHunk {
				break
			}
			m.flatIndex--
		}
	}
}

func (m *ReviewModel) jumpToNextFile() {
	if len(m.flatLines) == 0 {
		return
	}
	currentFile := m.flatLines[m.flatIndex].fileIndex

	for i := m.flatIndex + 1; i < len(m.flatLines); i++ {
		if m.flatLines[i].fileIndex != currentFile {
			m.flatIndex = i
			return
		}
	}
}

func (m *ReviewModel) jumpToPrevFile() {
	if len(m.flatLines) == 0 || m.flatIndex == 0 {
		return
	}
	currentFile := m.flatLines[m.flatIndex].fileIndex

	// Go to start of current file
	for m.flatIndex > 0 && m.flatLines[m.flatIndex-1].fileIndex == currentFile {
		m.flatIndex--
	}

	// Go to previous file
	if m.flatIndex > 0 {
		m.flatIndex--
		currentFile = m.flatLines[m.flatIndex].fileIndex
		// Go to start of that file
		for m.flatIndex > 0 && m.flatLines[m.flatIndex-1].fileIndex == currentFile {
			m.flatIndex--
		}
	}
}

func (m *ReviewModel) ensureVisible() {
	// Simple approach: just center on current line
	lineInView := 0
	for i := 0; i < m.flatIndex; i++ {
		lineInView++
		// Account for comments
		fl := m.flatLines[i]
		if !fl.isHeader {
			if c := m.review.GetCommentForLine(fl.file.GetFilePath(), fl.hunkIndex, fl.lineIndex); c != nil {
				lineInView++ // Comment takes an extra line
			}
		}
	}

	// Scroll to make current line visible
	if lineInView < m.viewport.YOffset {
		m.viewport.SetYOffset(lineInView)
	} else if lineInView >= m.viewport.YOffset+m.viewport.Height-2 {
		m.viewport.SetYOffset(lineInView - m.viewport.Height + 3)
	}
}

func (m *ReviewModel) updateViewportContent() {
	if len(m.flatLines) == 0 {
		m.viewport.SetContent("No changes to review")
		return
	}

	var sb strings.Builder
	currentFile := -1

	for i, fl := range m.flatLines {
		// File header
		if fl.fileIndex != currentFile {
			currentFile = fl.fileIndex
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(reviewFileStyle.Render("─── "+fl.file.GetFilePath()+" ───") + "\n")
		}

		isSelected := i == m.flatIndex

		if fl.isHeader {
			line := reviewHunkStyle.Render(fl.hunk.Header)
			if isSelected {
				line = reviewSelectedStyle.Render(line)
			}
			sb.WriteString(line + "\n")
		} else if fl.line != nil {
			// Format the line
			prefix := " "
			style := reviewContextStyle
			switch fl.line.Type {
			case diff.LineAdded:
				prefix = "+"
				style = reviewAddedStyle
			case diff.LineRemoved:
				prefix = "-"
				style = reviewRemovedStyle
			}

			lineNum := ""
			if fl.line.NewNum > 0 {
				lineNum = fmt.Sprintf("%4d ", fl.line.NewNum)
			} else if fl.line.OldNum > 0 {
				lineNum = fmt.Sprintf("%4d ", fl.line.OldNum)
			} else {
				lineNum = "     "
			}

			content := style.Render(prefix + fl.line.Content)
			line := reviewContextStyle.Render(lineNum) + content

			if isSelected {
				line = reviewSelectedStyle.Width(m.width).Render(line)
			}
			sb.WriteString(line + "\n")

			// Show comment if any
			if c := m.review.GetCommentForLine(fl.file.GetFilePath(), fl.hunkIndex, fl.lineIndex); c != nil {
				commentLine := "     💬 " + c.Text
				sb.WriteString(reviewCommentStyle.Render(commentLine) + "\n")
			}
		}
	}

	m.viewport.SetContent(sb.String())
}

func (m ReviewModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.diff.IsEmpty() {
		return "No changes to review"
	}

	// Header
	var currentFile string
	if len(m.flatLines) > 0 && m.flatIndex < len(m.flatLines) {
		currentFile = m.flatLines[m.flatIndex].file.GetFilePath()
	}
	header := reviewHeaderStyle.Width(m.width).Render(
		fmt.Sprintf("Review: %s  (%d/%d files, %d comments)",
			currentFile,
			m.currentFileIndex()+1,
			m.diff.TotalFiles(),
			len(m.review.Comments),
		),
	)

	// Main content
	content := m.viewport.View()

	// Comment input overlay
	if m.commenting {
		inputBox := reviewCommentInputStyle.Render(
			"Comment:\n" + m.textarea.View(),
		)
		// Center the input box
		lines := strings.Split(content, "\n")
		midLine := len(lines) / 2
		if midLine < len(lines) {
			lines[midLine] = inputBox
		}
		content = strings.Join(lines, "\n")
	}

	// Help
	helpText := "[j/k] navigate  [n/N] hunk  [f/F] file  [c] comment  [x] delete  [s] submit  [p] pause  [q] cancel"
	if m.commenting {
		helpText = "[Enter] save comment  [Esc] cancel"
	}
	help := reviewHelpStyle.Width(m.width).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, help)
}

func (m ReviewModel) currentFileIndex() int {
	if len(m.flatLines) > 0 && m.flatIndex < len(m.flatLines) {
		return m.flatLines[m.flatIndex].fileIndex
	}
	return 0
}

// Submitted returns true if the review was submitted.
func (m ReviewModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns true if the review was cancelled.
func (m ReviewModel) Cancelled() bool {
	return m.cancelled
}

// FeedbackText returns the formatted feedback text.
func (m ReviewModel) FeedbackText() string {
	return m.feedbackText
}
