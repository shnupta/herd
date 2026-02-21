package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/shnupta/herd/internal/git"
	"github.com/shnupta/herd/internal/session"
)

// testWorktrees returns a slice of worktrees for use in fixtures.
// Index 0 is always the main worktree.
func testWorktrees() []git.Worktree {
	return []git.Worktree{
		{Path: "/home/user/repo", Branch: "main", IsMain: true},
		{Path: "/home/user/worktrees/repo-feat-login", Branch: "feat/login"},
		{Path: "/home/user/worktrees/repo-fix-bug", Branch: "fix/bug"},
	}
}

func newTestWorktreeModel(wts []git.Worktree) WorktreeModel {
	return NewWorktreeModel(wts, "/home/user/repo", nil, 120, 40)
}

// sendKey is a convenience to call Update with a key rune.
func sendKey(m WorktreeModel, r rune) WorktreeModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(WorktreeModel)
}

func sendSpecialKey(m WorktreeModel, t tea.KeyType) WorktreeModel {
	updated, _ := m.Update(tea.KeyMsg{Type: t})
	return updated.(WorktreeModel)
}

// ── Listing state ─────────────────────────────────────────────────────────

func TestWorktreeModel_InitialState(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	if m.selected != 0 {
		t.Errorf("expected selected=0, got %d", m.selected)
	}
	if m.state != worktreeStateListing {
		t.Errorf("expected listing state, got %d", m.state)
	}
}

func TestWorktreeModel_NavigateDown(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	if m.selected != 1 {
		t.Errorf("expected selected=1 after j, got %d", m.selected)
	}
	m = sendKey(m, 'j')
	if m.selected != 2 {
		t.Errorf("expected selected=2 after second j, got %d", m.selected)
	}
}

func TestWorktreeModel_NavigateUp(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	m = sendKey(m, 'j')
	m = sendKey(m, 'k')
	if m.selected != 1 {
		t.Errorf("expected selected=1 after j,j,k, got %d", m.selected)
	}
}

func TestWorktreeModel_NavigateDoesNotGoBelowZero(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'k') // already at 0
	if m.selected != 0 {
		t.Errorf("expected selected to stay at 0, got %d", m.selected)
	}
}

func TestWorktreeModel_NavigateDoesNotExceedList(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	// list has 3 worktrees + "New worktree..." = 4 items (indices 0–3)
	for i := 0; i < 10; i++ {
		m = sendKey(m, 'j')
	}
	want := 3 // len(worktrees) — last index
	if m.selected != want {
		t.Errorf("expected selected=%d at bottom, got %d", want, m.selected)
	}
}

func TestWorktreeModel_EscCancels(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendSpecialKey(m, tea.KeyEscape)
	if !m.Cancelled() {
		t.Error("expected Cancelled()=true after esc")
	}
}

func TestWorktreeModel_EnterOnExistingWorktreeSetsChosenPath(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j') // select index 1 → worktrees[0] = main
	m = sendSpecialKey(m, tea.KeyEnter)
	if m.ChosenPath() != "/home/user/repo" {
		t.Errorf("expected ChosenPath=/home/user/repo, got %q", m.ChosenPath())
	}
}

func TestWorktreeModel_EnterOnNewWorktreeSwitchesToCreating(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	// selected=0 is "New worktree..."
	m = sendSpecialKey(m, tea.KeyEnter)
	if m.state != worktreeStateCreating {
		t.Errorf("expected worktreeStateCreating, got %d", m.state)
	}
}

// ── Remove/confirm state ──────────────────────────────────────────────────

func TestWorktreeModel_RemoveOnMainIsNoOp(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j') // select worktrees[0] = main
	m = sendKey(m, 'x')
	if m.state != worktreeStateListing {
		t.Errorf("x on main worktree should stay in listing, got state=%d", m.state)
	}
}

func TestWorktreeModel_RemoveOnNonMainOpensConfirm(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	m = sendKey(m, 'j') // select worktrees[1] = feat/login (non-main)
	m = sendKey(m, 'x')
	if m.state != worktreeStateConfirming {
		t.Errorf("expected confirming state, got %d", m.state)
	}
}

func TestWorktreeModel_ConfirmEnterSetsRemovePath(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	m = sendKey(m, 'j') // select feat/login
	m = sendKey(m, 'x') // enter confirming
	m = sendSpecialKey(m, tea.KeyEnter)
	wtPath, _, ok := m.ShouldRemove()
	if !ok {
		t.Fatal("expected ShouldRemove()=true after confirm")
	}
	if wtPath != "/home/user/worktrees/repo-feat-login" {
		t.Errorf("unexpected remove path: %q", wtPath)
	}
}

func TestWorktreeModel_ConfirmEscReturnsToListing(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	m = sendKey(m, 'j') // select feat/login
	m = sendKey(m, 'x') // enter confirming
	m = sendSpecialKey(m, tea.KeyEscape)
	if m.state != worktreeStateListing {
		t.Errorf("expected listing after esc in confirming, got %d", m.state)
	}
}

func TestWorktreeModel_RemoveWithAssociatedSession(t *testing.T) {
	wts := testWorktrees()
	sessions := []session.Session{
		{TmuxPane: "%99", ProjectPath: "/home/user/worktrees/repo-feat-login/src"},
	}
	m := NewWorktreeModel(wts, "/home/user/repo", sessions, 120, 40)
	m = sendKey(m, 'j')
	m = sendKey(m, 'j') // select feat/login
	m = sendKey(m, 'x')
	// The confirm screen should have picked up the associated session pane.
	if m.confirmSessionPane != "%99" {
		t.Errorf("expected confirmSessionPane=%%99, got %q", m.confirmSessionPane)
	}
}

// ── Create form state ─────────────────────────────────────────────────────

func TestWorktreeModel_CreateFormEscReturnsToListing(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendSpecialKey(m, tea.KeyEnter) // open create form
	m = sendSpecialKey(m, tea.KeyEscape)
	if m.state != worktreeStateListing {
		t.Errorf("expected listing after esc in create form, got %d", m.state)
	}
}

func TestWorktreeModel_CreateFormTabSwitchesField(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendSpecialKey(m, tea.KeyEnter) // open create form; focusedField=0 (branch)
	m = sendSpecialKey(m, tea.KeyTab)
	if m.focusedField != 1 {
		t.Errorf("expected focusedField=1 after tab, got %d", m.focusedField)
	}
	m = sendSpecialKey(m, tea.KeyTab)
	if m.focusedField != 0 {
		t.Errorf("expected focusedField=0 after second tab, got %d", m.focusedField)
	}
}

func TestWorktreeModel_CreateFormEnterWithEmptyFieldsDoesNotSubmit(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendSpecialKey(m, tea.KeyEnter) // open create form
	m = sendSpecialKey(m, tea.KeyEnter) // try to submit with empty inputs
	_, _, ok := m.ShouldCreate()
	if ok {
		t.Error("expected ShouldCreate()=false when both fields are empty")
	}
}

// ── View rendering ────────────────────────────────────────────────────────

func TestWorktreeModel_ViewListingContainsWorktrees(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	v := m.View()
	for _, branch := range []string{"feat/login", "fix/bug"} {
		if !containsStr(v, branch) {
			t.Errorf("view should contain branch %q", branch)
		}
	}
	if !containsStr(v, "New worktree") {
		t.Error("view should contain 'New worktree' option")
	}
}

func TestWorktreeModel_ViewCreatingContainsFormHints(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendSpecialKey(m, tea.KeyEnter) // switch to creating state
	v := m.View()
	if !containsStr(v, "Branch") {
		t.Error("create view should contain 'Branch' label")
	}
	if !containsStr(v, "Path") {
		t.Error("create view should contain 'Path' label")
	}
}

func TestWorktreeModel_ViewConfirmingContainsWorktreeInfo(t *testing.T) {
	m := newTestWorktreeModel(testWorktrees())
	m = sendKey(m, 'j')
	m = sendKey(m, 'j') // select feat/login
	m = sendKey(m, 'x')
	v := m.View()
	if !containsStr(v, "feat/login") {
		t.Errorf("confirm view should contain branch name")
	}
}

// ── TUI model integration ─────────────────────────────────────────────────

func TestTUI_WorktreeKeyNoGitRootStaysNormal(t *testing.T) {
	// Sessions with no GitRoot set — pressing 'w' should be a no-op.
	sessions := testSessions()
	m, fw := newTestModel(t, sessions)
	defer fw.Close()

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 50))
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	fm := final.(Model)

	if fm.mode != ModeNormal {
		t.Errorf("expected ModeNormal when no GitRoot, got %d", fm.mode)
	}
}

func TestTUI_WorktreeModeEscReturnsToNormal(t *testing.T) {
	// Inject a WorktreeModel directly to bypass the git exec call.
	sessions := testSessions()
	m, fw := newTestModel(t, sessions)
	defer fw.Close()

	wm := NewWorktreeModel(testWorktrees(), "/home/user/repo", sessions, m.width, m.height)
	m.worktreeModel = &wm
	m.mode = ModeWorktree

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 50))
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	fm := final.(Model)

	if fm.mode != ModeNormal {
		t.Errorf("expected ModeNormal after esc, got %d", fm.mode)
	}
	if fm.worktreeModel != nil {
		t.Error("expected worktreeModel=nil after cancel")
	}
}

func TestTUI_WorktreeModeViewRendered(t *testing.T) {
	// With a WorktreeModel injected, View() should delegate to the worktree panel.
	sessions := testSessions()
	m, fw := newTestModel(t, sessions)
	defer fw.Close()

	wm := NewWorktreeModel(testWorktrees(), "/home/user/repo", sessions, m.width, m.height)
	m.worktreeModel = &wm
	m.mode = ModeWorktree

	v := m.View()
	if !containsStr(v, "feat/login") {
		t.Errorf("expected View() to contain 'feat/login' when in ModeWorktree, got:\n%s", v)
	}
	if !containsStr(v, "fix/bug") {
		t.Errorf("expected View() to contain 'fix/bug' when in ModeWorktree, got:\n%s", v)
	}
}

// containsStr is a string-based helper (complements contains() for []byte).
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}
