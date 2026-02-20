package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shnupta/herd/internal/tmux"
)

// PickerModel is a project picker for creating new sessions.
type PickerModel struct {
	textinput textinput.Model
	projects  []string // All known project paths
	filtered  []string // Filtered by search
	selected  int
	width     int
	height    int

	// Result
	chosenPath string
	cancelled  bool
}

// PickerKeyMap defines key bindings for the picker.
type PickerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
}

var pickerKeys = PickerKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "ctrl+p")),
	Down:   key.NewBinding(key.WithKeys("down", "ctrl+n")),
	Select: key.NewBinding(key.WithKeys("enter")),
	Cancel: key.NewBinding(key.WithKeys("esc", "ctrl+c")),
}

var (
	pickerTitleStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	pickerItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			PaddingLeft(2)

	pickerSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#374151")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				PaddingLeft(2)

	pickerInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7C3AED")).
				Padding(0, 1)

	pickerHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			PaddingLeft(1)
)

// NewPickerModel creates a new project picker.
func NewPickerModel(existingPaths []string) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	// Deduplicate and sort paths
	seen := make(map[string]bool)
	var projects []string
	for _, p := range existingPaths {
		if p != "" && !seen[p] {
			seen[p] = true
			projects = append(projects, p)
		}
	}

	// Add home directory common project locations
	home, _ := os.UserHomeDir()
	commonDirs := []string{
		filepath.Join(home, "projects"),
		filepath.Join(home, "code"),
		filepath.Join(home, "dev"),
		filepath.Join(home, "src"),
		filepath.Join(home, "work"),
	}
	for _, dir := range commonDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					p := filepath.Join(dir, e.Name())
					if !seen[p] {
						seen[p] = true
						projects = append(projects, p)
					}
				}
			}
		}
	}

	sort.Strings(projects)

	return PickerModel{
		textinput: ti,
		projects:  projects,
		filtered:  projects,
	}
}

func (m PickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textinput.Width = min(50, m.width-10)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, pickerKeys.Cancel):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, pickerKeys.Select):
			if len(m.filtered) > 0 && m.selected < len(m.filtered) {
				m.chosenPath = m.filtered[m.selected]
			}
			return m, nil

		case key.Matches(msg, pickerKeys.Up):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case key.Matches(msg, pickerKeys.Down):
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	// Filter projects based on input
	m.filterProjects()

	return m, tea.Batch(cmds...)
}

func (m *PickerModel) filterProjects() {
	query := strings.ToLower(m.textinput.Value())
	if query == "" {
		m.filtered = m.projects
	} else {
		m.filtered = nil
		for _, p := range m.projects {
			if strings.Contains(strings.ToLower(p), query) {
				m.filtered = append(m.filtered, p)
			}
		}
	}

	// Adjust selection
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
}

func (m PickerModel) View() string {
	var sb strings.Builder

	// Title
	title := pickerTitleStyle.Width(m.width).Render("New Session — Select Project")
	sb.WriteString(title + "\n\n")

	// Search input
	input := pickerInputStyle.Render(m.textinput.View())
	sb.WriteString(input + "\n\n")

	// Project list
	maxVisible := m.height - 8
	if maxVisible < 3 {
		maxVisible = 3
	}

	start := 0
	if m.selected >= maxVisible {
		start = m.selected - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	if len(m.filtered) == 0 {
		sb.WriteString(pickerItemStyle.Render("No matching projects") + "\n")
	} else {
		for i := start; i < end; i++ {
			p := m.filtered[i]
			// Shorten path for display
			display := shortenPath(p)
			if i == m.selected {
				sb.WriteString(pickerSelectedStyle.Width(m.width - 4).Render("▸ " + display) + "\n")
			} else {
				sb.WriteString(pickerItemStyle.Render("  " + display) + "\n")
			}
		}
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(pickerHelpStyle.Render("[↑/↓] navigate  [enter] select  [esc] cancel"))

	return sb.String()
}

// ChosenPath returns the selected project path, empty if none.
func (m PickerModel) ChosenPath() string {
	return m.chosenPath
}

// Cancelled returns true if the picker was cancelled.
func (m PickerModel) Cancelled() bool {
	return m.cancelled
}

// LaunchSession creates a new tmux window with claude in the given directory.
func LaunchSession(projectPath string) error {
	sess, err := tmux.CurrentSession()
	if err != nil {
		return err
	}
	_, err = tmux.NewWindow(sess, projectPath, "claude")
	return err
}

func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
