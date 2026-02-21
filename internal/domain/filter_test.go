package domain

import (
	"reflect"
	"testing"

	"github.com/shnupta/herd/internal/session"
)

func TestApplyFilter_EmptyQuery(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1"},
		{TmuxPane: "%2"},
	}
	result := ApplyFilter("", sessions)
	expected := []int{0, 1}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyFilter_MatchesProjectPath(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1", ProjectPath: "/home/user/myproject"},
		{TmuxPane: "%2", ProjectPath: "/home/user/other"},
	}
	result := ApplyFilter("myproject", sessions)
	expected := []int{0}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyFilter_MatchesGitBranch(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1", GitBranch: "main"},
		{TmuxPane: "%2", GitBranch: "feat/login"},
	}
	result := ApplyFilter("feat", sessions)
	expected := []int{1}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyFilter_MatchesTmuxPane(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%10"},
		{TmuxPane: "%20"},
	}
	result := ApplyFilter("%20", sessions)
	expected := []int{1}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1", ProjectPath: "/home/user/MyProject"},
	}
	result := ApplyFilter("MYPROJECT", sessions)
	expected := []int{0}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	sessions := []session.Session{
		{TmuxPane: "%1", ProjectPath: "/home/user/foo"},
	}
	result := ApplyFilter("zzz", sessions)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestApplyFilter_MatchesID(t *testing.T) {
	sessions := []session.Session{
		{ID: "session-abc-123", TmuxPane: "%1"},
		{ID: "session-def-456", TmuxPane: "%2"},
	}
	result := ApplyFilter("abc", sessions)
	expected := []int{0}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
