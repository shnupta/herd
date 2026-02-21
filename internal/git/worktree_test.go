package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWorktrees_MainOnly(t *testing.T) {
	input := []byte("worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\n")
	wts := parseWorktrees(input)
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if wts[0].Path != "/home/user/repo" {
		t.Errorf("unexpected path: %q", wts[0].Path)
	}
	if wts[0].Branch != "main" {
		t.Errorf("unexpected branch: %q", wts[0].Branch)
	}
	if !wts[0].IsMain {
		t.Error("first worktree should be marked as main")
	}
}

func TestParseWorktrees_MultipleWorktrees(t *testing.T) {
	input := []byte(strings.Join([]string{
		"worktree /home/user/repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /home/user/worktrees/repo-feat-payments",
		"HEAD def456",
		"branch refs/heads/feat/payments",
		"",
	}, "\n"))

	wts := parseWorktrees(input)
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	if !wts[0].IsMain {
		t.Error("first worktree should be main")
	}
	if wts[1].IsMain {
		t.Error("second worktree should not be main")
	}
	if wts[1].Branch != "feat/payments" {
		t.Errorf("unexpected branch: %q", wts[1].Branch)
	}
	if wts[1].Path != "/home/user/worktrees/repo-feat-payments" {
		t.Errorf("unexpected path: %q", wts[1].Path)
	}
}

func TestParseWorktrees_DetachedHead(t *testing.T) {
	input := []byte("worktree /tmp/detached\nHEAD abc123\ndetached\n\n")
	wts := parseWorktrees(input)
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if wts[0].Branch != "" {
		t.Errorf("detached HEAD should have empty branch, got %q", wts[0].Branch)
	}
}

func TestParseWorktrees_Empty(t *testing.T) {
	wts := parseWorktrees([]byte(""))
	if len(wts) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(wts))
	}
}

func TestDefaultWorktreePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := DefaultWorktreePath("/dev/myrepo", "feat/payments")
	want := filepath.Join(home, ".herd", "worktrees", "myrepo-feat-payments")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitiseBranch(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"main", "main"},
		{"feat/payments", "feat-payments"},
		{"fix/some bug", "fix-some-bug"},
		{"release\\1.0", "release-1.0"},
		{"feat:colon", "feat-colon"},
	}
	for _, tc := range cases {
		got := sanitiseBranch(tc.in)
		if got != tc.want {
			t.Errorf("sanitiseBranch(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
