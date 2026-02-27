package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/lipgloss"

	"github.com/shnupta/herd/internal/names"
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
	if m.mode == ModeReview && m.reviewModel != nil {
		return m.reviewModel.View()
	}

	// If in worktree mode, show the worktree panel
	if m.mode == ModeWorktree && m.worktreeModel != nil {
		return m.worktreeModel.View()
	}

	// If in picker mode, show the project picker
	if m.mode == ModePicker && m.pickerModel != nil {
		return m.pickerModel.View()
	}

	// If in rename mode, show the rename overlay
	if m.mode == ModeRename {
		return m.renderRenameOverlay()
	}

	// If in group-set mode, show the group overlay
	if m.mode == ModeGroupSet {
		return m.renderGroupSetOverlay()
	}

	// No sessions — show landing page with the normal header/help chrome.
	if len(m.sessions) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderHeader(),
			m.renderLandingPage(),
			m.renderHelp(),
		)
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

	// Show filter input if in filter mode or filter is active.
	// When filtering, fall back to a flat (ungrouped) list for simplicity.
	if m.mode == ModeFilter || m.isFiltered() {
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			PaddingLeft(1)
		if m.mode == ModeFilter {
			sb.WriteString(filterStyle.Render("/" + m.filterInput.Value() + "▎") + "\n")
		} else {
			sb.WriteString(filterStyle.Render("/" + m.filterQuery) + "\n")
		}

		sessions := m.filteredSessions()
		if len(sessions) == 0 {
			sb.WriteString(styleSessionMeta.Render("no matches"))
			return sb.String()
		}

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

	// Grouped view.
	items := m.viewItems()
	if len(items) == 0 {
		sb.WriteString(styleSessionMeta.Render("no claude sessions\nfound in tmux"))
		return sb.String()
	}

	for _, item := range items {
		if item.isHeader {
			isSelected := m.cursorOnGroup == item.groupKey
			sb.WriteString(m.renderGroupHeader(item, isSelected) + "\n")
		} else {
			sb.WriteString(m.renderSessionItem(item.sessionIdx, m.sessions[item.sessionIdx]) + "\n")
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func (m Model) renderSessionItem(i int, s session.Session) string {
	icon := stateIcon(s.State.String())
	name := names.Get(s.Key())
	if name == "" {
		if agentName := m.teamsStore.MemberNameForSession(s.TmuxPane, s.ID); agentName != "" {
			name = "@" + agentName
		}
	}
	if name == "" {
		name = filepath.Base(s.ProjectPath)
		if name == "." || name == "" {
			name = s.TmuxPane
		}
	}

	// Only show pin on ungrouped sessions; grouped sessions show it on the header.
	gKey, _ := m.groupKeyAndName(s)
	pinIndicator := ""
	if gKey == "" {
		if _, isPinned := m.pinned[s.Key()]; isPinned {
			pinIndicator = "📌 "
		}
	}

	nameStyle := styleSessionItem
	metaStyle := styleSessionMeta
	if i == m.selected {
		nameStyle = styleSessionItemSelected
		metaStyle = styleSessionMeta.
			Background(colSelected).
			Foreground(lipgloss.Color("#FDE68A"))
	} else if gKey != "" {
		nameStyle = styleSessionItem.Background(colGroupedBg)
		metaStyle = styleSessionMeta.Background(colGroupedBg)
	}

	nameLine := nameStyle.
		Width(sessionPaneWidth - 1).
		Render(pinIndicator + icon + " " + name)

	// Sub-line: tool name or idle duration
	meta := sessionMeta(s)
	metaLine := metaStyle.
		Width(sessionPaneWidth - 1).
		Render(meta)

	return nameLine + "\n" + metaLine
}

func (m Model) renderGroupHeader(item viewItem, selected bool) string {
	arrow := "▼"
	if m.collapsedGroups[item.groupKey] {
		arrow = "▶"
	}
	dot := stateIcon(item.aggState.String())
	pinIndicator := ""
	if m.isGroupPinned(item.groupKey) {
		pinIndicator = "📌 "
	}
	label := fmt.Sprintf("%s%s %s (%d)  %s", pinIndicator, arrow, item.groupName, item.count, dot)

	style := styleGroupHeader
	if selected {
		style = styleGroupHeaderSelected
	}
	return style.Width(sessionPaneWidth - 1).Render(label)
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


func (m Model) renderLandingPage() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colAccent).
		Bold(true)

	subtextStyle := lipgloss.NewStyle().
		Foreground(colSubtext)

	hintStyle := lipgloss.NewStyle().
		Foreground(colText)

	body := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("herd"),
		"",
		subtextStyle.Render("no Claude sessions found in tmux"),
		"",
		hintStyle.Render("open Claude Code in a tmux pane to get started"),
	)

	page := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - 2). // total - header(1) - help(1)
		Align(lipgloss.Center, lipgloss.Center).
		Render(body)

	return page
}

func (m Model) renderRenameOverlay() string {
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1)
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		PaddingLeft(1)

	var sb strings.Builder
	sb.WriteString(titleStyle.Width(m.width).Render("Rename Session") + "\n\n")
	sb.WriteString(inputStyle.Render(m.renameInput.View()) + "\n\n")
	sb.WriteString(helpStyle.Render("[enter] save  [esc] cancel  (empty to clear name)"))
	return sb.String()
}

func (m Model) renderGroupSetOverlay() string {
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1)
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		PaddingLeft(1)

	var sb strings.Builder
	sb.WriteString(titleStyle.Width(m.width).Render("Set Group") + "\n\n")
	sb.WriteString(inputStyle.Render(m.groupSetInput.View()) + "\n\n")
	sb.WriteString(helpStyle.Render("[enter] save  [esc] cancel  (empty to use auto-detected group)"))
	return sb.String()
}

func (m Model) renderHelp() string {
	if m.insertMode {
		return styleHelpInsert.Width(m.width).Render("INSERT  [ctrl+h] exit")
	}
	if m.mode == ModeFilter {
		return styleHelpInsert.Width(m.width).Render("FILTER  [enter] apply  [esc] clear")
	}
	parts := []string{
		"[j/k] nav",
		"[J/K] move",
		"[p] pin",
		"[e] rename",
		"[space] collapse",
		"[g] group",
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
