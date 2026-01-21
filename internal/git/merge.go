// Package git provides git worktree and branch management.
package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// branchNamePattern defines the allowed characters in git branch names.
// Allows alphanumeric, hyphens, underscores, forward slashes, and dots.
// Disallows: spaces, shell metacharacters, and names starting with - (flag injection).
var branchNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// ValidateBranchName checks if a branch name matches the allowed pattern.
// Returns an error if the branch name is invalid or potentially dangerous.
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("branch name too long (max 255 characters)")
	}
	if !branchNamePattern.MatchString(name) {
		return fmt.Errorf("invalid branch name: must start with alphanumeric and contain only alphanumeric, hyphens, underscores, dots, or forward slashes")
	}
	// Additional checks for git-specific invalid patterns
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid branch name: cannot contain '..'")
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("invalid branch name: cannot end with '.lock'")
	}
	if strings.HasSuffix(name, "/") {
		return fmt.Errorf("invalid branch name: cannot end with '/'")
	}
	return nil
}

// DiffResult contains the result of a git diff operation.
type DiffResult struct {
	Diff        string   // The diff content
	FilesChanged []string // List of changed files
	Additions    int      // Number of lines added
	Deletions    int      // Number of lines deleted
}

// MergeResult contains the result of a git merge operation.
type MergeResult struct {
	Success   bool
	MergeCommit string // The merge commit hash
	Message   string
}

// GetDiff returns the diff between a worktree's current state and the base branch.
func (m *Manager) GetDiff(worktreePath, baseBranch string) (*DiffResult, error) {
	// Validate branch name to prevent command injection
	if err := ValidateBranchName(baseBranch); err != nil {
		return nil, fmt.Errorf("invalid base branch: %w", err)
	}

	// Get the diff content
	// Use -- to separate revision range from paths
	cmd := exec.Command("git", "diff", "--", baseBranch+"...HEAD")
	cmd.Dir = worktreePath
	diffOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Get list of changed files
	cmd = exec.Command("git", "diff", "--name-only", "--", baseBranch+"...HEAD")
	cmd.Dir = worktreePath
	filesOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := []string{}
	for _, f := range strings.Split(strings.TrimSpace(string(filesOut)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}

	// Get stats
	cmd = exec.Command("git", "diff", "--numstat", "--", baseBranch+"...HEAD")
	cmd.Dir = worktreePath
	statsOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get diff stats: %w", err)
	}

	additions, deletions := parseDiffStats(string(statsOut))

	return &DiffResult{
		Diff:         string(diffOut),
		FilesChanged: files,
		Additions:    additions,
		Deletions:    deletions,
	}, nil
}

// Merge merges a worker branch into the base branch.
func (m *Manager) Merge(workerBranch, baseBranch string) (*MergeResult, error) {
	// Validate branch names to prevent command injection
	if err := ValidateBranchName(workerBranch); err != nil {
		return nil, fmt.Errorf("invalid worker branch: %w", err)
	}
	if err := ValidateBranchName(baseBranch); err != nil {
		return nil, fmt.Errorf("invalid base branch: %w", err)
	}

	// First, checkout the base branch
	// Use -- to separate options from branch name
	cmd := exec.Command("git", "checkout", "--", baseBranch)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to checkout base branch: %s: %w", string(out), err)
	}

	// Merge the worker branch with --no-ff to always create a merge commit
	// Use -- to separate options from branch name
	cmd = exec.Command("git", "merge", "--no-ff", "-m", fmt.Sprintf("Merge branch '%s'", workerBranch), "--", workerBranch)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// Check if it's a conflict
		if strings.Contains(string(out), "CONFLICT") {
			return &MergeResult{
				Success: false,
				Message: "Merge conflict detected",
			}, nil
		}
		return nil, fmt.Errorf("failed to merge: %s: %w", string(out), err)
	}

	// Get the merge commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = m.repoRoot
	commitOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get merge commit: %w", err)
	}

	return &MergeResult{
		Success:     true,
		MergeCommit: strings.TrimSpace(string(commitOut)),
		Message:     "Merge successful",
	}, nil
}

// HasConflicts checks if merging the worker branch would cause conflicts.
func (m *Manager) HasConflicts(workerBranch, baseBranch string) (bool, []string, error) {
	// Validate branch names to prevent command injection
	if err := ValidateBranchName(workerBranch); err != nil {
		return false, nil, fmt.Errorf("invalid worker branch: %w", err)
	}
	if err := ValidateBranchName(baseBranch); err != nil {
		return false, nil, fmt.Errorf("invalid base branch: %w", err)
	}

	// Use git merge-tree to check for conflicts without actually merging
	// First get the merge base
	// Use -- to separate options from commit refs
	cmd := exec.Command("git", "merge-base", "--", baseBranch, workerBranch)
	cmd.Dir = m.repoRoot
	baseOut, err := cmd.Output()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get merge base: %w", err)
	}
	mergeBase := strings.TrimSpace(string(baseOut))

	// Use merge-tree to simulate the merge
	// mergeBase is a commit hash from git output, so it's safe
	cmd = exec.Command("git", "merge-tree", "--", mergeBase, baseBranch, workerBranch)
	cmd.Dir = m.repoRoot
	out, _ := cmd.Output()

	// Look for conflict markers in the output
	conflicts := []string{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "<<<<<") || strings.HasPrefix(line, "+<<<<<<<") {
			conflicts = append(conflicts, line)
		}
	}

	return len(conflicts) > 0, conflicts, nil
}

// AbortMerge aborts an in-progress merge.
func (m *Manager) AbortMerge() error {
	cmd := exec.Command("git", "merge", "--abort")
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to abort merge: %s: %w", string(out), err)
	}
	return nil
}

// GetWorktreeBranch returns the current branch of a worktree.
func (m *Manager) GetWorktreeBranch(worktreePath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseDiffStats parses git diff --numstat output and returns total additions/deletions.
func parseDiffStats(stats string) (additions, deletions int) {
	for _, line := range strings.Split(stats, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			var a, d int
			fmt.Sscanf(fields[0], "%d", &a)
			fmt.Sscanf(fields[1], "%d", &d)
			additions += a
			deletions += d
		}
	}
	return
}
