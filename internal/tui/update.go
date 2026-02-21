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
	"github.com/shnupta/herd/internal/git"
	"github.com/shnupta/herd/internal/groups"
	"github.com/shnupta/herd/internal/hook"
	"github.com/shnupta/herd/internal/names"
	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/state"
	"github.com/shnupta/herd/internal/tmux"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeReview:
		// Review mode only intercepts key/window/mouse messages;
		// other messages (ticks, refresh, etc.) fall through to the main handler.
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg, tea.MouseMsg:
			return m.updateReviewMode(msg)
		}
	case ModePicker:
		// Picker mode only intercepts key/window messages;
		// other messages fall through to the main handler.
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg:
			return m.updatePickerMode(msg)
		}
	case ModeFilter:
		return m.updateFilterMode(msg)
	case ModeRename:
		return m.updateRenameMode(msg)
	case ModeGroupSet:
		return m.updateGroupSetMode(msg)
	case ModeWorktree:
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg:
			return m.updateWorktreeMode(msg)
		}
	}

	return m.updateNormal(msg)
}

// ── Per-mode update handlers ───────────────────────────────────────────────

func (m Model) updateReviewMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.reviewModel == nil {
		return m.updateNormal(msg)
	}

	updated, cmd := m.reviewModel.Update(msg)
	reviewModel := updated.(ReviewModel)
	m.reviewModel = &reviewModel

	if reviewModel.Submitted() {
		if sel := m.selectedSession(); sel != nil && reviewModel.FeedbackText() != "" {
			_ = m.tmuxClient.SendKeys(sel.TmuxPane, reviewModel.FeedbackText())
		}
		m.mode = ModeNormal
		m.reviewModel = nil
		m.lastCapture = ""
		m.forceViewportRefresh = true
		if sel := m.selectedSession(); sel != nil {
			return m, tea.Batch(tickCapture(), tickSessionRefresh(), m.fetchCapture(sel.TmuxPane))
		}
		return m, tea.Batch(tickCapture(), tickSessionRefresh())
	} else if reviewModel.Cancelled() {
		m.mode = ModeNormal
		m.reviewModel = nil
		m.lastCapture = ""
		m.forceViewportRefresh = true
		if sel := m.selectedSession(); sel != nil {
			return m, tea.Batch(tickCapture(), tickSessionRefresh(), m.fetchCapture(sel.TmuxPane))
		}
		return m, tea.Batch(tickCapture(), tickSessionRefresh())
	}

	return m, cmd
}

func (m Model) updatePickerMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pickerModel == nil {
		return m.updateNormal(msg)
	}

	updated, cmd := m.pickerModel.Update(msg)
	pickerModel := updated.(PickerModel)
	m.pickerModel = &pickerModel

	if pickerModel.ChosenPath() != "" {
		if paneID, err := LaunchSession(pickerModel.ChosenPath(), m.tmuxClient); err != nil {
			m.err = err
		} else {
			m.pendingSelectPane = paneID
			m.pendingQuickRetried = false
		}
		m.mode = ModeNormal
		m.pickerModel = nil
		m.lastCapture = ""
		m.forceViewportRefresh = true
		return m, tea.Batch(m.discoverSessions(), tickCapture(), tickSessionRefresh())
	} else if pickerModel.Cancelled() {
		m.mode = ModeNormal
		m.pickerModel = nil
		m.lastCapture = ""
		m.forceViewportRefresh = true
		if sel := m.selectedSession(); sel != nil {
			return m, tea.Batch(tickCapture(), tickSessionRefresh(), m.fetchCapture(sel.TmuxPane))
		}
		return m, tea.Batch(tickCapture(), tickSessionRefresh())
	}

	return m, cmd
}

func (m Model) updateWorktreeMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.worktreeModel == nil {
		return m.updateNormal(msg)
	}

	updated, cmd := m.worktreeModel.Update(msg)
	wm := updated.(WorktreeModel)
	m.worktreeModel = &wm

	if path := wm.ChosenPath(); path != "" {
		m.mode = ModeNormal
		m.worktreeModel = nil
		return m, m.openOrSwitchWorktree(path)
	}
	if createPath, branch, ok := wm.ShouldCreate(); ok {
		repoRoot := ""
		if sel := m.selectedSession(); sel != nil {
			repoRoot = sel.GitRoot
		}
		m.mode = ModeNormal
		m.worktreeModel = nil
		return m, createAndLaunchWorktree(m.tmuxClient, repoRoot, createPath, branch)
	}
	if wtPath, sessionPane, ok := wm.ShouldRemove(); ok {
		repoRoot := ""
		if sel := m.selectedSession(); sel != nil {
			repoRoot = sel.GitRoot
		}
		m.mode = ModeNormal
		m.worktreeModel = nil
		return m, removeWorktree(m.tmuxClient, repoRoot, wtPath, sessionPane)
	}
	if wm.Cancelled() {
		m.mode = ModeNormal
		m.worktreeModel = nil
		m.lastCapture = ""
		m.forceViewportRefresh = true
		if sel := m.selectedSession(); sel != nil {
			return m, tea.Batch(tickCapture(), tickSessionRefresh(), m.fetchCapture(sel.TmuxPane))
		}
		return m, tea.Batch(tickCapture(), tickSessionRefresh())
	}

	return m, cmd
}

func (m Model) updateFilterMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			m.filterQuery = ""
			m.filterInput.Reset()
			m.filtered = nil
			return m, nil
		case "enter":
			m.mode = ModeNormal
			m.filterInput.Blur()
			return m, nil
		case "backspace":
			if m.filterInput.Value() == "" {
				m.mode = ModeNormal
				m.filterQuery = ""
				m.filtered = nil
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterQuery = m.filterInput.Value()
	m.updateFilter()
	return m, cmd
}

func (m Model) updateRenameMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			m.renameInput.Reset()
			m.renameKey = ""
			return m, nil
		case "enter":
			label := strings.TrimSpace(m.renameInput.Value())
			if label == "" {
				_ = names.Delete(m.renameKey)
			} else {
				_ = names.Set(m.renameKey, label)
			}
			m.mode = ModeNormal
			m.renameInput.Reset()
			m.renameKey = ""
			m.itemsDirty = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) updateGroupSetMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			m.groupSetInput.Reset()
			m.groupSetKey = ""
			return m, nil
		case "enter":
			groupName := strings.TrimSpace(m.groupSetInput.Value())
			_ = groups.Set(m.groupSetKey, groupName)
			m.mode = ModeNormal
			m.groupSetInput.Reset()
			m.groupSetKey = ""
			m.itemsDirty = true
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.groupSetInput, cmd = m.groupSetInput.Update(msg)
	return m, cmd
}

// ── Normal mode ────────────────────────────────────────────────────────────

func (m Model) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Terminal resize ────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.recalcLayout()
		if !m.ready {
			m.ready = true
		}
		if sel := m.selectedSession(); sel != nil {
			cmds = append(cmds, m.resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
		}

	// ── Initial session discovery ──────────────────────────────────────────
	case sessionsDiscoveredMsg:
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
		m.itemsDirty = true
		// Apply persisted state files first so sessions have their IDs before
		// cleanupSidebarState() evaluates which keys are active. Without this,
		// session: keys in savedOrder get pruned on startup because sessions
		// haven't received IDs yet, destroying the persisted order.
		if states, err := state.ReadAll(); err == nil {
			m = m.applyStates(states)
		}
		m.cleanupSidebarState()
		if m.sidebarDirty {
			m.saveSidebarState()
		}
		m.sortSessions()

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
		if m.pendingSelectPane != "" {
			for i, s := range m.sessions {
				if s.TmuxPane == m.pendingSelectPane {
					m.selected = i
					m.lastCapture = ""
					m.forceViewportRefresh = true
					m.pendingGotoBottom = true
					m.pendingSelectPane = ""
					m.pendingQuickRetried = false
					break
				}
			}
			if m.pendingSelectPane != "" && !m.pendingQuickRetried {
				m.pendingQuickRetried = true
				cmds = append(cmds, m.pendingDiscoveryTick())
			}
		}
		if m.ready {
			if sel := m.selectedSession(); sel != nil {
				cmds = append(cmds, m.resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
			}
		}

	// ── Session list auto-refresh ──────────────────────────────────────────
	case sessionRefreshMsg:
		_ = m.teamsStore.Load() // pick up new/updated team configs
		cmds = append(cmds, m.discoverSessions(), tickSessionRefresh())

	// ── Capture-pane poll ──────────────────────────────────────────────────
	case tickMsg:
		cmds = append(cmds, tickCapture())
		if sel := m.selectedSession(); sel != nil {
			cmds = append(cmds, m.fetchCapture(sel.TmuxPane))
		}

	case captureMsg:
		if sel := m.selectedSession(); sel != nil && sel.TmuxPane == msg.paneID {
			contentChanged := msg.content != m.lastCapture
			if contentChanged || m.forceViewportRefresh {
				m.lastCapture = msg.content
				m.forceViewportRefresh = false
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

	// ── Worktree launched ──────────────────────────────────────────────────
	case worktreeLaunchedMsg:
		m.pendingSelectPane = string(msg)
		return m, tea.Batch(m.discoverSessions(), tickCapture(), tickSessionRefresh())

	// ── Worktree removed ───────────────────────────────────────────────────
	case worktreeRemovedMsg:
		if msg.sessionPane != "" {
			// Remove killed session from in-memory list.
			for i, s := range m.sessions {
				if s.TmuxPane == msg.sessionPane {
					delete(m.pinned, s.Key())
					m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
					if m.selected >= len(m.sessions) {
						m.selected = maxInt(0, len(m.sessions)-1)
					}
					m.lastCapture = ""
					m.pendingGotoBottom = true
					m.cleanupSidebarState()
					m.saveSidebarState()
					break
				}
			}
		}
		return m, m.discoverSessions()

	// ── Error ──────────────────────────────────────────────────────────────
	case errMsg:
		m.err = msg.err

	// ── Keyboard ──────────────────────────────────────────────────────────
	case tea.KeyMsg:
		if m.insertMode {
			if msg.String() == "ctrl+h" {
				m.insertMode = false
			} else if sel := m.selectedSession(); sel != nil {
				if err := m.forwardKey(sel.TmuxPane, msg); err != nil {
					m.err = err
					m.insertMode = false
				} else {
					cmds = append(cmds, m.fetchCapture(sel.TmuxPane))
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, keys.Quit):
			for _, s := range m.sessions {
				_ = m.tmuxClient.ResizePaneAuto(s.TmuxPane)
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			changed := m.moveUp()
			// Always fetch immediately so the viewport reflects the new session
			// without waiting for the next 100ms tick.
			if sel := m.selectedSession(); sel != nil {
				if changed {
					m.lastCapture = ""
					m.forceViewportRefresh = true
					m.pendingGotoBottom = true
					cmds = append(cmds, m.resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
				}
				cmds = append(cmds, m.fetchCapture(sel.TmuxPane))
			}

		case key.Matches(msg, keys.Down):
			changed := m.moveDown()
			if sel := m.selectedSession(); sel != nil {
				if changed {
					m.lastCapture = ""
					m.forceViewportRefresh = true
					m.pendingGotoBottom = true
					cmds = append(cmds, m.resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
				}
				cmds = append(cmds, m.fetchCapture(sel.TmuxPane))
			}

		case key.Matches(msg, keys.Jump):
			if sel := m.selectedSession(); sel != nil {
				if err := m.tmuxClient.SwitchToPane(sel.TmuxPane); err != nil {
					m.err = err
				}
			}

		case key.Matches(msg, keys.Insert):
			m.insertMode = true

		case key.Matches(msg, keys.Refresh):
			cmds = append(cmds, m.discoverSessions())

		case key.Matches(msg, keys.Install):
			selfPath, _ := os.Executable()
			if err := hook.Install(selfPath); err != nil {
				m.err = err
			}

		case key.Matches(msg, keys.Kill):
			if sel := m.selectedSession(); sel != nil {
				if err := m.tmuxClient.KillPane(sel.TmuxPane); err != nil {
					m.err = err
				} else {
					delete(m.pinned, sel.Key())
					m.sessions = append(m.sessions[:m.selected], m.sessions[m.selected+1:]...)
					if m.selected >= len(m.sessions) {
						m.selected = maxInt(0, len(m.sessions)-1)
					}
					m.itemsDirty = true
					var cmd tea.Cmd
					m, cmd = m.selectSession()
					cmds = append(cmds, cmd)
					m.forceViewportRefresh = true
					m.itemsDirty = true
					m.cleanupSidebarState()
					m.saveSidebarState()
				}
			}

		case key.Matches(msg, keys.New):
			var existingPaths []string
			for _, s := range m.sessions {
				if s.ProjectPath != "" {
					existingPaths = append(existingPaths, s.ProjectPath)
				}
			}
			pickerModel := NewPickerModel(existingPaths)
			updatedModel, _ := pickerModel.Update(tea.WindowSizeMsg{
				Width:  m.width,
				Height: m.height,
			})
			pickerModel = updatedModel.(PickerModel)
			m.pickerModel = &pickerModel
			m.mode = ModePicker

		case key.Matches(msg, keys.Worktree):
			if sel := m.selectedSession(); sel != nil && sel.GitRoot != "" {
				if worktrees, err := git.ListWorktrees(sel.GitRoot); err == nil {
					wm := NewWorktreeModel(worktrees, sel.GitRoot, m.sessions, m.width, m.height)
					m.worktreeModel = &wm
					m.mode = ModeWorktree
				}
			}

		case key.Matches(msg, keys.Review):
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
							updatedModel, _ := reviewModel.Update(tea.WindowSizeMsg{
								Width:  m.width,
								Height: m.height,
							})
							reviewModel = updatedModel.(ReviewModel)
							m.reviewModel = &reviewModel
							m.mode = ModeReview
						}
					}
				}
			}

		case key.Matches(msg, keys.Filter):
			m.mode = ModeFilter
			m.filterInput.Focus()

		case key.Matches(msg, keys.Rename):
			if sel := m.selectedSession(); sel != nil {
				m.renameKey = sel.Key()
				m.renameInput.SetValue(names.Get(m.renameKey))
				m.renameInput.Focus()
				m.mode = ModeRename
			}

		case key.Matches(msg, keys.ToggleGroup):
			m.toggleGroupAtCursor()
			m.itemsDirty = true

		case key.Matches(msg, keys.SetGroup):
			if m.cursorOnGroup == "" {
				if sel := m.selectedSession(); sel != nil {
					m.groupSetKey = sel.Key()
					current := groups.Get(m.groupSetKey)
					m.groupSetInput.SetValue(current)
					m.groupSetInput.Focus()
					m.mode = ModeGroupSet
				}
			}

		case key.Matches(msg, keys.Pin):
			if sel := m.selectedSession(); sel != nil {
				gKey, _ := m.groupKeyAndName(*sel)
				if gKey != "" {
					// Pin or unpin the entire group as a unit so the group
					// moves to the top (or back) together.
					_, isCurrentlyPinned := m.pinned[sel.Key()]
					if isCurrentlyPinned {
						for _, s := range m.sessions {
							if k, _ := m.groupKeyAndName(s); k == gKey {
								delete(m.pinned, s.Key())
							}
						}
					} else {
						// Each member gets a sequential counter so that
						// within-group reorder (J/K) survives session refreshes.
						for _, s := range m.sessions {
							if k, _ := m.groupKeyAndName(s); k == gKey {
								m.pinCounter++
								m.pinned[s.Key()] = m.pinCounter
							}
						}
					}
				} else {
					// Ungrouped session — pin individually.
					if _, isPinned := m.pinned[sel.Key()]; isPinned {
						delete(m.pinned, sel.Key())
					} else {
						m.pinCounter++
						m.pinned[sel.Key()] = m.pinCounter
					}
				}
				m.sortSessions()
				m.saveSidebarState()
				m.itemsDirty = true
			}

		case key.Matches(msg, keys.MoveUp):
			if moved, newSel := m.moveSessionUp(); moved {
				m.selected = newSel
				var cmd tea.Cmd
				m, cmd = m.selectSession()
				cmds = append(cmds, cmd)
				m.forceViewportRefresh = true
				m.saveSidebarState()
				m.itemsDirty = true
			}

		case key.Matches(msg, keys.MoveDown):
			if moved, newSel := m.moveSessionDown(); moved {
				m.selected = newSel
				var cmd tea.Cmd
				m, cmd = m.selectSession()
				cmds = append(cmds, cmd)
				m.forceViewportRefresh = true
				m.saveSidebarState()
				m.itemsDirty = true
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
				idx, groupKey := m.sessionIndexAtY(msg.Y)
				if groupKey != "" {
					// Clicked a group header — toggle collapse
					m.collapsedGroups[groupKey] = !m.collapsedGroups[groupKey]
					m.itemsDirty = true
				} else if idx >= 0 && idx < len(m.sessions) {
					if m.selected != idx {
						m.selected = idx
						m.cursorOnGroup = ""
						m.itemsDirty = true
						var cmd tea.Cmd
						m, cmd = m.selectSession()
						cmds = append(cmds, cmd)
						m.forceViewportRefresh = true
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

// ── Worktree helpers ───────────────────────────────────────────────────────

// openOrSwitchWorktree returns a Cmd that switches to an existing herd session
// whose ProjectPath is inside wtPath, or launches a new Claude session there.
func (m *Model) openOrSwitchWorktree(wtPath string) tea.Cmd {
	for i, s := range m.sessions {
		if strings.HasPrefix(s.ProjectPath, wtPath) {
			m.selected = i
			m.lastCapture = ""
			m.pendingGotoBottom = true
			return m.fetchCapture(s.TmuxPane)
		}
	}
	client := m.tmuxClient
	return func() tea.Msg {
		paneID, err := LaunchSession(wtPath, client)
		if err != nil {
			return errMsg{err}
		}
		return worktreeLaunchedMsg(paneID)
	}
}

// removeWorktree is a Cmd that kills the associated session (if any) then removes the worktree.
func removeWorktree(client tmux.ClientIface, repoRoot, wtPath, sessionPane string) tea.Cmd {
	return func() tea.Msg {
		if sessionPane != "" {
			_ = client.KillPane(sessionPane)
		}
		if err := git.RemoveWorktree(repoRoot, wtPath); err != nil {
			return errMsg{err}
		}
		return worktreeRemovedMsg{sessionPane: sessionPane}
	}
}

// createAndLaunchWorktree is a Cmd that creates the worktree then launches Claude.
func createAndLaunchWorktree(client tmux.ClientIface, repoRoot, path, branch string) tea.Cmd {
	return func() tea.Msg {
		if err := git.AddWorktree(repoRoot, path, branch); err != nil {
			return errMsg{err}
		}
		paneID, err := LaunchSession(path, client)
		if err != nil {
			return errMsg{err}
		}
		return worktreeLaunchedMsg(paneID)
	}
}

// ── Key forwarding ─────────────────────────────────────────────────────────

// tmuxKeyNames maps tea key strings to tmux send-keys names.
var tmuxKeyNames = map[string]string{
	"enter":     "Enter",
	"backspace": "BSpace",
	"delete":    "DC",
	"tab":       "Tab",
	"ctrl+i":    "Tab",
	"shift+tab": "BTab",
	"up":        "Up",
	"down":      "Down",
	"left":      "Left",
	"right":     "Right",
	"home":      "Home",
	"end":       "End",
	"pgup":      "PPage",
	"pgdown":    "NPage",
	"esc":       "Escape",
	" ":         "Space",
	"ctrl+a":    "C-a",
	"ctrl+b":    "C-b",
	"ctrl+c":    "C-c",
	"ctrl+d":    "C-d",
	"ctrl+e":    "C-e",
	"ctrl+f":    "C-f",
	"ctrl+g":    "C-g",
	// ctrl+h is the exit-insert-mode key — intentionally absent.
	"ctrl+j": "C-j",
	"ctrl+k": "C-k",
	"ctrl+l": "C-l",
	"ctrl+m": "Enter",
	"ctrl+n": "C-n",
	"ctrl+o": "C-o",
	"ctrl+p": "C-p",
	"ctrl+q": "C-q",
	"ctrl+r": "C-r",
	"ctrl+s": "C-s",
	"ctrl+t": "C-t",
	"ctrl+u": "C-u",
	"ctrl+v": "C-v",
	"ctrl+w": "C-w",
	"ctrl+x": "C-x",
	"ctrl+y": "C-y",
	"ctrl+z": "C-z",
}

// forwardKey sends a single key event to the given tmux pane.
// ctrl+h is the exit-insert-mode key and is never forwarded.
func (m Model) forwardKey(paneID string, msg tea.KeyMsg) error {
	if msg.String() == "ctrl+h" {
		return nil // exit key — handled by caller
	}
	if name, ok := tmuxKeyNames[msg.String()]; ok {
		return m.tmuxClient.SendKeyName(paneID, name)
	}
	if msg.Type == tea.KeyRunes {
		if len(msg.Runes) == 0 {
			return nil
		}
		return m.tmuxClient.SendLiteral(paneID, string(msg.Runes))
	}
	return nil
}

// selectSession resets viewport state for a newly selected session and returns
// commands to resize the observed pane and fetch its capture.
func (m Model) selectSession() (Model, tea.Cmd) {
	m.lastCapture = ""
	m.pendingGotoBottom = true
	var cmds []tea.Cmd
	if sel := m.selectedSession(); sel != nil {
		cmds = append(cmds, m.resizePaneToViewport(sel.TmuxPane, m.viewport.Width, m.viewport.Height))
		cmds = append(cmds, m.fetchCapture(sel.TmuxPane))
	}
	return m, tea.Batch(cmds...)
}

// ── Helpers ────────────────────────────────────────────────────────────────

// resizePaneToViewport resizes the tmux window containing paneID to width×height
// so that the observed session formats its output to fit the herd viewport.
// This is a fire-and-forget async command; errors are silently ignored.
func (m Model) resizePaneToViewport(paneID string, width, height int) tea.Cmd {
	if paneID == "" || width <= 0 || height <= 0 {
		return nil
	}
	client := m.tmuxClient
	return func() tea.Msg {
		_ = client.ResizeWindow(paneID, width, height)
		return nil
	}
}

func (m Model) fetchCapture(paneID string) tea.Cmd {
	client := m.tmuxClient
	return func() tea.Msg {
		content, err := client.CapturePane(paneID, 2000)
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
		m.sessions[i].State = session.ParseState(st.State)
		m.sessions[i].CurrentTool = st.CurrentTool
		m.sessions[i].UpdatedAt = st.UpdatedAt
	}
	return m
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

// sessionIndexAtY maps a screen Y coordinate to the sidebar item at that
// position. Returns (sessionIdx, groupKey) where:
//   - sessionIdx >= 0: a session row was clicked (groupKey is "")
//   - groupKey != "": a group header was clicked (sessionIdx is -1)
//   - (-1, ""): click fell outside any item
func (m *Model) sessionIndexAtY(y int) (int, string) {
	const headerH = 1
	contentY := y - headerH
	if contentY < 0 {
		return -1, ""
	}

	// When filtering, there's an extra filter bar line at the top
	// and the list is flat (no group headers).
	if m.mode == ModeFilter || m.isFiltered() {
		contentY-- // skip filter bar
		if contentY < 0 {
			return -1, ""
		}
		sessions := m.filteredSessions()
		idx := contentY / 2
		if idx >= len(sessions) {
			return -1, ""
		}
		if m.filtered != nil && idx < len(m.filtered) {
			return m.filtered[idx], ""
		}
		return idx, ""
	}

	// Walk viewItems to find the item at the clicked row.
	// Group headers occupy 1 row; session items occupy 2 rows.
	items := m.viewItems()
	row := 0
	for _, item := range items {
		if item.isHeader {
			if contentY == row {
				return -1, item.groupKey
			}
			row++
		} else {
			if contentY >= row && contentY < row+2 {
				return item.sessionIdx, ""
			}
			row += 2
		}
	}
	return -1, ""
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
