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
	// All inner styles carry the purple header background so the full bar is
	// solid — lipgloss won't fill the background of pre-rendered ANSI spans.
	hbg := colAccent

	left := lipgloss.NewStyle().Bold(true).Background(hbg).Foreground(lipgloss.Color("#FFFFFF")).Render("herd")
	sel := m.selectedSession()
	if sel != nil {
		sep := lipgloss.NewStyle().Background(hbg).Foreground(lipgloss.Color("#C4B5FD")).Render("  ·  ")
		proj := lipgloss.NewStyle().Background(hbg).Foreground(colGoldText).Render(filepath.Base(sel.ProjectPath))
		left += sep + proj
		if sel.GitBranch != "" {
			branch := lipgloss.NewStyle().Background(hbg).Foreground(lipgloss.Color("#C4B5FD")).Render("  [" + sel.GitBranch + "]")
			left += branch
		}
	}

	// Right: coloured stat pills — each pill keeps its state colour on purple bg.
	right := m.aggregateStats()

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + right
	return styleHeader.Width(m.width).Render(content)
}

// aggregateStats returns a coloured summary of session states.
func (m Model) aggregateStats() string {
	if len(m.sessions) == 0 {
		return ""
	}

	counts := make(map[session.State]int)
	for _, s := range m.sessions {
		counts[s.State]++
	}

	pill := func(color lipgloss.Color, text string) string {
		return lipgloss.NewStyle().
			Background(colAccent).
			Foreground(color).
			Bold(true).
			Render(text)
	}

	var parts []string
	if n := counts[session.StateWorking]; n > 0 {
		parts = append(parts, pill(colGreen, fmt.Sprintf("● %d working", n)))
	}
	if n := counts[session.StateWaiting]; n > 0 {
		parts = append(parts, pill(colBlue, fmt.Sprintf("◉ %d waiting", n)))
	}
	if n := counts[session.StatePlanReady]; n > 0 {
		parts = append(parts, pill(colAmber, fmt.Sprintf("◆ %d plan", n)))
	}
	if n := counts[session.StateNotifying]; n > 0 {
		parts = append(parts, pill(colPurple, fmt.Sprintf("◈ %d notify", n)))
	}
	if n := counts[session.StateIdle]; n > 0 {
		parts = append(parts, pill(colCyan, fmt.Sprintf("○ %d idle", n)))
	}
	if len(parts) == 0 {
		return lipgloss.NewStyle().Background(colAccent).Foreground(colSubtext).Render(fmt.Sprintf("%d sessions", len(m.sessions)))
	}
	sep := lipgloss.NewStyle().Background(colAccent).Foreground(lipgloss.Color("#C4B5FD")).Render("  ·  ")
	return strings.Join(parts, sep)
}

func (m Model) renderOutputHeader() string {
	sel := m.selectedSession()
	if sel == nil {
		return "no session selected"
	}

	icon := stateIcon(sel.State.String())
	label := stateLabel(sel.State.String(), sel.CurrentTool)

	paneStyle := lipgloss.NewStyle().Foreground(colSubtle)
	left := " " + icon + " " + label + "  " + paneStyle.Render(sel.TmuxPane)

	right := ""
	if !m.viewport.AtBottom() {
		pct := int(m.viewport.ScrollPercent() * 100)
		right = lipgloss.NewStyle().Foreground(colSubtle).Render(fmt.Sprintf("%d%%", pct))
	}

	available := m.width - sessionPaneWidth - 1
	gap := available - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	result := left + strings.Repeat(" ", gap) + right
	return ansi.Truncate(result, available, "")
}

func (m Model) renderSessionList() string {
	var sb strings.Builder

	// Filter mode: flat list with no tree decoration.
	if m.mode == ModeFilter || m.isFiltered() {
		if m.mode == ModeFilter {
			sb.WriteString(styleFilter.Render("/" + m.filterInput.Value() + "▎") + "\n")
		} else {
			sb.WriteString(styleFilter.Render("/" + m.filterQuery) + "\n")
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
			sb.WriteString(m.renderSessionItem(indices[i], s, "", false, false) + "\n")
		}
		return strings.TrimSuffix(sb.String(), "\n")
	}

	// Tree view.
	items := m.viewItems()
	if len(items) == 0 {
		sb.WriteString(styleSessionMeta.Render("no claude sessions\nfound in tmux"))
		return sb.String()
	}

	// Pre-compute which group each non-header item is in and whether it's the
	// last child, so we can pick the right connector.
	// lastInGroup[groupKey] = index of last child item in items slice.
	lastInGroup := make(map[string]int)
	for idx, item := range items {
		if !item.isHeader && item.groupKey != "" {
			lastInGroup[item.groupKey] = idx
		}
	}

	for idx, item := range items {
		if item.isHeader {
			isSelected := m.cursorOnGroup == item.groupKey
			sb.WriteString(m.renderGroupHeader(item, isSelected) + "\n")
		} else {
			s := m.sessions[item.sessionIdx]
			inGroup := item.groupKey != ""
			isLast := inGroup && lastInGroup[item.groupKey] == idx
			sb.WriteString(m.renderSessionItem(item.sessionIdx, s, item.groupKey, inGroup, isLast) + "\n")
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func (m Model) renderSessionItem(i int, s session.Session, groupKey string, inGroup, isLastChild bool) string {
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

	selected := i == m.selected

	// Tree connectors (only for grouped sessions).
	// Each connector + space = 2 chars, keeping content width = sessionPaneWidth-3.
	var connector, metaPrefix string
	if inGroup {
		connStyle := lipgloss.NewStyle().Foreground(colSubtle)
		if isLastChild {
			connector = connStyle.Render("└─") + " "
			metaPrefix = "   " // blank continuation column
		} else {
			connector = connStyle.Render("├─") + " "
			metaPrefix = connStyle.Render("│") + "  "
		}
	}

	// Pin indicator only for ungrouped sessions (groups show it on header).
	pinIndicator := ""
	if groupKey == "" {
		if _, isPinned := m.pinned[s.Key()]; isPinned {
			pinIndicator = "📌 "
		}
	}

	innerW := sessionPaneWidth - 1 - lipgloss.Width(connector)
	if innerW < 4 {
		innerW = 4
	}

	var nameStyle, metaStyle lipgloss.Style
	if selected {
		nameStyle = styleSessionItemSelected.Width(innerW)
		metaStyle = styleSessionMeta.Background(colGoldDim).Foreground(colGoldText).Width(innerW)
	} else {
		bg := stateBg(s.State.String())
		if inGroup {
			bg = colGroupedBg
		}
		nameStyle = styleSessionItem.Background(bg).Width(innerW)
		metaStyle = styleSessionMeta.Background(bg).Width(innerW)
	}

	nameLine := connector + nameStyle.Render(pinIndicator+icon+" "+name)
	metaLine := metaPrefix + metaStyle.Render(sessionMeta(s))

	return nameLine + "\n" + metaLine
}

func (m Model) renderGroupHeader(item viewItem, selected bool) string {
	collapsed := m.collapsedGroups[item.groupKey]
	connStyle := lipgloss.NewStyle().Foreground(colSubtle)

	var arrow string
	if collapsed {
		arrow = connStyle.Render("▶") + " "
	} else {
		arrow = connStyle.Render("▼") + " "
	}

	dot := stateIcon(item.aggState.String())
	pinIndicator := ""
	if m.isGroupPinned(item.groupKey) {
		pinIndicator = "📌 "
	}

	countStr := lipgloss.NewStyle().Foreground(colSubtle).Render(fmt.Sprintf("(%d)", item.count))
	label := pinIndicator + item.groupName + " " + countStr + "  " + dot

	innerW := sessionPaneWidth - 1 - lipgloss.Width(arrow)
	if innerW < 4 {
		innerW = 4
	}

	var style lipgloss.Style
	if selected {
		style = styleGroupHeaderSelected.Width(innerW)
	} else {
		style = styleGroupHeader.Width(innerW)
	}

	return arrow + style.Render(label)
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
	var sb strings.Builder
	sb.WriteString(styleOverlayTitle.Width(m.width).Render("Rename Session") + "\n\n")
	sb.WriteString(styleOverlayInput.Render(m.renameInput.View()) + "\n\n")
	sb.WriteString(styleOverlayHelp.Render("[enter] save  [esc] cancel  (empty to clear name)"))
	return sb.String()
}

func (m Model) renderGroupSetOverlay() string {
	var sb strings.Builder
	sb.WriteString(styleOverlayTitle.Width(m.width).Render("Set Group") + "\n\n")
	sb.WriteString(styleOverlayInput.Render(m.groupSetInput.View()) + "\n\n")
	sb.WriteString(styleOverlayHelp.Render("[enter] save  [esc] cancel  (empty to use auto-detected group)"))
	return sb.String()
}

func (m Model) renderHelp() string {
	if m.insertMode {
		return styleHelpInsert.Width(m.width).Render("  INSERT  [ctrl+h] exit")
	}
	if m.mode == ModeFilter {
		return styleHelpFilter.Width(m.width).Render("  FILTER  [enter] apply  [esc] clear")
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
