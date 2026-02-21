package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shnupta/herd/internal/diff"
	"github.com/shnupta/herd/internal/hook"
	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/state"
	"github.com/shnupta/herd/internal/tmux"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// If in review mode, delegate only relevant messages to review model
	// Other messages (ticks, refresh, etc.) continue to main handler
	if m.reviewMode && m.reviewModel != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg, tea.MouseMsg:
			updated, cmd := m.reviewModel.Update(msg)
			reviewModel := updated.(ReviewModel)
			m.reviewModel = &reviewModel

			if reviewModel.Submitted() {
				// Send feedback to the agent via stdin
				if sel := m.selectedSession(); sel != nil && reviewModel.FeedbackText() != "" {
					_ = tmux.SendKeys(sel.TmuxPane, reviewModel.FeedbackText())
				}
				m.reviewMode = false
				m.reviewModel = nil
				m.lastCapture = "" // Force viewport refresh
				// Restart all polling loops
				if sel := m.selectedSession(); sel != nil {
					return m, tea.Batch(tickCapture(), tickSessionRefresh(), fetchCapture(sel.TmuxPane))
				}
				return m, tea.Batch(tickCapture(), tickSessionRefresh())
			} else if reviewModel.Cancelled() {
				m.reviewMode = false
				m.reviewModel = nil
				m.lastCapture = "" // Force viewport refresh
				// Restart all polling loops
				if sel := m.selectedSession(); sel != nil {
					return m, tea.Batch(tickCapture(), tickSessionRefresh(), fetchCapture(sel.TmuxPane))
				}
				return m, tea.Batch(tickCapture(), tickSessionRefresh())
			}

			return m, cmd
		}
		// Other messages fall through to main handler
	}

	// If in picker mode, delegate only relevant messages to picker model
	// Other messages (ticks, refresh, etc.) continue to main handler
	if m.pickerMode && m.pickerModel != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg:
			updated, cmd := m.pickerModel.Update(msg)
			pickerModel := updated.(PickerModel)
			m.pickerModel = &pickerModel

			if pickerModel.ChosenPath() != "" {
				// Launch new session and remember the pane ID for selection
				if paneID, err := LaunchSession(pickerModel.ChosenPath()); err != nil {
					m.err = err
				} else {
					m.pendingSelectPane = paneID
					m.pendingQuickRetried = false
				}
				m.pickerMode = false
				m.pickerModel = nil
				m.lastCapture = "" // Force viewport refresh
				// Refresh session list and restart capture polling
				return m, tea.Batch(discoverSessions(), tickCapture(), tickSessionRefresh())
			} else if pickerModel.Cancelled() {
				m.pickerMode = false
				m.pickerModel = nil
				m.lastCapture = "" // Force viewport refresh
				// Restart capture polling
				if sel := m.selectedSession(); sel != nil {
					return m, tea.Batch(tickCapture(), tickSessionRefresh(), fetchCapture(sel.TmuxPane))
				}
				return m, tea.Batch(tickCapture(), tickSessionRefresh())
			}

			return m, cmd
		}
		// Other messages fall through to main handler
	}

	// If in filter mode, handle filter input
	if m.filterMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				// Clear filter and exit filter mode
				m.filterMode = false
				m.filterQuery = ""
				m.filterInput.Reset()
				m.filtered = nil
				return m, nil
			case "enter":
				// Exit filter mode but keep filter active
				m.filterMode = false
				m.filterInput.Blur()
				return m, nil
			case "backspace":
				if m.filterInput.Value() == "" {
					// Exit filter mode if backspacing on empty
					m.filterMode = false
					m.filterQuery = ""
					m.filtered = nil
					return m, nil
				}
			}
		}

		// Update filter input
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterQuery = m.filterInput.Value()
		m.updateFilter()
		return m, cmd
	}

	// If in group-set mode, handle group name input
	if m.groupSetMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.groupSetMode = false
				m.groupSetInput.Reset()
				m.groupSetKey = ""
				return m, nil
			case "enter":
				groupName := strings.TrimSpace(m.groupSetInput.Value())
				_ = m.groupsStore.Set(m.groupSetKey, groupName)
				m.groupSetMode = false
				m.groupSetInput.Reset()
				m.groupSetKey = ""
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.groupSetInput, cmd = m.groupSetInput.Update(msg)
		return m, cmd
	}

	// If in rename mode, handle rename input
	if m.renameMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.renameMode = false
				m.renameInput.Reset()
				m.renameKey = ""
				return m, nil
			case "enter":
				label := strings.TrimSpace(m.renameInput.Value())
				if label == "" {
					_ = m.namesStore.Delete(m.renameKey)
				} else {
					_ = m.namesStore.Set(m.renameKey, label)
				}
				m.renameMode = false
				m.renameInput.Reset()
				m.renameKey = ""
				return m, nil
			}
		}

		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {

	// ── Terminal resize ────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.recalcLayout()
		if !m.ready {
			m.ready = true
		}
		// Resize the observed pane to match the new viewport dimensions so
		// that Claude formats its output to fit what we can display.
		if sel := m.selectedSession(); sel != nil {
			cmds = append(cmds, resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
		}

	// ── Initial session discovery ──────────────────────────────────────────
	case sessionsDiscoveredMsg:
		// Save selected pane BEFORE replacing sessions list
		var selectedPane string
		if m.selected < len(m.sessions) {
			selectedPane = m.sessions[m.selected].TmuxPane
		}

		existing := make(map[string]session.Session)
		for _, s := range m.sessions {
			existing[s.TmuxPane] = s
		}
		var merged []session.Session
		for _, s := range msg {
			if prev, ok := existing[s.TmuxPane]; ok {
				s.ID = prev.ID
				s.State = prev.State
				s.CurrentTool = prev.CurrentTool
			}
			merged = append(merged, s)
		}
		m.sessions = merged
		// Cleanup stale entries from sidebar state
		m.cleanupSidebarState()
		if m.sidebarDirty {
			m.saveSidebarState()
		}
		// Sort sessions with pinned at top and apply saved order
		m.sortSessions()

		// Restore selection to the previously selected pane
		if selectedPane != "" {
			for i, s := range m.sessions {
				if s.TmuxPane == selectedPane {
					m.selected = i
					break
				}
			}
		}
		if m.selected >= len(m.sessions) {
			m.selected = maxInt(0, len(m.sessions)-1)
		}
		// If we have a pending pane selection, find and select it.
		// We keep trying across refreshes because the new pane may still be
		// initializing (shell starting, claude not yet running) when the first
		// discovery fires.
		if m.pendingSelectPane != "" {
			for i, s := range m.sessions {
				if s.TmuxPane == m.pendingSelectPane {
					m.selected = i
					m.lastCapture = ""        // Force viewport refresh
					m.pendingGotoBottom = true // Jump to bottom of new session
					m.pendingSelectPane = ""   // Found — stop searching
					m.pendingQuickRetried = false
					break
				}
			}
			// Still waiting — fire one quick 500ms retry (Claude may still be
			// initialising). After that, let the normal 3s timer handle it.
			if m.pendingSelectPane != "" && !m.pendingQuickRetried {
				m.pendingQuickRetried = true
				cmds = append(cmds, pendingDiscoveryTick())
			}
		}
		if states, err := state.ReadAll(); err == nil {
			m = m.applyStates(states)
		}
		// Resize the newly selected pane (viewport dimensions are known once ready).
		if m.ready {
			if sel := m.selectedSession(); sel != nil {
				cmds = append(cmds, resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
			}
		}

	// ── Session list auto-refresh ──────────────────────────────────────────
	case sessionRefreshMsg:
		_ = m.teamsStore.Load() // pick up new/updated team configs
		cmds = append(cmds, discoverSessions(), tickSessionRefresh())

	// ── Capture-pane poll ──────────────────────────────────────────────────
	case tickMsg:
		cmds = append(cmds, tickCapture())
		if sel := m.selectedSession(); sel != nil {
			cmds = append(cmds, fetchCapture(sel.TmuxPane))
		}

	case captureMsg:
		if sel := m.selectedSession(); sel != nil && sel.TmuxPane == msg.paneID && msg.content != m.lastCapture {
			m.lastCapture = msg.content
			// After a session switch, always jump to the bottom of the new session's
			// output rather than inheriting the scroll position from the previous one.
			if m.pendingGotoBottom {
				m.atBottom = true
				m.pendingGotoBottom = false
			} else {
				m.atBottom = m.viewport.AtBottom()
			}

			m.viewport.SetContent(truncateLines(cleanCapture(msg.content), m.viewport.Width))
			if m.atBottom {
				m.viewport.GotoBottom()
			}
		}

	// ── Hook state update ──────────────────────────────────────────────────
	case stateUpdateMsg:
		m = m.applyStates([]state.SessionState{state.SessionState(msg)})
		cmds = append(cmds, waitForStateEvent(m.stateWatcher))

	// ── Spinner ────────────────────────────────────────────────────────────
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	// ── Error ──────────────────────────────────────────────────────────────
	case errMsg:
		m.err = msg.err

	// ── Keyboard ──────────────────────────────────────────────────────────
	case tea.KeyMsg:
		if m.insertMode {
			if msg.String() == "ctrl+h" {
				m.insertMode = false
			} else if sel := m.selectedSession(); sel != nil {
				if err := forwardKey(sel.TmuxPane, msg); err != nil {
					m.err = err
				} else {
					// Immediately re-fetch capture so typing feels responsive.
					cmds = append(cmds, fetchCapture(sel.TmuxPane))
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, keys.Quit):
			// Restore auto-sizing on all observed panes before quitting so
			// Claude sessions return to their natural terminal dimensions.
			for _, s := range m.sessions {
				_ = tmux.ResizePaneAuto(s.TmuxPane)
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			changed := m.moveUp()
			// Always fetch immediately so the viewport reflects the new session
			// without waiting for the next 100ms tick.
			if sel := m.selectedSession(); sel != nil {
				if changed {
					m.lastCapture = ""
					m.pendingGotoBottom = true
					cmds = append(cmds, resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
				}
				cmds = append(cmds, fetchCapture(sel.TmuxPane))
			}

		case key.Matches(msg, keys.Down):
			changed := m.moveDown()
			if sel := m.selectedSession(); sel != nil {
				if changed {
					m.lastCapture = ""
					m.pendingGotoBottom = true
					cmds = append(cmds, resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
				}
				cmds = append(cmds, fetchCapture(sel.TmuxPane))
			}

		case key.Matches(msg, keys.Jump):
			if sel := m.selectedSession(); sel != nil {
				if err := tmux.SwitchToPane(sel.TmuxPane); err != nil {
					m.err = err
				}
			}

		case key.Matches(msg, keys.Insert):
			m.insertMode = true

		case key.Matches(msg, keys.Refresh):
			cmds = append(cmds, discoverSessions())

		case key.Matches(msg, keys.Install):
			selfPath, _ := os.Executable()
			if err := hook.Install(selfPath); err != nil {
				m.err = err
			}

		case key.Matches(msg, keys.Kill):
			if sel := m.selectedSession(); sel != nil {
				if err := tmux.KillPane(sel.TmuxPane); err != nil {
					m.err = err
				} else {
					// Remove pin for killed session
					delete(m.pinned, sel.Key())
					m.sessions = append(m.sessions[:m.selected], m.sessions[m.selected+1:]...)
					if m.selected >= len(m.sessions) {
						m.selected = maxInt(0, len(m.sessions)-1)
					}
					m.lastCapture = ""
					m.pendingGotoBottom = true
					// Cleanup and save sidebar state
					m.cleanupSidebarState()
					m.saveSidebarState()
				}
			}

		case key.Matches(msg, keys.New):
			// Open project picker to create new session
			var existingPaths []string
			for _, s := range m.sessions {
				if s.ProjectPath != "" {
					existingPaths = append(existingPaths, s.ProjectPath)
				}
			}
			pickerModel := NewPickerModel(existingPaths)
			// Send initial size
			updatedModel, _ := pickerModel.Update(tea.WindowSizeMsg{
				Width:  m.width,
				Height: m.height,
			})
			pickerModel = updatedModel.(PickerModel)
			m.pickerModel = &pickerModel
			m.pickerMode = true

		case key.Matches(msg, keys.Worktree):
			// TODO: worktree panel

		case key.Matches(msg, keys.Review):
			// Open diff review for the selected session's project
			if sel := m.selectedSession(); sel != nil {
				gitRoot, err := diff.GetGitRoot(sel.ProjectPath)
				if err == nil {
					diffText, err := diff.GetGitDiff(gitRoot)
					if err == nil && diffText != "" {
						parsed, err := diff.Parse(diffText)
						if err == nil && !parsed.IsEmpty() {
							sessionID := sel.ID
							if sessionID == "" {
								sessionID = sel.TmuxPane
							}
							reviewModel := NewReviewModel(parsed, sessionID, gitRoot)
							// Send initial size
							updatedModel, _ := reviewModel.Update(tea.WindowSizeMsg{
								Width:  m.width,
								Height: m.height,
							})
							reviewModel = updatedModel.(ReviewModel)
							m.reviewModel = &reviewModel
							m.reviewMode = true
						}
					}
				}
			}

		case key.Matches(msg, keys.Filter):
			// Enter filter mode
			m.filterMode = true
			m.filterInput.Focus()

		case key.Matches(msg, keys.Rename):
			// Open rename overlay for the selected session
			if sel := m.selectedSession(); sel != nil {
				m.renameKey = sel.Key()
				m.renameInput.SetValue(m.namesStore.Get(m.renameKey))
				m.renameInput.Focus()
				m.renameMode = true
			}

		case key.Matches(msg, keys.ToggleGroup):
			m.toggleGroupAtCursor()

		case key.Matches(msg, keys.SetGroup):
			// Open group-set overlay for the selected session
			if m.cursorOnGroup == "" {
				if sel := m.selectedSession(); sel != nil {
					m.groupSetKey = sel.Key()
					current := m.groupsStore.Get(m.groupSetKey)
					m.groupSetInput.SetValue(current)
					m.groupSetInput.Focus()
					m.groupSetMode = true
				}
			}

		case key.Matches(msg, keys.Pin):
			// Toggle pin on selected session (keyed by session key for uniqueness)
			if sel := m.selectedSession(); sel != nil {
				if _, isPinned := m.pinned[sel.Key()]; isPinned {
					delete(m.pinned, sel.Key())
				} else {
					m.pinCounter++
					m.pinned[sel.Key()] = m.pinCounter
				}
				m.sortSessions()
				m.saveSidebarState()
			}

		case key.Matches(msg, keys.MoveUp):
			// Move selected session up in the list
			if m.selected > 0 {
				m.sessions[m.selected], m.sessions[m.selected-1] = m.sessions[m.selected-1], m.sessions[m.selected]
				// If swapping pinned sessions, swap their pin order too
				key1, key2 := m.sessions[m.selected].Key(), m.sessions[m.selected-1].Key()
				if order1, ok1 := m.pinned[key1]; ok1 {
					if order2, ok2 := m.pinned[key2]; ok2 {
						m.pinned[key1], m.pinned[key2] = order2, order1
					}
				}
				m.selected--
				m.lastCapture = ""
				m.pendingGotoBottom = true
				m.saveSidebarState()
			}

		case key.Matches(msg, keys.MoveDown):
			// Move selected session down in the list
			if m.selected < len(m.sessions)-1 {
				m.sessions[m.selected], m.sessions[m.selected+1] = m.sessions[m.selected+1], m.sessions[m.selected]
				// If swapping pinned sessions, swap their pin order too
				key1, key2 := m.sessions[m.selected].Key(), m.sessions[m.selected+1].Key()
				if order1, ok1 := m.pinned[key1]; ok1 {
					if order2, ok2 := m.pinned[key2]; ok2 {
						m.pinned[key1], m.pinned[key2] = order2, order1
					}
				}
				m.selected++
				m.lastCapture = ""
				m.pendingGotoBottom = true
				m.saveSidebarState()
			}
		}

	// ── Mouse ──────────────────────────────────────────────────────────────
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.ScrollUp(3)
		case tea.MouseButtonWheelDown:
			m.viewport.ScrollDown(3)
		case tea.MouseButtonLeft:
			if msg.X < sessionPaneWidth {
				if idx := m.sessionIndexAtY(msg.Y); idx >= 0 && idx < len(m.sessions) {
					if m.selected != idx {
						m.selected = idx
						m.lastCapture = ""
						m.pendingGotoBottom = true
						if sel := m.selectedSession(); sel != nil {
							cmds = append(cmds, resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
						}
					}
				}
			}
		}
	}

	// Forward scroll and other events to viewport when not in insert mode.
	// Session-navigation keys (up/k, down/j) must not reach the viewport —
	// the viewport has its own bindings for those keys and would scroll the
	// content in addition to switching sessions, causing a visible flicker.
	if !m.insertMode {
		if keyMsg, isKey := msg.(tea.KeyMsg); !isKey || (!key.Matches(keyMsg, keys.Up) && !key.Matches(keyMsg, keys.Down) && !key.Matches(keyMsg, keys.ToggleGroup)) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// ── Key forwarding ─────────────────────────────────────────────────────────

// forwardKey sends a single key event to the given tmux pane.
// ctrl+h is the exit-insert-mode key and is never forwarded.
func forwardKey(paneID string, msg tea.KeyMsg) error {
	switch msg.String() {
	case "ctrl+h":
		return nil // exit key — handled by caller
	case "enter":
		return tmux.SendKeyName(paneID, "Enter")
	case "backspace":
		return tmux.SendKeyName(paneID, "BSpace")
	case "delete":
		return tmux.SendKeyName(paneID, "DC")
	case "tab", "ctrl+i":
		return tmux.SendKeyName(paneID, "Tab")
	case "shift+tab":
		return tmux.SendKeyName(paneID, "BTab")
	case "up":
		return tmux.SendKeyName(paneID, "Up")
	case "down":
		return tmux.SendKeyName(paneID, "Down")
	case "left":
		return tmux.SendKeyName(paneID, "Left")
	case "right":
		return tmux.SendKeyName(paneID, "Right")
	case "home":
		return tmux.SendKeyName(paneID, "Home")
	case "end":
		return tmux.SendKeyName(paneID, "End")
	case "pgup":
		return tmux.SendKeyName(paneID, "PPage")
	case "pgdown":
		return tmux.SendKeyName(paneID, "NPage")
	case "esc":
		return tmux.SendKeyName(paneID, "Escape")
	case "ctrl+a":
		return tmux.SendKeyName(paneID, "C-a")
	case "ctrl+b":
		return tmux.SendKeyName(paneID, "C-b")
	case "ctrl+c":
		return tmux.SendKeyName(paneID, "C-c")
	case "ctrl+d":
		return tmux.SendKeyName(paneID, "C-d")
	case "ctrl+e":
		return tmux.SendKeyName(paneID, "C-e")
	case "ctrl+f":
		return tmux.SendKeyName(paneID, "C-f")
	case "ctrl+g":
		return tmux.SendKeyName(paneID, "C-g")
	// ctrl+h: exit key, not forwarded
	case "ctrl+j":
		return tmux.SendKeyName(paneID, "C-j")
	case "ctrl+k":
		return tmux.SendKeyName(paneID, "C-k")
	case "ctrl+l":
		return tmux.SendKeyName(paneID, "C-l")
	case "ctrl+m":
		return tmux.SendKeyName(paneID, "Enter")
	case "ctrl+n":
		return tmux.SendKeyName(paneID, "C-n")
	case "ctrl+o":
		return tmux.SendKeyName(paneID, "C-o")
	case "ctrl+p":
		return tmux.SendKeyName(paneID, "C-p")
	case "ctrl+q":
		return tmux.SendKeyName(paneID, "C-q")
	case "ctrl+r":
		return tmux.SendKeyName(paneID, "C-r")
	case "ctrl+s":
		return tmux.SendKeyName(paneID, "C-s")
	case "ctrl+t":
		return tmux.SendKeyName(paneID, "C-t")
	case "ctrl+u":
		return tmux.SendKeyName(paneID, "C-u")
	case "ctrl+v":
		return tmux.SendKeyName(paneID, "C-v")
	case "ctrl+w":
		return tmux.SendKeyName(paneID, "C-w")
	case "ctrl+x":
		return tmux.SendKeyName(paneID, "C-x")
	case "ctrl+y":
		return tmux.SendKeyName(paneID, "C-y")
	case "ctrl+z":
		return tmux.SendKeyName(paneID, "C-z")
	}

	// Printable runes — send literally.
	if len(msg.Runes) > 0 {
		return tmux.SendLiteral(paneID, string(msg.Runes))
	}

	return nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

// resizePaneToViewport resizes the tmux window containing paneID to width×height
// so that the observed session formats its output to fit the herd viewport.
// This is a fire-and-forget async command; errors are silently ignored.
func resizePaneToViewport(paneID string, width, height int) tea.Cmd {
	if paneID == "" || width <= 0 || height <= 0 {
		return nil
	}
	return func() tea.Msg {
		_ = tmux.ResizeWindow(paneID, width, height)
		return nil
	}
}

func fetchCapture(paneID string) tea.Cmd {
	return func() tea.Msg {
		content, err := tmux.CapturePane(paneID, 2000)
		if err != nil {
			return nil
		}
		return captureMsg{paneID: paneID, content: content}
	}
}

func (m Model) applyStates(states []state.SessionState) Model {
	byPane := make(map[string]state.SessionState)
	byID := make(map[string]state.SessionState)
	for _, s := range states {
		if s.TmuxPane != "" {
			byPane[s.TmuxPane] = s
		}
		if s.SessionID != "" {
			byID[s.SessionID] = s
		}
	}
	for i, sess := range m.sessions {
		var st state.SessionState
		var found bool
		if sess.ID != "" {
			st, found = byID[sess.ID]
		}
		if !found {
			st, found = byPane[sess.TmuxPane]
		}
		if !found {
			continue
		}
		m.sessions[i].ID = st.SessionID
		m.sessions[i].State = parseState(st.State)
		m.sessions[i].CurrentTool = st.CurrentTool
		m.sessions[i].UpdatedAt = st.UpdatedAt
	}
	return m
}

func parseState(s string) session.State {
	switch s {
	case "working":
		return session.StateWorking
	case "waiting":
		return session.StateWaiting
	case "plan_ready":
		return session.StatePlanReady
	case "notifying":
		return session.StateNotifying
	case "idle":
		return session.StateIdle
	default:
		return session.StateUnknown
	}
}

func (m Model) recalcLayout() Model {
	// outputHeaderH is 2 because styleOutputHeader has BorderBottom which adds a row.
	const headerH, outputHeaderH, helpH = 1, 2, 1

	vpWidth := m.width - sessionPaneWidth - 1
	vpHeight := m.height - headerH - outputHeaderH - helpH

	if vpWidth < 10 {
		vpWidth = 10
	}
	if vpHeight < 3 {
		vpHeight = 3
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
	return m
}

func (m Model) sessionIndexAtY(y int) int {
	// Header is row 0; each session takes 2 rows (name + meta line).
	contentY := y - 1
	if contentY < 0 {
		return -1
	}
	return contentY / 2
}

// truncateLines clips any line wider than maxWidth to prevent frame overflow.
// Uses ANSI-aware truncation so escape codes don't corrupt the layout.
func truncateLines(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, maxWidth, "")
	}
	return strings.Join(lines, "\n")
}

func cleanCapture(s string) string {
	lines := strings.Split(s, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[:end], "\n")
}

// updateFilter filters the session list based on the current filter query.
func (m *Model) updateFilter() {
	if m.filterQuery == "" {
		m.filtered = nil
		return
	}

	query := strings.ToLower(m.filterQuery)
	m.filtered = nil

	for i, s := range m.sessions {
		// Match against project path, git branch, pane ID, and session ID
		searchable := strings.ToLower(s.ProjectPath + " " + s.GitBranch + " " + s.TmuxPane + " " + s.ID)
		if strings.Contains(searchable, query) {
			m.filtered = append(m.filtered, i)
		}
	}

	// Adjust selection to stay within filtered results
	if len(m.filtered) > 0 {
		// Check if current selection is in filtered list
		found := false
		for _, idx := range m.filtered {
			if idx == m.selected {
				found = true
				break
			}
		}
		if !found {
			m.selected = m.filtered[0]
		}
	}
}

// filteredSessions returns the sessions that match the current filter.
// If no filter is active, returns all sessions.
func (m *Model) filteredSessions() []session.Session {
	if m.filtered == nil || len(m.filtered) == 0 {
		if m.filterQuery == "" {
			return m.sessions
		}
		return nil
	}
	
	result := make([]session.Session, len(m.filtered))
	for i, idx := range m.filtered {
		result[i] = m.sessions[idx]
	}
	return result
}

// isFiltered returns true if a filter is active.
func (m *Model) isFiltered() bool {
	return m.filterQuery != ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

