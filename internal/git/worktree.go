// Package git provides git worktree and branch management.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree.
type Worktree struct {
	Path   string
	Branch string
	Commit string
}

// Manager handles git worktree operations.
type Manager struct {
	repoRoot     string
	worktreeBase string
}

// NewManager creates a new git manager.
func NewManager(repoRoot, worktreeBase string) *Manager {
	return &Manager{
		repoRoot:     repoRoot,
		worktreeBase: worktreeBase,
	}
}

// RepoRoot returns the root of the git repository.
func (m *Manager) RepoRoot() string {
	return m.repoRoot
}

// WorktreeBase returns the base directory for worktrees.
func (m *Manager) WorktreeBase() string {
	return m.worktreeBase
}

// CreateWorktree creates a new worktree for a worker.
// If the worktree already exists, it returns the existing one.
func (m *Manager) CreateWorktree(name, baseBranch string) (*Worktree, error) {
	worktreePath := filepath.Join(m.worktreeBase, name)
	branchName := fmt.Sprintf("cosa/%s", name)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, verify it's valid and return it
		commit, _ := m.getHeadCommit(worktreePath)
		return &Worktree{
			Path:   worktreePath,
			Branch: branchName,
			Commit: commit,
		}, nil
	}

	// Create branch from base
	if baseBranch == "" {
		baseBranch = "HEAD"
	}

	// Check if branch already exists
	branchExists := m.branchExists(branchName)

	if branchExists {
		// Create worktree using existing branch
		cmd := exec.Command("git", "worktree", "add", worktreePath, branchName)
		cmd.Dir = m.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to create worktree: %s: %w", string(out), err)
		}
	} else {
		// Create worktree with new branch
		cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
		cmd.Dir = m.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to create worktree: %s: %w", string(out), err)
		}
	}

	// Get the current commit
	commit, err := m.getHeadCommit(worktreePath)
	if err != nil {
		commit = ""
	}

	return &Worktree{
		Path:   worktreePath,
		Branch: branchName,
		Commit: commit,
	}, nil
}

// RemoveWorktree removes a worktree.
func (m *Manager) RemoveWorktree(name string, force bool) error {
	worktreePath := filepath.Join(m.worktreeBase, name)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", string(out), err)
	}

	return nil
}

// ListWorktrees lists all worktrees.
func (m *Manager) ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// GetWorktree returns information about a specific worktree.
func (m *Manager) GetWorktree(name string) (*Worktree, error) {
	worktreePath := filepath.Join(m.worktreeBase, name)

	worktrees, err := m.ListWorktrees()
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Path == worktreePath {
			return &wt, nil
		}
	}

	return nil, fmt.Errorf("worktree not found: %s", name)
}

// PruneWorktrees removes stale worktree entries.
func (m *Manager) PruneWorktrees() error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to prune worktrees: %s: %w", string(out), err)
	}
	return nil
}

// GetCurrentBranch returns the current branch name.
func (m *Manager) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDefaultBranch attempts to determine the default branch (main/master).
func (m *Manager) GetDefaultBranch() string {
	// Try common default branch names first
	if m.branchExists("main") {
		return "main"
	}
	if m.branchExists("master") {
		return "master"
	}

	// Try to get the current branch name (works if we're on a named branch)
	if branch, err := m.GetCurrentBranch(); err == nil && branch != "HEAD" && branch != "" {
		return branch
	}

	// Last resort: return "master" as the conventional default
	// This is better than "HEAD" which doesn't work for diffs
	return "master"
}

func (m *Manager) branchExists(name string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+name)
	cmd.Dir = m.repoRoot
	return cmd.Run() == nil
}

func (m *Manager) getHeadCommit(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsGitRepo checks if a directory is a git repository.
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode()&os.ModeSymlink != 0
}

// FindRepoRoot finds the root of the git repository.
// Note: If called from a worktree, this returns the worktree path, not the main repo.
// Use FindMainRepoRoot if you need the main repository root.
func FindRepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FindMainRepoRoot finds the root of the main git repository, even when called from a worktree.
// This is useful for finding shared resources like .cosa directory.
func FindMainRepoRoot(path string) (string, error) {
	// Get the common git directory (shared between main repo and worktrees)
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	gitCommonDir := strings.TrimSpace(string(out))

	// The common dir is either ".git" (main repo) or an absolute path to main repo's .git
	// In either case, the repo root is the parent directory
	if gitCommonDir == ".git" {
		// We're in the main repo, use standard method
		return FindRepoRoot(path)
	}

	// gitCommonDir is absolute path like /path/to/repo/.git
	// The repo root is its parent
	return filepath.Dir(gitCommonDir), nil
}

// CreateJobWorktree creates a new worktree for a specific job.
// The worktree is placed in a "jobs" subdirectory under the worktree base.
// The branch name is cosa/job/<shortJobID> based off the given base branch.
func (m *Manager) CreateJobWorktree(jobID, baseBranch string) (*Worktree, error) {
	// Use first 8 chars of job ID for shorter branch names
	shortID := jobID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	jobsDir := filepath.Join(m.worktreeBase, "jobs")
	worktreePath := filepath.Join(jobsDir, shortID)
	branchName := fmt.Sprintf("cosa/job/%s", shortID)

	// Ensure the jobs directory exists
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create jobs directory: %w", err)
	}

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		commit, _ := m.getHeadCommit(worktreePath)
		return &Worktree{
			Path:   worktreePath,
			Branch: branchName,
			Commit: commit,
		}, nil
	}

	// Create branch from base
	if baseBranch == "" {
		baseBranch = "HEAD"
	}

	// Check if branch already exists
	branchExists := m.branchExists(branchName)

	if branchExists {
		cmd := exec.Command("git", "worktree", "add", worktreePath, branchName)
		cmd.Dir = m.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to create job worktree: %s: %w", string(out), err)
		}
	} else {
		cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
		cmd.Dir = m.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to create job worktree: %s: %w", string(out), err)
		}
	}

	commit, err := m.getHeadCommit(worktreePath)
	if err != nil {
		commit = ""
	}

	return &Worktree{
		Path:   worktreePath,
		Branch: branchName,
		Commit: commit,
	}, nil
}

// RemoveJobWorktree removes a job's worktree by job ID.
func (m *Manager) RemoveJobWorktree(jobID string, force bool) error {
	shortID := jobID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	jobsDir := filepath.Join(m.worktreeBase, "jobs")
	worktreePath := filepath.Join(jobsDir, shortID)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove job worktree: %s: %w", string(out), err)
	}

	return nil
}

// DeleteBranch deletes a local branch.
func (m *Manager) DeleteBranch(branchName string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := exec.Command("git", "branch", flag, branchName)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch: %s: %w", string(out), err)
	}

	return nil
}

// GetJobWorktreePath returns the path for a job's worktree.
func (m *Manager) GetJobWorktreePath(jobID string) string {
	shortID := jobID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return filepath.Join(m.worktreeBase, "jobs", shortID)
}

// GetJobBranchName returns the branch name for a job.
func (m *Manager) GetJobBranchName(jobID string) string {
	shortID := jobID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("cosa/job/%s", shortID)
}
