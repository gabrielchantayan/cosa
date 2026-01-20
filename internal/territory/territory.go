// Package territory manages the Cosa workspace (territory) for a project.
package territory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cosa/internal/git"
)

const (
	// DirName is the name of the territory directory.
	DirName = ".cosa"

	// ConfigFile is the territory configuration file.
	ConfigFile = "territory.json"

	// WorktreesDir is the directory for worktrees.
	WorktreesDir = "worktrees"

	// StateFile stores runtime state.
	StateFile = "state.json"
)

// Territory represents a Cosa workspace for a project.
type Territory struct {
	Path       string    `json:"path"`        // Project root path
	RepoRoot   string    `json:"repo_root"`   // Git repository root
	BaseBranch string    `json:"base_branch"` // Default branch for new worktrees
	CreatedAt  time.Time `json:"created_at"`
	Config     Config    `json:"config"`

	gitManager *git.Manager
}

// Config contains territory-specific configuration.
type Config struct {
	// DefaultPriority for new jobs.
	DefaultPriority int `json:"default_priority"`

	// AutoReview enables automatic code review.
	AutoReview bool `json:"auto_review"`

	// TestCommand to run before review.
	TestCommand string `json:"test_command,omitempty"`

	// BuildCommand to run before review.
	BuildCommand string `json:"build_command,omitempty"`

	// DevBranch is the development/staging branch where workers merge their work.
	// If empty, workers merge directly to BaseBranch (main/master).
	DevBranch string `json:"dev_branch,omitempty"`
}

// Init initializes a new territory in the given directory.
func Init(projectPath string) (*Territory, error) {
	// Find git repository root
	repoRoot, err := git.FindRepoRoot(projectPath)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	territoryPath := filepath.Join(repoRoot, DirName)

	// Check if already initialized
	if _, err := os.Stat(territoryPath); err == nil {
		return nil, fmt.Errorf("territory already initialized at %s", territoryPath)
	}

	// Create territory directory structure
	worktreesPath := filepath.Join(territoryPath, WorktreesDir)
	if err := os.MkdirAll(worktreesPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create territory directory: %w", err)
	}

	// Determine base branch
	gitMgr := git.NewManager(repoRoot, worktreesPath)
	baseBranch := gitMgr.GetDefaultBranch()

	territory := &Territory{
		Path:       territoryPath,
		RepoRoot:   repoRoot,
		BaseBranch: baseBranch,
		CreatedAt:  time.Now(),
		Config: Config{
			DefaultPriority: 3,
			AutoReview:      true,
		},
		gitManager: gitMgr,
	}

	// Save configuration
	if err := territory.Save(); err != nil {
		return nil, fmt.Errorf("failed to save territory config: %w", err)
	}

	// Add .cosa to .gitignore if not already there
	addToGitignore(repoRoot, DirName)

	return territory, nil
}

// Load loads an existing territory.
// This function works correctly even when called from within a worktree.
func Load(projectPath string) (*Territory, error) {
	// Use FindMainRepoRoot to handle worktrees correctly
	repoRoot, err := git.FindMainRepoRoot(projectPath)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	territoryPath := filepath.Join(repoRoot, DirName)
	configPath := filepath.Join(territoryPath, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("territory not initialized (run 'cosa territory init')")
		}
		return nil, fmt.Errorf("failed to read territory config: %w", err)
	}

	var territory Territory
	if err := json.Unmarshal(data, &territory); err != nil {
		return nil, fmt.Errorf("failed to parse territory config: %w", err)
	}

	worktreesPath := filepath.Join(territoryPath, WorktreesDir)
	territory.gitManager = git.NewManager(repoRoot, worktreesPath)

	return &territory, nil
}

// Save writes the territory configuration to disk.
func (t *Territory) Save() error {
	configPath := filepath.Join(t.Path, ConfigFile)
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// WorktreesPath returns the path to the worktrees directory.
func (t *Territory) WorktreesPath() string {
	return filepath.Join(t.Path, WorktreesDir)
}

// GitManager returns the git manager.
func (t *Territory) GitManager() *git.Manager {
	return t.gitManager
}

// CreateWorkerWorktree creates a worktree for a worker.
func (t *Territory) CreateWorkerWorktree(workerName string) (*git.Worktree, error) {
	return t.gitManager.CreateWorktree(workerName, t.BaseBranch)
}

// RemoveWorkerWorktree removes a worker's worktree.
func (t *Territory) RemoveWorkerWorktree(workerName string, force bool) error {
	return t.gitManager.RemoveWorktree(workerName, force)
}

// MergeTargetBranch returns the branch where workers should merge their work.
// Returns DevBranch if configured, otherwise BaseBranch.
func (t *Territory) MergeTargetBranch() string {
	if t.Config.DevBranch != "" {
		return t.Config.DevBranch
	}
	return t.BaseBranch
}

// SetDevBranch sets the development/staging branch for worker merges.
func (t *Territory) SetDevBranch(branch string) error {
	t.Config.DevBranch = branch
	return t.Save()
}

// ClearDevBranch removes the development branch configuration,
// causing workers to merge directly to the base branch.
func (t *Territory) ClearDevBranch() error {
	t.Config.DevBranch = ""
	return t.Save()
}

// Exists checks if a territory exists at the given path.
// This function works correctly even when called from within a worktree.
func Exists(projectPath string) bool {
	// Use FindMainRepoRoot to handle worktrees correctly
	repoRoot, err := git.FindMainRepoRoot(projectPath)
	if err != nil {
		return false
	}
	territoryPath := filepath.Join(repoRoot, DirName, ConfigFile)
	_, err = os.Stat(territoryPath)
	return err == nil
}

func addToGitignore(repoRoot, pattern string) {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")

	// Read existing .gitignore
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	// Check if pattern already exists
	lines := string(content)
	for _, line := range filepath.SplitList(lines) {
		if line == pattern {
			return
		}
	}

	// Append pattern
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString(pattern + "\n")
}
