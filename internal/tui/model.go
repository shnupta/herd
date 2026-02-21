package tui

import (
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shnupta/herd/internal/groups"
	"github.com/shnupta/herd/internal/names"
	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/sidebar"
	"github.com/shnupta/herd/internal/state"
)

// viewItem represents a single renderable/navigable row in the session sidebar.
// It is either a group header or a session entry.
type viewItem struct {
	isHeader  bool
	groupKey  string
	groupName string
	count     int
	aggState  session.State
	sessionIdx int // index into m.sessions; meaningful only when !isHeader
}

// msg types used by the BubbleTea event loop.

type tickMsg time.Time
type sessionRefreshMsg time.Time

type sessionsDiscoveredMsg []session.Session

type captureMsg struct {
	paneID  string
	content string
}

type stateUpdateMsg state.SessionState

type errMsg struct{ err error }

type worktreeLaunchedMsg string

type worktreeRemovedMsg struct{ sessionPane string }

// Model is the root BubbleTea model.
type Model struct {
	// Dimensions
	width  int
	height int

	// Session list
	sessions []session.Session
	selected int

	// Output panel
	viewport          viewport.Model
	lastCapture       string // raw content from last capture-pane
	atBottom          bool   // whether viewport was at the bottom before update
	pendingGotoBottom bool   // true after a session switch; forces GotoBottom on next capture

	// Input
	insertMode bool // true when keystrokes are forwarded to the selected pane

	// Filter mode
	filterMode   bool               // true when filtering session list
	filterInput  textinput.Model    // text input for filter
	filterQuery  string             // current filter query
	filtered     []int              // indices of sessions that match filter

	// Diff review mode
	reviewMode  bool         // true when in diff review mode
	reviewModel *ReviewModel // the review sub-model

	// Project picker mode
	pickerMode  bool         // true when in project picker mode
	pickerModel *PickerModel // the picker sub-model

	// Worktree mode
	worktreeMode  bool           // true when the worktree panel is open
	worktreeModel *WorktreeModel // the worktree sub-model

	// Rename mode
	renameMode  bool             // true when the rename overlay is open
	renameInput textinput.Model  // text input for the rename overlay
	renameKey   string           // session key being renamed

	// Group-set mode
	groupSetMode  bool            // true when the group-assign overlay is open
	groupSetInput textinput.Model // text input for the group name
	groupSetKey   string          // session key being re-grouped

	// Custom session names
	namesStore *names.Store

	// Session grouping
	groupsStore     *groups.Store
	collapsedGroups map[string]bool // groupKey → true when collapsed
	cursorOnGroup   string          // non-empty when cursor rests on a collapsed group header

	// Pending selection after new session creation
	pendingSelectPane string // pane ID to select after next session discovery

	// Pinning and ordering (keyed by session key: "session:<id>" or "pane:<id>")
	pinned       map[string]int // sessionKey -> pin order (lower = pinned earlier)
	pinCounter   int            // increments on each pin to assign order
	savedOrder   []string       // persisted order of session keys
	sidebarDirty bool           // true if sidebar state needs saving

	// State
	spinner  spinner.Model
	stateWatcher *state.Watcher
	err      error
	ready    bool
}

const (
	pollInterval         = 100 * time.Millisecond
	sessionRefreshInterval = 3 * time.Second
)

// New returns an initialised Model.
func New(w *state.Watcher) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 100

	ri := textinput.New()
	ri.Placeholder = "session name..."
	ri.CharLimit = 100

	gi := textinput.New()
	gi.Placeholder = "group name (empty to auto-detect)..."
	gi.CharLimit = 100

	// Load persisted sidebar state
	pinned := make(map[string]int)
	var savedOrder []string
	var pinCounter int
	if sidebarState, err := sidebar.Load(); err == nil {
		pinned = sidebarState.Pinned
		savedOrder = sidebarState.Order
		// Find max pin order to set counter
		for _, order := range pinned {
			if order > pinCounter {
				pinCounter = order
			}
		}
	}

	// Load persisted session names
	home, _ := os.UserHomeDir()
	ns := names.NewStore(home + "/.herd/names.json")
	_ = ns.Load()

	// Load persisted custom group assignments
	gs := groups.NewStore(home + "/.herd/groups.json")
	_ = gs.Load()

	return Model{
		spinner:         sp,
		stateWatcher:    w,
		atBottom:        true,
		filterInput:     fi,
		renameInput:     ri,
		groupSetInput:   gi,
		pinned:          pinned,
		pinCounter:      pinCounter,
		savedOrder:      savedOrder,
		namesStore:      ns,
		groupsStore:     gs,
		collapsedGroups: make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		discoverSessions(),
		tickCapture(),
		tickSessionRefresh(),
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

// tickSessionRefresh returns a command that fires after sessionRefreshInterval.
func tickSessionRefresh() tea.Cmd {
	return tea.Tick(sessionRefreshInterval, func(t time.Time) tea.Msg {
		return sessionRefreshMsg(t)
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

// sortSessions sorts sessions with pinned sessions at the top (by pin order),
// then applies saved order for unpinned sessions.
func (m *Model) sortSessions() {
	if len(m.sessions) <= 1 {
		return
	}

	// Remember currently selected pane to restore selection after sort
	var selectedPane string
	if m.selected < len(m.sessions) {
		selectedPane = m.sessions[m.selected].TmuxPane
	}

	// Build order index from saved order
	orderIndex := make(map[string]int)
	for i, key := range m.savedOrder {
		orderIndex[key] = i
	}

	// Separate pinned and unpinned sessions
	var pinned, unpinned []session.Session
	for _, s := range m.sessions {
		if _, ok := m.pinned[s.Key()]; ok {
			pinned = append(pinned, s)
		} else {
			unpinned = append(unpinned, s)
		}
	}

	// Sort pinned sessions by their pin order
	for i := 0; i < len(pinned)-1; i++ {
		for j := i + 1; j < len(pinned); j++ {
			if m.pinned[pinned[i].Key()] > m.pinned[pinned[j].Key()] {
				pinned[i], pinned[j] = pinned[j], pinned[i]
			}
		}
	}

	// Sort unpinned sessions by saved order (unknown keys go to end)
	for i := 0; i < len(unpinned)-1; i++ {
		for j := i + 1; j < len(unpinned); j++ {
			iOrder, iOk := orderIndex[unpinned[i].Key()]
			jOrder, jOk := orderIndex[unpinned[j].Key()]
			// If both have saved order, sort by it
			// If only one has saved order, it comes first
			// If neither has saved order, keep original order
			if iOk && jOk && iOrder > jOrder {
				unpinned[i], unpinned[j] = unpinned[j], unpinned[i]
			} else if !iOk && jOk {
				unpinned[i], unpinned[j] = unpinned[j], unpinned[i]
			}
		}
	}

	// Combine: pinned first, then unpinned
	m.sessions = append(pinned, unpinned...)

	// Restore selection
	if selectedPane != "" {
		for i, s := range m.sessions {
			if s.TmuxPane == selectedPane {
				m.selected = i
				break
			}
		}
	}
}

// saveSidebarState persists the current pin and order state.
func (m *Model) saveSidebarState() {
	// Build order from current session list using session keys
	order := make([]string, 0, len(m.sessions))
	for _, s := range m.sessions {
		order = append(order, s.Key())
	}
	m.savedOrder = order

	state := &sidebar.State{
		Pinned: m.pinned,
		Order:  order,
	}
	_ = sidebar.Save(state) // Best effort, ignore errors
	m.sidebarDirty = false
}

// ── Group helpers ──────────────────────────────────────────────────────────

// groupKeyAndName returns the group key and human-readable name for a session.
// Returns ("", "") when the session has no explicit group assignment, meaning
// it should appear as a flat item with no header in the sidebar.
func (m *Model) groupKeyAndName(s session.Session) (key, name string) {
	if custom := m.groupsStore.Get(s.Key()); custom != "" {
		return "custom:" + custom, custom
	}
	return "", "" // no explicit group — render flat
}

// worstState returns the highest-priority state from the provided slice.
// Priority: Working > Waiting > PlanReady > Notifying > Idle > Unknown.
func worstState(states []session.State) session.State {
	priority := map[session.State]int{
		session.StateWorking:   5,
		session.StateWaiting:   4,
		session.StatePlanReady: 3,
		session.StateNotifying: 2,
		session.StateIdle:      1,
		session.StateUnknown:   0,
	}
	worst := session.StateUnknown
	for _, s := range states {
		if priority[s] > priority[worst] {
			worst = s
		}
	}
	return worst
}

// buildViewItems builds the ordered list of renderable/navigable sidebar rows.
// Sessions without an explicit group assignment appear as flat items with no
// header. Sessions in an explicit group are gathered under a named header,
// inserted at the position of the group's first session in m.sessions order.
// Collapsed groups contribute only their header row.
func (m *Model) buildViewItems() []viewItem {
	if len(m.sessions) == 0 {
		return nil
	}

	// Pre-compute per-group aggregate data (count, states) so we can render
	// headers correctly when we encounter the first session of each group.
	type groupData struct {
		name     string
		sessions []int // indices into m.sessions
	}
	groupMap := make(map[string]*groupData)
	for i, s := range m.sessions {
		gKey, gName := m.groupKeyAndName(s)
		if gKey == "" {
			continue // ungrouped — no aggregate needed
		}
		if _, exists := groupMap[gKey]; !exists {
			groupMap[gKey] = &groupData{name: gName}
		}
		groupMap[gKey].sessions = append(groupMap[gKey].sessions, i)
	}

	emittedHeaders := make(map[string]bool)
	var items []viewItem

	for i, s := range m.sessions {
		gKey, _ := m.groupKeyAndName(s)

		if gKey == "" {
			// Ungrouped session — flat item, no header.
			items = append(items, viewItem{
				isHeader:   false,
				sessionIdx: i,
			})
			continue
		}

		// Explicitly grouped session — emit the group header once, then the session.
		if !emittedHeaders[gKey] {
			emittedHeaders[gKey] = true
			g := groupMap[gKey]
			var states []session.State
			for _, idx := range g.sessions {
				states = append(states, m.sessions[idx].State)
			}
			items = append(items, viewItem{
				isHeader:  true,
				groupKey:  gKey,
				groupName: g.name,
				count:     len(g.sessions),
				aggState:  worstState(states),
			})
		}
		if !m.collapsedGroups[gKey] {
			items = append(items, viewItem{
				isHeader:   false,
				groupKey:   gKey,
				sessionIdx: i,
			})
		}
	}
	return items
}

// findCursorPos returns the index of the current cursor position in items.
// Returns -1 if not found.
func (m *Model) findCursorPos(items []viewItem) int {
	if m.cursorOnGroup != "" {
		for i, item := range items {
			if item.isHeader && item.groupKey == m.cursorOnGroup {
				return i
			}
		}
		return -1
	}
	for i, item := range items {
		if !item.isHeader && item.sessionIdx == m.selected {
			return i
		}
	}
	return -1
}

// moveDown advances the cursor to the next navigable row and returns true if
// the selected session changed (so callers can trigger viewport/resize actions).
func (m *Model) moveDown() bool {
	items := m.buildViewItems()
	if len(items) == 0 {
		return false
	}
	curPos := m.findCursorPos(items)
	if curPos < 0 {
		curPos = -1
	}
	for i := curPos + 1; i < len(items); i++ {
		item := items[i]
		if item.isHeader {
			if m.collapsedGroups[item.groupKey] {
				m.cursorOnGroup = item.groupKey
				return false
			}
			continue
		}
		prev := m.selected
		m.cursorOnGroup = ""
		m.selected = item.sessionIdx
		return m.selected != prev
	}
	return false
}

// moveUp moves the cursor to the previous navigable row and returns true if
// the selected session changed.
func (m *Model) moveUp() bool {
	items := m.buildViewItems()
	if len(items) == 0 {
		return false
	}
	curPos := m.findCursorPos(items)
	if curPos < 0 {
		curPos = len(items)
	}
	for i := curPos - 1; i >= 0; i-- {
		item := items[i]
		if item.isHeader {
			if m.collapsedGroups[item.groupKey] {
				m.cursorOnGroup = item.groupKey
				return false
			}
			continue
		}
		prev := m.selected
		m.cursorOnGroup = ""
		m.selected = item.sessionIdx
		return m.selected != prev
	}
	return false
}

// toggleGroupAtCursor collapses or expands the group that contains the current
// cursor position (whether the cursor is on a header or a session within it).
func (m *Model) toggleGroupAtCursor() {
	if m.cursorOnGroup != "" {
		// Cursor is on a collapsed group header — expand it and move into it.
		gKey := m.cursorOnGroup
		m.collapsedGroups[gKey] = false
		m.cursorOnGroup = ""
		// Select the first session in this group.
		for _, s := range m.sessions {
			k, _ := m.groupKeyAndName(s)
			if k == gKey {
				for i, ms := range m.sessions {
					if ms.TmuxPane == s.TmuxPane {
						m.selected = i
						break
					}
				}
				break
			}
		}
		return
	}
	// Cursor is on a session — collapse its group (only if it has one).
	if m.selected < len(m.sessions) {
		gKey, _ := m.groupKeyAndName(m.sessions[m.selected])
		if gKey != "" {
			m.collapsedGroups[gKey] = !m.collapsedGroups[gKey]
		}
	}
}

// cleanupSidebarState removes entries for sessions no longer active.
func (m *Model) cleanupSidebarState() {
	activeKeys := make(map[string]bool)
	for _, s := range m.sessions {
		activeKeys[s.Key()] = true
	}

	// Clean pinned entries
	changed := false
	for key := range m.pinned {
		if !activeKeys[key] {
			delete(m.pinned, key)
			changed = true
		}
	}

	// Clean saved order
	var newOrder []string
	for _, key := range m.savedOrder {
		if activeKeys[key] {
			newOrder = append(newOrder, key)
		} else {
			changed = true
		}
	}
	m.savedOrder = newOrder

	if changed {
		m.sidebarDirty = true
	}
}
