package domain

import (
	"strings"

	"github.com/shnupta/herd/internal/session"
)

// ApplyFilter returns indices into sessions that match the query.
// Match is case-insensitive and checks ProjectPath, GitBranch, TmuxPane, ID.
// An empty query returns all indices.
func ApplyFilter(query string, sessions []session.Session) []int {
	if query == "" {
		indices := make([]int, len(sessions))
		for i := range sessions {
			indices[i] = i
		}
		return indices
	}

	q := strings.ToLower(query)
	var result []int

	for i, s := range sessions {
		searchable := strings.ToLower(s.ProjectPath + " " + s.GitBranch + " " + s.TmuxPane + " " + s.ID)
		if strings.Contains(searchable, q) {
			result = append(result, i)
		}
	}
	return result
}
