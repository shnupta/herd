package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/shnupta/herd/internal/session"
)

func (m Model) View() string {
	if !m.ready {
		return "initialising..."
	}
	if m.err != nil {
		return fmt.Sprintf("error: %v\n\nPress q to quit.", m.err)
	}

	header := m.renderHeader()
	outputHeader := m.renderOutputHeader()

	sessionList := m.renderSessionList()
	sessionPane := styleSessionPane.
		Width(sessionPaneWidth).
		Height(m.height - 2). // total - header(1) - help(1)
		Render(sessionList)

	viewportContent := m.viewport.View()
	outputPane := lipgloss.NewStyle().
		Width(m.width - sessionPaneWidth - 1).
		Height(m.viewport.Height).
		Render(viewportContent)

	outputHeader = styleOutputHeader.
		Width(m.width - sessionPaneWidth - 1).
		Render(outputHeader)

	rightCol := lipgloss.JoinVertical(lipgloss.Left, outputHeader, outputPane)
	middle := lipgloss.JoinHorizontal(lipgloss.Top, sessionPane, rightCol)

	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		middle,
		help,
	)
}

func (m Model) renderHeader() string {
	title := "herd"
	sel := m.selectedSession()
	if sel != nil {
		title = fmt.Sprintf("herd  ·  %s", filepath.Base(sel.ProjectPath))
		if sel.GitBranch != "" {
			title += "  [" + sel.GitBranch + "]"
		}
	}
	return styleHeader.Width(m.width).Render(title)
}

func (m Model) renderOutputHeader() string {
	sel := m.selectedSession()
	if sel == nil {
		return "no session selected"
	}

	icon := stateIcon(sel.State.String())
	label := stateLabel(sel.State.String(), sel.CurrentTool)

	// Pane ID anchored to the left section so it never shifts.
	// Scroll % floats to the far right and only appears when not at bottom.
	subtext := lipgloss.NewStyle().Foreground(colSubtext)
	left := icon + " " + label + "  " + subtext.Render(sel.TmuxPane)

	right := ""
	if !m.viewport.AtBottom() {
		pct := int(m.viewport.ScrollPercent() * 100)
		right = subtext.Render(fmt.Sprintf("%d%%", pct))
	}

	gap := m.width - sessionPaneWidth - 1 - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderSessionList() string {
	if len(m.sessions) == 0 {
		return styleSessionMeta.Render("no claude sessions\nfound in tmux")
	}

	var rows []string
	for i, s := range m.sessions {
		rows = append(rows, m.renderSessionItem(i, s))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderSessionItem(i int, s session.Session) string {
	icon := stateIcon(s.State.String())
	name := filepath.Base(s.ProjectPath)
	if name == "." || name == "" {
		name = s.TmuxPane
	}

	nameStyle := styleSessionItem
	if i == m.selected {
		nameStyle = styleSessionItemSelected
	}

	nameLine := nameStyle.
		Width(sessionPaneWidth - 1).
		Render(icon + " " + name)

	// Sub-line: tool name or idle duration
	meta := sessionMeta(s)
	metaLine := styleSessionMeta.
		Width(sessionPaneWidth - 1).
		Render(meta)

	return nameLine + "\n" + metaLine
}

func sessionMeta(s session.Session) string {
	switch s.State {
	case session.StateWorking:
		if s.CurrentTool != "" {
			return s.CurrentTool + "  ⟳"
		}
		return "working  ⟳"
	case session.StateWaiting:
		return "waiting for input"
	case session.StatePlanReady:
		return "plan ready"
	case session.StateNotifying:
		return "notification"
	case session.StateIdle:
		if !s.UpdatedAt.IsZero() {
			return "idle  " + fmtDuration(time.Since(s.UpdatedAt))
		}
		return "idle"
	default:
		if s.GitBranch != "" {
			return s.GitBranch
		}
		return s.TmuxPane
	}
}


func (m Model) renderHelp() string {
	if m.insertMode {
		return styleHelpInsert.Width(m.width).Render("INSERT  [ctrl+h] exit")
	}
	parts := []string{
		"[j/k] navigate",
		"[i] insert",
		"[t] jump",
		"[x] kill",
		"[n] new",
		"[w] worktrees",
		"[r] refresh",
		"[I] install hooks",
	}
	return styleHelp.Width(m.width).Render(strings.Join(parts, "  "))
}

// fmtDuration formats a duration as a short human string, e.g. "2m" or "45s".
func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
