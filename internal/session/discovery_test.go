package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shnupta/herd/internal/tmux"
)

func noBranch(string) string { return "" }
func noRoot(string) string   { return "" }

func TestBuildSessionsEmpty(t *testing.T) {
	sessions := buildSessions(nil, noBranch, noRoot)
	if sessions != nil {
		t.Errorf("buildSessions(nil) = %v, want nil", sessions)
	}
}

func TestBuildSessionsFiltersNonClaude(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "bash"},
		{ID: "%2", CurrentCmd: "vim"},
		{ID: "%3", CurrentCmd: "zsh"},
	}
	sessions := buildSessions(panes, noBranch, noRoot)
	if len(sessions) != 0 {
		t.Errorf("buildSessions with non-claude panes = %d sessions, want 0", len(sessions))
	}
}

func TestBuildSessionsWithVersionString(t *testing.T) {
	panes := []tmux.Pane{
		{
			ID:          "%5",
			SessionName: "mysession",
			WindowIndex: 1,
			PaneIndex:   0,
			CurrentPath: "/home/user/project",
			CurrentCmd:  "2.1.47",
		},
	}
	sessions := buildSessions(panes, func(dir string) string {
		return "main"
	}, noRoot)

	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.TmuxPane != "%5" {
		t.Errorf("TmuxPane = %q, want %%5", s.TmuxPane)
	}
	if s.TmuxSession != "mysession" {
		t.Errorf("TmuxSession = %q, want mysession", s.TmuxSession)
	}
	if s.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath = %q, want /home/user/project", s.ProjectPath)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want main", s.GitBranch)
	}
	if s.State != StateUnknown {
		t.Errorf("State = %v, want StateUnknown", s.State)
	}
	if s.WindowIndex != 1 {
		t.Errorf("WindowIndex = %d, want 1", s.WindowIndex)
	}
	if s.PaneIndex != 0 {
		t.Errorf("PaneIndex = %d, want 0", s.PaneIndex)
	}
}

func TestBuildSessionsWithClaudeCommand(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%10", CurrentCmd: "claude", CurrentPath: "/work"},
		{ID: "%11", CurrentCmd: "bash", CurrentPath: "/work"},
	}
	sessions := buildSessions(panes, noBranch, noRoot)
	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	if sessions[0].TmuxPane != "%10" {
		t.Errorf("TmuxPane = %q, want %%10", sessions[0].TmuxPane)
	}
}

func TestBuildSessionsMixedPanes(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "3.0.0"},
		{ID: "%2", CurrentCmd: "bash"},
		{ID: "%3", CurrentCmd: "claude"},
	}
	sessions := buildSessions(panes, noBranch, noRoot)
	if len(sessions) != 2 {
		t.Errorf("buildSessions = %d sessions, want 2 (version + claude)", len(sessions))
	}
}

func TestBuildSessionsWithGitRoot(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "claude", CurrentPath: "/home/user/project/src"},
	}
	sessions := buildSessions(panes, noBranch, func(dir string) string {
		return "/home/user/project"
	})
	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	if sessions[0].GitRoot != "/home/user/project" {
		t.Errorf("GitRoot = %q, want /home/user/project", sessions[0].GitRoot)
	}
}

func TestBuildSessionsAllClaude(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "claude", CurrentPath: "/a"},
		{ID: "%2", CurrentCmd: "2.1.47", CurrentPath: "/b"},
		{ID: "%3", CurrentCmd: "Claude", CurrentPath: "/c"},
	}
	sessions := buildSessions(panes, noBranch, noRoot)
	if len(sessions) != 3 {
		t.Errorf("buildSessions = %d sessions, want 3", len(sessions))
	}
}

func TestBuildSessionsBranchFnReceivesCorrectDir(t *testing.T) {
	panes := []tmux.Pane{
		{ID: "%1", CurrentCmd: "claude", CurrentPath: "/specific/path"},
	}
	var calledWith string
	sessions := buildSessions(panes, func(dir string) string {
		calledWith = dir
		return "feature-branch"
	}, noRoot)
	if calledWith != "/specific/path" {
		t.Errorf("branchFn called with %q, want /specific/path", calledWith)
	}
	if len(sessions) != 1 {
		t.Fatalf("buildSessions = %d sessions, want 1", len(sessions))
	}
	if sessions[0].GitBranch != "feature-branch" {
		t.Errorf("GitBranch = %q, want feature-branch", sessions[0].GitBranch)
	}
}

// initTestRepo creates a minimal git repo in a temp directory and returns the path.
// The repo has one commit so that HEAD and branch references are valid.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGitBranchReturnsMainOrMaster(t *testing.T) {
	dir := initTestRepo(t)

	branch := gitBranch(dir)
	// Default branch varies by git config; it's usually "main" or "master".
	if branch == "" {
		t.Error("gitBranch returned empty for a valid git repo with a commit")
	}
}

func TestGitBranchCustomBranch(t *testing.T) {
	dir := initTestRepo(t)

	cmd := exec.Command("git", "-C", dir, "checkout", "-b", "feature-xyz")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b: %v\n%s", err, out)
	}

	branch := gitBranch(dir)
	if branch != "feature-xyz" {
		t.Errorf("gitBranch = %q, want feature-xyz", branch)
	}
}

func TestGitBranchDetachedHead(t *testing.T) {
	dir := initTestRepo(t)

	cmd := exec.Command("git", "-C", dir, "checkout", "--detach", "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach: %v\n%s", err, out)
	}

	branch := gitBranch(dir)
	if branch != "" {
		t.Errorf("gitBranch on detached HEAD = %q, want empty", branch)
	}
}

func TestGitBranchNonGitDir(t *testing.T) {
	dir := t.TempDir() // not a git repo
	branch := gitBranch(dir)
	if branch != "" {
		t.Errorf("gitBranch on non-git dir = %q, want empty", branch)
	}
}

func TestGitBranchNonexistentDir(t *testing.T) {
	branch := gitBranch("/nonexistent/path/unlikely/to/exist")
	if branch != "" {
		t.Errorf("gitBranch on nonexistent dir = %q, want empty", branch)
	}
}

func TestGitRootReturnsRepoRoot(t *testing.T) {
	dir := initTestRepo(t)

	root := gitRoot(dir)
	// Resolve symlinks — t.TempDir() may involve /var -> /private/var on macOS.
	expected, _ := filepath.EvalSymlinks(dir)
	if root != expected {
		t.Errorf("gitRoot = %q, want %q", root, expected)
	}
}

func TestGitRootFromSubdirectory(t *testing.T) {
	dir := initTestRepo(t)

	sub := filepath.Join(dir, "deep", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	root := gitRoot(sub)
	expected, _ := filepath.EvalSymlinks(dir)
	if root != expected {
		t.Errorf("gitRoot from subdirectory = %q, want %q", root, expected)
	}
}

func TestGitRootNonGitDir(t *testing.T) {
	dir := t.TempDir()
	root := gitRoot(dir)
	if root != "" {
		t.Errorf("gitRoot on non-git dir = %q, want empty", root)
	}
}

func TestGitRootNonexistentDir(t *testing.T) {
	root := gitRoot("/nonexistent/path/unlikely/to/exist")
	if root != "" {
		t.Errorf("gitRoot on nonexistent dir = %q, want empty", root)
	}
}

// --- Discover() tests using MockClient ---

func TestDiscoverListPanesError(t *testing.T) {
	mock := &tmux.MockClient{
		ListPanesErr: errors.New("tmux not running"),
	}
	sessions, err := Discover(mock)
	if err == nil {
		t.Fatal("Discover should return error when ListPanes fails")
	}
	if sessions != nil {
		t.Errorf("Discover returned %d sessions, want nil", len(sessions))
	}
}

func TestDiscoverEmptyPanes(t *testing.T) {
	mock := &tmux.MockClient{}
	sessions, err := Discover(mock)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Discover returned %d sessions, want 0", len(sessions))
	}
}

func TestDiscoverFiltersClaude(t *testing.T) {
	mock := &tmux.MockClient{
		Panes: []tmux.Pane{
			{ID: "%1", CurrentCmd: "bash", CurrentPath: "/home"},
			{ID: "%2", CurrentCmd: "2.1.47", CurrentPath: "/project/a"},
			{ID: "%3", CurrentCmd: "vim", CurrentPath: "/home"},
			{ID: "%4", CurrentCmd: "claude", CurrentPath: "/project/b"},
			{ID: "%5", CurrentCmd: "zsh", CurrentPath: "/home"},
		},
	}
	sessions, err := Discover(mock)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("Discover returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].TmuxPane != "%2" {
		t.Errorf("sessions[0].TmuxPane = %q, want %%2", sessions[0].TmuxPane)
	}
	if sessions[1].TmuxPane != "%4" {
		t.Errorf("sessions[1].TmuxPane = %q, want %%4", sessions[1].TmuxPane)
	}
}

func TestDiscoverPopulatesSessionFields(t *testing.T) {
	dir := initTestRepo(t)

	mock := &tmux.MockClient{
		Panes: []tmux.Pane{
			{
				ID:          "%7",
				SessionName: "dev",
				WindowIndex: 3,
				PaneIndex:   1,
				CurrentCmd:  "claude",
				CurrentPath: dir,
			},
		},
	}
	sessions, err := Discover(mock)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("Discover returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.TmuxPane != "%7" {
		t.Errorf("TmuxPane = %q, want %%7", s.TmuxPane)
	}
	if s.TmuxSession != "dev" {
		t.Errorf("TmuxSession = %q, want dev", s.TmuxSession)
	}
	if s.WindowIndex != 3 {
		t.Errorf("WindowIndex = %d, want 3", s.WindowIndex)
	}
	if s.PaneIndex != 1 {
		t.Errorf("PaneIndex = %d, want 1", s.PaneIndex)
	}
	if s.ProjectPath != dir {
		t.Errorf("ProjectPath = %q, want %q", s.ProjectPath, dir)
	}
	if s.State != StateUnknown {
		t.Errorf("State = %v, want StateUnknown", s.State)
	}
	// Git branch should be non-empty since we pointed at a real repo.
	if s.GitBranch == "" {
		t.Error("GitBranch is empty, expected a branch name for a valid repo")
	}
	// Git root should resolve to the repo dir.
	expectedRoot, _ := filepath.EvalSymlinks(dir)
	if s.GitRoot != expectedRoot {
		t.Errorf("GitRoot = %q, want %q", s.GitRoot, expectedRoot)
	}
}

func TestDiscoverNonGitPath(t *testing.T) {
	dir := t.TempDir() // not a git repo

	mock := &tmux.MockClient{
		Panes: []tmux.Pane{
			{ID: "%9", CurrentCmd: "3.0.0", CurrentPath: dir},
		},
	}
	sessions, err := Discover(mock)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("Discover returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].GitBranch != "" {
		t.Errorf("GitBranch = %q, want empty for non-git dir", sessions[0].GitBranch)
	}
	if sessions[0].GitRoot != "" {
		t.Errorf("GitRoot = %q, want empty for non-git dir", sessions[0].GitRoot)
	}
}

func TestDiscoverBranchCaching(t *testing.T) {
	// Two panes with the same CurrentPath should reuse cached git results.
	// We can't directly observe the cache, but we verify both sessions
	// get the same branch value from a real repo.
	dir := initTestRepo(t)

	mock := &tmux.MockClient{
		Panes: []tmux.Pane{
			{ID: "%1", CurrentCmd: "claude", CurrentPath: dir},
			{ID: "%2", CurrentCmd: "2.0.0", CurrentPath: dir},
		},
	}
	sessions, err := Discover(mock)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("Discover returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].GitBranch != sessions[1].GitBranch {
		t.Errorf("branch mismatch: %q vs %q", sessions[0].GitBranch, sessions[1].GitBranch)
	}
	if sessions[0].GitRoot != sessions[1].GitRoot {
		t.Errorf("root mismatch: %q vs %q", sessions[0].GitRoot, sessions[1].GitRoot)
	}
}
