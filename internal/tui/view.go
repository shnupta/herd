package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
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

	// If in review mode, show the review UI
	if m.reviewMode && m.reviewModel != nil {
		return m.reviewModel.View()
	}

	// If in picker mode, show the project picker
	if m.pickerMode && m.pickerModel != nil {
		return m.pickerModel.View()
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

	// Add aggregate stats
	stats := m.aggregateStats()
	if stats != "" {
		title += "  ·  " + stats
	}

	return styleHeader.Width(m.width).Render(title)
}

// aggregateStats returns a summary of session states.
func (m Model) aggregateStats() string {
	if len(m.sessions) == 0 {
		return ""
	}

	counts := make(map[session.State]int)
	for _, s := range m.sessions {
		counts[s.State]++
	}

	var parts []string

	// Order matters for readability
	if n := counts[session.StateWorking]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d working", n))
	}
	if n := counts[session.StateWaiting]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", n))
	}
	if n := counts[session.StatePlanReady]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d plan", n))
	}
	if n := counts[session.StateNotifying]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d notify", n))
	}
	if n := counts[session.StateIdle]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d idle", n))
	}

	// If all are unknown/untracked, just show total
	if len(parts) == 0 {
		return fmt.Sprintf("%d sessions", len(m.sessions))
	}

	return strings.Join(parts, "  ")
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
	left := " " + icon + " " + label + "  " + subtext.Render(sel.TmuxPane)

	right := ""
	if !m.viewport.AtBottom() {
		pct := int(m.viewport.ScrollPercent() * 100)
		right = subtext.Render(fmt.Sprintf("%d%%", pct))
	}

	available := m.width - sessionPaneWidth - 1
	gap := available - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	// Hard-truncate so wide emoji / miscounted ANSI never overflow the row.
	result := left + strings.Repeat(" ", gap) + right
	return ansi.Truncate(result, available, "")
}

func (m Model) renderSessionList() string {
	var sb strings.Builder

	// Show filter input if in filter mode or filter is active
	if m.filterMode || m.isFiltered() {
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			PaddingLeft(1)
		if m.filterMode {
			sb.WriteString(filterStyle.Render("/" + m.filterInput.Value() + "▎") + "\n")
		} else {
			sb.WriteString(filterStyle.Render("/" + m.filterQuery) + "\n")
		}
	}

	sessions := m.filteredSessions()
	if len(sessions) == 0 {
		if m.isFiltered() {
			sb.WriteString(styleSessionMeta.Render("no matches"))
		} else {
			sb.WriteString(styleSessionMeta.Render("no claude sessions\nfound in tmux"))
		}
		return sb.String()
	}

	// Get the actual indices for selected highlighting
	indices := m.filtered
	if indices == nil {
		indices = make([]int, len(m.sessions))
		for i := range m.sessions {
			indices[i] = i
		}
	}

	for i, s := range sessions {
		actualIdx := indices[i]
		sb.WriteString(m.renderSessionItem(actualIdx, s) + "\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func (m Model) renderSessionItem(i int, s session.Session) string {
	icon := stateIcon(s.State.String())
	name := filepath.Base(s.ProjectPath)
	if name == "." || name == "" {
		name = s.TmuxPane
	}

	// Add pin indicator
	pinIndicator := ""
	if _, isPinned := m.pinned[s.Key()]; isPinned {
		pinIndicator = "📌 "
	}

	nameStyle := styleSessionItem
	if i == m.selected {
		nameStyle = styleSessionItemSelected
	}

	nameLine := nameStyle.
		Width(sessionPaneWidth - 1).
		Render(pinIndicator + icon + " " + name)

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
	if m.filterMode {
		return styleHelpInsert.Width(m.width).Render("FILTER  [enter] apply  [esc] clear")
	}
	parts := []string{
		"[j/k] nav",
		"[J/K] move",
		"[p] pin",
		"[/] filter",
		"[i] insert",
		"[t] jump",
		"[d] diff",
		"[n] new",
		"[x] kill",
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
