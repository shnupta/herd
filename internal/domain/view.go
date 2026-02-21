package domain

import (
	"github.com/shnupta/herd/internal/session"
)

// ViewItem represents a single row in the sidebar — either a group header or a session row.
type ViewItem struct {
	IsHeader   bool
	GroupKey   string
	GroupName  string
	Count      int
	AggState   session.State
	SessionIdx int // index into the sessions slice; -1 for headers
}

// PreGroupedSession is a session with its group assignment pre-computed.
type PreGroupedSession struct {
	Session   session.Session
	GroupKey  string // empty string means ungrouped
	GroupName string
}

// BuildViewItems constructs the flat sidebar row list from pre-grouped sessions.
// Groups consecutive sessions with the same GroupKey under a header.
// Collapsed groups are represented by just their header row.
func BuildViewItems(sessions []PreGroupedSession, collapsed map[string]bool) []ViewItem {
	if len(sessions) == 0 {
		return nil
	}

	type groupData struct {
		name     string
		sessions []int // indices into sessions
	}
	groupMap := make(map[string]*groupData)
	for i, pg := range sessions {
		if pg.GroupKey == "" {
			continue
		}
		if _, exists := groupMap[pg.GroupKey]; !exists {
			groupMap[pg.GroupKey] = &groupData{name: pg.GroupName}
		}
		groupMap[pg.GroupKey].sessions = append(groupMap[pg.GroupKey].sessions, i)
	}

	emittedGroups := make(map[string]bool)
	var items []ViewItem

	for i, pg := range sessions {
		if pg.GroupKey == "" {
			items = append(items, ViewItem{
				IsHeader:   false,
				SessionIdx: i,
			})
			continue
		}

		if emittedGroups[pg.GroupKey] {
			continue
		}
		emittedGroups[pg.GroupKey] = true

		g := groupMap[pg.GroupKey]
		var states []session.State
		for _, idx := range g.sessions {
			states = append(states, sessions[idx].Session.State)
		}
		items = append(items, ViewItem{
			IsHeader:   true,
			GroupKey:    pg.GroupKey,
			GroupName:   g.name,
			Count:       len(g.sessions),
			AggState:    WorstState(states),
			SessionIdx: -1,
		})
		if !collapsed[pg.GroupKey] {
			for _, idx := range g.sessions {
				items = append(items, ViewItem{
					IsHeader:   false,
					GroupKey:    pg.GroupKey,
					SessionIdx: idx,
				})
			}
		}
	}
	return items
}

// WorstState returns the most severe state among the provided states.
// Severity order: working > waiting > plan_ready > notifying > idle > unknown
func WorstState(states []session.State) session.State {
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
