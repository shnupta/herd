package domain

import (
	"sort"

	"github.com/shnupta/herd/internal/session"
	"github.com/shnupta/herd/internal/state"
)

// MergeSessions applies state updates to a session slice.
// Matches by session ID first, then by pane ID as fallback.
func MergeSessions(sessions []session.Session, updates []state.SessionState) []session.Session {
	byPane := make(map[string]state.SessionState)
	byID := make(map[string]state.SessionState)
	for _, s := range updates {
		if s.TmuxPane != "" {
			byPane[s.TmuxPane] = s
		}
		if s.SessionID != "" {
			byID[s.SessionID] = s
		}
	}

	result := make([]session.Session, len(sessions))
	copy(result, sessions)

	for i, sess := range result {
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
		result[i].ID = st.SessionID
		result[i].State = session.ParseState(st.State)
		result[i].CurrentTool = st.CurrentTool
		result[i].UpdatedAt = st.UpdatedAt
	}
	return result
}

// SortSessions sorts sessions by pin order then saved order.
// pinned: map of session Key() → pin counter (lower = pinned first)
// savedOrder: ordered list of session Key() values for unpinned sessions
func SortSessions(sessions []session.Session, pinned map[string]int, savedOrder []string) []session.Session {
	if len(sessions) <= 1 {
		return sessions
	}

	result := make([]session.Session, len(sessions))
	copy(result, sessions)

	orderIndex := make(map[string]int)
	for i, key := range savedOrder {
		orderIndex[key] = i
	}

	var pinnedSessions, unpinnedSessions []session.Session
	for _, s := range result {
		if _, ok := pinned[s.Key()]; ok {
			pinnedSessions = append(pinnedSessions, s)
		} else {
			unpinnedSessions = append(unpinnedSessions, s)
		}
	}

	sort.SliceStable(pinnedSessions, func(i, j int) bool {
		return pinned[pinnedSessions[i].Key()] < pinned[pinnedSessions[j].Key()]
	})

	sort.Slice(unpinnedSessions, func(i, j int) bool {
		iOrder, iOk := orderIndex[unpinnedSessions[i].Key()]
		jOrder, jOk := orderIndex[unpinnedSessions[j].Key()]
		if iOk && jOk {
			return iOrder < jOrder
		}
		return jOk && !iOk
	})

	return append(pinnedSessions, unpinnedSessions...)
}
