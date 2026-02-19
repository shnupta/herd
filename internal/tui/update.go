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

	// If in review mode, delegate to review model
	if m.reviewMode && m.reviewModel != nil {
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
		} else if reviewModel.Cancelled() {
			m.reviewMode = false
			m.reviewModel = nil
		}

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
		cmds = append(cmds, m.resizePaneCmd())

	// ── Initial session discovery ──────────────────────────────────────────
	case sessionsDiscoveredMsg:
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
		if m.selected >= len(m.sessions) {
			m.selected = maxInt(0, len(m.sessions)-1)
		}
		if states, err := state.ReadAll(); err == nil {
			m = m.applyStates(states)
		}

	// ── Session list auto-refresh ──────────────────────────────────────────
	case sessionRefreshMsg:
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
			m.atBottom = m.viewport.AtBottom()
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
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			if m.selected > 0 {
				m.selected--
				m.lastCapture = ""
				cmds = append(cmds, m.resizePaneCmd())
			}

		case key.Matches(msg, keys.Down):
			if m.selected < len(m.sessions)-1 {
				m.selected++
				m.lastCapture = ""
				cmds = append(cmds, m.resizePaneCmd())
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
					m.sessions = append(m.sessions[:m.selected], m.sessions[m.selected+1:]...)
					if m.selected >= len(m.sessions) {
						m.selected = maxInt(0, len(m.sessions)-1)
					}
					m.lastCapture = ""
					cmds = append(cmds, m.resizePaneCmd())
				}
			}

		case key.Matches(msg, keys.New):
			// TODO: project selector modal

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
						cmds = append(cmds, m.resizePaneCmd())
					}
				}
			}
		}
	}

	// Forward scroll and other events to viewport when not in insert mode.
	if !m.insertMode {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// resizePaneCmd returns a Cmd that resizes the selected Claude pane to match
// the viewport width, then immediately fetches a fresh capture so the viewport
// reflects the new line width without waiting for the next poll tick.
func (m Model) resizePaneCmd() tea.Cmd {
	sel := m.selectedSession()
	if sel == nil || m.viewport.Width <= 0 {
		return nil
	}
	paneID := sel.TmuxPane
	width := m.viewport.Width
	return func() tea.Msg {
		_ = tmux.ResizePane(paneID, width)
		content, err := tmux.CapturePane(paneID, 2000)
		if err != nil {
			return nil
		}
		return captureMsg{paneID: paneID, content: content}
	}
}
