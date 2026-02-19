package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/state"
)

// msg types used by the BubbleTea event loop.

type tickMsg time.Time

type sessionsDiscoveredMsg []session.Session

type captureMsg struct {
	paneID  string
	content string
}

type stateUpdateMsg state.SessionState

type errMsg struct{ err error }

// Model is the root BubbleTea model.
type Model struct {
	// Dimensions
	width  int
	height int

	// Session list
	sessions []session.Session
	selected int

	// Output panel
	viewport    viewport.Model
	lastCapture string // raw content from last capture-pane
	atBottom    bool   // whether viewport was at the bottom before update

	// Input
	insertMode bool // true when keystrokes are forwarded to the selected pane

	// State
	spinner  spinner.Model
	stateWatcher *state.Watcher
	err      error
	ready    bool
}

const pollInterval = 100 * time.Millisecond

// New returns an initialised Model.
func New(w *state.Watcher) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		spinner:      sp,
		stateWatcher: w,
		atBottom:     true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		discoverSessions(),
		tickCapture(),
		waitForStateEvent(m.stateWatcher),
		m.spinner.Tick,
	)
}

// discoverSessions triggers async session discovery.
func discoverSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := session.Discover()
		if err != nil {
			return errMsg{err}
		}
		return sessionsDiscoveredMsg(sessions)
	}
}

// tickCapture returns a command that fires after pollInterval.
func tickCapture() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// waitForStateEvent waits for the next state file event from fsnotify.
func waitForStateEvent(w *state.Watcher) tea.Cmd {
	if w == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-w.Events
		if !ok {
			return nil
		}
		return stateUpdateMsg(ev)
	}
}

// selectedSession returns a pointer to the currently selected session, or nil.
func (m *Model) selectedSession() *session.Session {
	if len(m.sessions) == 0 || m.selected >= len(m.sessions) {
		return nil
	}
	return &m.sessions[m.selected]
}
