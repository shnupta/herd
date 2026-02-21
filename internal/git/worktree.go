package git

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a single git worktree.
type Worktree struct {
	Path   string
	Branch string // "" if detached HEAD
	IsMain bool
	IsBare bool
}

// ListWorktrees returns all worktrees for the given repo root.
func ListWorktrees(repoRoot string) ([]Worktree, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

func parseWorktrees(data []byte) []Worktree {
	var worktrees []Worktree
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var current *Worktree
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &Worktree{}
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "bare":
			current.IsBare = true
		}
	}
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	// The first worktree is always the main one.
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}

	return worktrees
}

// AddWorktree creates a new git worktree at path on the given branch.
// If the branch doesn't exist it creates it; if it already exists, checks it out.
func AddWorktree(repoRoot, path, branch string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, path)
	if err := cmd.Run(); err != nil {
		// Branch may already exist — try checking it out directly.
		cmd = exec.Command("git", "-C", repoRoot, "worktree", "add", path, branch)
		return cmd.Run()
	}
	return nil
}

// DefaultWorktreePath returns the conventional path for a new worktree.
// e.g. repoRoot=/dev/herd, branch=feat/payments → ~/.herd/worktrees/herd-feat-payments
func DefaultWorktreePath(repoRoot, branch string) string {
	home, _ := os.UserHomeDir()
	base := filepath.Base(repoRoot)
	return filepath.Join(home, ".herd", "worktrees", base+"-"+sanitiseBranch(branch))
}

// RemoveWorktree removes the git worktree at path within the given repo.
func RemoveWorktree(repoRoot, path string) error {
	return exec.Command("git", "-C", repoRoot, "worktree", "remove", path).Run()
}

// sanitiseBranch replaces path-unsafe characters with "-".
func sanitiseBranch(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
