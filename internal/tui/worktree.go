package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shnupta/herd/internal/git"
	"github.com/shnupta/herd/internal/session"
)

type worktreeViewState int

const (
	worktreeStateListing   worktreeViewState = iota
	worktreeStateCreating
	worktreeStateConfirming
)

// WorktreeModel handles the worktree panel UI.
type WorktreeModel struct {
	repoRoot  string
	worktrees []git.Worktree
	sessions  []session.Session
	selected  int // 0 = "New worktree...", 1+ = worktrees[selected-1]
	width     int
	height    int
	state     worktreeViewState

	// Create form
	branchInput  textinput.Model
	pathInput    textinput.Model
	focusedField int  // 0 = branch, 1 = path
	pathManual   bool // true once user has manually edited path

	// Confirm-remove state
	confirmWorktreeIdx int    // index into m.worktrees
	confirmSessionPane string // pane ID of associated session, or ""

	// Result signals
	chosenPath        string
	createPath        string
	createBranch      string
	removeWorktreePath string
	removeSessionPane  string
	cancelled         bool
}

type worktreeKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
	Tab    key.Binding
	Remove key.Binding
}

var worktreeKeys = worktreeKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k")),
	Down:   key.NewBinding(key.WithKeys("down", "j")),
	Select: key.NewBinding(key.WithKeys("enter")),
	Cancel: key.NewBinding(key.WithKeys("esc")),
	Tab:    key.NewBinding(key.WithKeys("tab")),
	Remove: key.NewBinding(key.WithKeys("x")),
}

var (
	worktreeTitleStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	worktreeItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB")).
				PaddingLeft(2)

	worktreeSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#374151")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				PaddingLeft(2)

	worktreeNewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				PaddingLeft(2)

	worktreeNewSelectedStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#374151")).
					Foreground(lipgloss.Color("#10B981")).
					Bold(true).
					PaddingLeft(2)

	worktreeInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7C3AED")).
				Padding(0, 1)

	worktreeHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				PaddingLeft(1)

	worktreeLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Width(8)
)

// NewWorktreeModel creates a WorktreeModel ready for display.
func NewWorktreeModel(worktrees []git.Worktree, repoRoot string, sessions []session.Session, w, h int) WorktreeModel {
	bi := textinput.New()
	bi.Placeholder = "branch name (e.g. feat/payments)"
	bi.Focus()
	bi.CharLimit = 200
	bi.Width = min(50, w-10)

	pi := textinput.New()
	pi.Placeholder = "path"
	pi.CharLimit = 500
	pi.Width = min(50, w-10)

	return WorktreeModel{
		repoRoot:    repoRoot,
		worktrees:   worktrees,
		sessions:    sessions,
		width:       w,
		height:      h,
		branchInput: bi,
		pathInput:   pi,
	}
}

func (m WorktreeModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WorktreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.branchInput.Width = min(50, m.width-10)
		m.pathInput.Width = min(50, m.width-10)
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case worktreeStateCreating:
			return m.updateCreating(msg)
		case worktreeStateConfirming:
			return m.updateConfirming(msg)
		default:
			return m.updateListing(msg)
		}
	}

	return m, nil
}

func (m WorktreeModel) updateListing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	listLen := len(m.worktrees) + 1 // +1 for "New worktree..."

	switch {
	case key.Matches(msg, worktreeKeys.Cancel):
		m.cancelled = true

	case key.Matches(msg, worktreeKeys.Up):
		if m.selected > 0 {
			m.selected--
		}

	case key.Matches(msg, worktreeKeys.Down):
		if m.selected < listLen-1 {
			m.selected++
		}

	case key.Matches(msg, worktreeKeys.Select):
		if m.selected == 0 {
			// Switch to create form.
			m.state = worktreeStateCreating
			m.focusedField = 0
			m.branchInput.Focus()
			m.pathInput.Blur()
			m.branchInput.SetValue("")
			m.pathInput.SetValue("")
			m.pathManual = false
			return m, textinput.Blink
		}
		// Existing worktree selected.
		m.chosenPath = m.worktrees[m.selected-1].Path

	case key.Matches(msg, worktreeKeys.Remove):
		if m.selected == 0 {
			break // no-op on "New worktree..."
		}
		wt := m.worktrees[m.selected-1]
		if wt.IsMain {
			break // no-op on main worktree
		}
		// Find associated session.
		pane := ""
		for _, s := range m.sessions {
			if strings.HasPrefix(s.ProjectPath, wt.Path) {
				pane = s.TmuxPane
				break
			}
		}
		m.confirmWorktreeIdx = m.selected - 1
		m.confirmSessionPane = pane
		m.state = worktreeStateConfirming
	}

	return m, nil
}

func (m WorktreeModel) updateConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, worktreeKeys.Select):
		wt := m.worktrees[m.confirmWorktreeIdx]
		m.removeWorktreePath = wt.Path
		m.removeSessionPane = m.confirmSessionPane
	case key.Matches(msg, worktreeKeys.Cancel):
		m.state = worktreeStateListing
	}
	return m, nil
}

func (m WorktreeModel) updateCreating(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case key.Matches(msg, worktreeKeys.Cancel):
		m.state = worktreeStateListing
		m.branchInput.Blur()
		m.pathInput.Blur()
		return m, nil

	case key.Matches(msg, worktreeKeys.Tab):
		m.focusedField = 1 - m.focusedField
		if m.focusedField == 0 {
			m.branchInput.Focus()
			m.pathInput.Blur()
		} else {
			m.pathInput.Focus()
			m.branchInput.Blur()
		}
		return m, textinput.Blink

	case key.Matches(msg, worktreeKeys.Select):
		branch := strings.TrimSpace(m.branchInput.Value())
		path := strings.TrimSpace(m.pathInput.Value())
		if branch != "" && path != "" {
			m.createBranch = branch
			m.createPath = path
		}
		return m, nil
	}

	// Update the focused input.
	if m.focusedField == 0 {
		m.branchInput, cmd = m.branchInput.Update(msg)
		if !m.pathManual {
			branch := m.branchInput.Value()
			if branch != "" {
				m.pathInput.SetValue(git.DefaultWorktreePath(m.repoRoot, branch))
			} else {
				m.pathInput.SetValue("")
			}
		}
	} else {
		prevPath := m.pathInput.Value()
		m.pathInput, cmd = m.pathInput.Update(msg)
		if m.pathInput.Value() != prevPath {
			m.pathManual = true
		}
	}

	return m, cmd
}

func (m WorktreeModel) View() string {
	repoName := filepath.Base(m.repoRoot)
	switch m.state {
	case worktreeStateCreating:
		return m.viewCreating(repoName)
	case worktreeStateConfirming:
		return m.viewConfirming(repoName)
	default:
		return m.viewListing(repoName)
	}
}

func (m WorktreeModel) viewListing(repoName string) string {
	var sb strings.Builder
	sb.WriteString(worktreeTitleStyle.Width(m.width).Render("Worktrees — "+repoName) + "\n\n")

	// "New worktree..." row
	if m.selected == 0 {
		sb.WriteString(worktreeNewSelectedStyle.Width(m.width-4).Render("▸ + New worktree...") + "\n")
	} else {
		sb.WriteString(worktreeNewStyle.Render("  + New worktree...") + "\n")
	}

	// Existing worktrees
	for i, wt := range m.worktrees {
		listIdx := i + 1
		branch := wt.Branch
		if branch == "" {
			branch = "(detached)"
		}
		label := fmt.Sprintf("%-14s %s", branch, shortenPath(wt.Path))
		if wt.IsMain {
			label += "  [main]"
		}
		if listIdx == m.selected {
			sb.WriteString(worktreeSelectedStyle.Width(m.width-4).Render("▸ "+label) + "\n")
		} else {
			sb.WriteString(worktreeItemStyle.Render("  "+label) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(worktreeHelpStyle.Render("[j/k] nav  [enter] open  [x] remove  [esc] cancel"))
	return sb.String()
}

func (m WorktreeModel) viewConfirming(repoName string) string {
	wt := m.worktrees[m.confirmWorktreeIdx]
	branch := wt.Branch
	if branch == "" {
		branch = "(detached)"
	}
	sessionLine := m.confirmSessionPane
	if sessionLine == "" {
		sessionLine = "none"
	}

	var sb strings.Builder
	sb.WriteString(worktreeTitleStyle.Width(m.width).Render("Remove Worktree — "+repoName) + "\n\n")
	sb.WriteString(worktreeLabelStyle.Render("Branch") + "   " + branch + "\n")
	sb.WriteString(worktreeLabelStyle.Render("Path") + "     " + shortenPath(wt.Path) + "\n")
	sb.WriteString(worktreeLabelStyle.Render("Session") + "  " + sessionLine)
	if m.confirmSessionPane != "" {
		sb.WriteString("  (will be killed)")
	}
	sb.WriteString("\n\n")
	sb.WriteString(worktreeHelpStyle.Render("[enter] confirm  [esc] cancel"))
	return sb.String()
}

func (m WorktreeModel) viewCreating(repoName string) string {
	var sb strings.Builder
	sb.WriteString(worktreeTitleStyle.Width(m.width).Render("New Worktree — "+repoName) + "\n\n")

	branchLine := worktreeLabelStyle.Render("Branch") + "  " + worktreeInputStyle.Render(m.branchInput.View())
	pathLine := worktreeLabelStyle.Render("Path") + "    " + worktreeInputStyle.Render(m.pathInput.View())
	sb.WriteString(branchLine + "\n")
	sb.WriteString(pathLine + "\n\n")
	sb.WriteString(worktreeHelpStyle.Render("[tab] switch field  [enter] create  [esc] back"))
	return sb.String()
}

// ChosenPath returns the path of an existing worktree that was selected, or "".
func (m WorktreeModel) ChosenPath() string {
	return m.chosenPath
}

// ShouldCreate returns the path and branch for a new worktree, with ok=true when ready.
func (m WorktreeModel) ShouldCreate() (path, branch string, ok bool) {
	if m.createPath != "" && m.createBranch != "" {
		return m.createPath, m.createBranch, true
	}
	return "", "", false
}

// ShouldRemove returns the worktree path and associated session pane to kill,
// with ok=true when the user has confirmed a removal.
func (m WorktreeModel) ShouldRemove() (wtPath, sessionPane string, ok bool) {
	if m.removeWorktreePath != "" {
		return m.removeWorktreePath, m.removeSessionPane, true
	}
	return "", "", false
}

// Cancelled returns true if the panel was closed without a selection.
func (m WorktreeModel) Cancelled() bool {
	return m.cancelled
}
