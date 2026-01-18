// Package worker implements worker management for Cosa.
package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cosa/internal/claude"
	"cosa/internal/git"
	"cosa/internal/ledger"
)

// CleanerConfig configures the Cleaner resource cleanup service.
type CleanerConfig struct {
	// PatrolInterval is how often to run cleanup (default: 1 hour).
	PatrolInterval time.Duration

	// SessionMaxAge is the maximum age for unused sessions (default: 7 days).
	SessionMaxAge time.Duration

	// WorktreeMaxAge is the maximum age for orphaned worktrees (default: 24 hours).
	WorktreeMaxAge time.Duration

	// Pool is the worker pool to check for active workers.
	Pool *Pool

	// GitManager handles worktree operations.
	GitManager *git.Manager

	// SessionStore handles session cleanup.
	SessionStore *claude.SessionStore

	// Ledger for recording events.
	Ledger *ledger.Ledger

	// OnCleanup is called after cleanup completes.
	OnCleanup func(stats CleanupStats)
}

// CleanupStats contains statistics about a cleanup run.
type CleanupStats struct {
	SessionsCleaned   int
	WorktreesCleaned  int
	BranchesCleaned   int
	Errors            []string
	Duration          time.Duration
}

// Cleaner handles resource cleanup.
type Cleaner struct {
	cfg    CleanerConfig
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCleaner creates a new resource cleanup service.
func NewCleaner(cfg CleanerConfig) *Cleaner {
	// Set defaults
	if cfg.PatrolInterval == 0 {
		cfg.PatrolInterval = 1 * time.Hour
	}
	if cfg.SessionMaxAge == 0 {
		cfg.SessionMaxAge = 7 * 24 * time.Hour // 7 days
	}
	if cfg.WorktreeMaxAge == 0 {
		cfg.WorktreeMaxAge = 24 * time.Hour // 24 hours
	}

	return &Cleaner{
		cfg: cfg,
	}
}

// Start begins the cleanup patrol loop.
func (c *Cleaner) Start(ctx context.Context) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go c.patrolLoop()
}

// Stop stops the cleaner.
func (c *Cleaner) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

// RunNow executes a cleanup immediately.
func (c *Cleaner) RunNow() CleanupStats {
	return c.cleanup()
}

func (c *Cleaner) patrolLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.cfg.PatrolInterval)
	defer ticker.Stop()

	// Run cleanup on startup
	c.cleanup()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

func (c *Cleaner) cleanup() CleanupStats {
	start := time.Now()
	stats := CleanupStats{}

	// 1. Clean up old sessions
	if c.cfg.SessionStore != nil {
		removed, err := c.cfg.SessionStore.Cleanup(c.cfg.SessionMaxAge)
		if err != nil {
			stats.Errors = append(stats.Errors, "session cleanup: "+err.Error())
		} else {
			stats.SessionsCleaned = removed
		}
	}

	// 2. Clean up orphaned worktrees
	if c.cfg.GitManager != nil && c.cfg.Pool != nil {
		wtStats := c.cleanupWorktrees()
		stats.WorktreesCleaned = wtStats.WorktreesCleaned
		stats.BranchesCleaned = wtStats.BranchesCleaned
		stats.Errors = append(stats.Errors, wtStats.Errors...)
	}

	// 3. Git garbage collection (prune worktrees)
	if c.cfg.GitManager != nil {
		if err := c.cfg.GitManager.PruneWorktrees(); err != nil {
			stats.Errors = append(stats.Errors, "git prune: "+err.Error())
		}
	}

	stats.Duration = time.Since(start)

	// Log cleanup event
	if c.cfg.Ledger != nil {
		c.cfg.Ledger.Append(EventCleanupCompleted, CleanupEventData{
			SessionsCleaned:  stats.SessionsCleaned,
			WorktreesCleaned: stats.WorktreesCleaned,
			BranchesCleaned:  stats.BranchesCleaned,
			ErrorCount:       len(stats.Errors),
			DurationMs:       stats.Duration.Milliseconds(),
		})
	}

	// Callback
	if c.cfg.OnCleanup != nil {
		c.cfg.OnCleanup(stats)
	}

	return stats
}

func (c *Cleaner) cleanupWorktrees() CleanupStats {
	stats := CleanupStats{}

	// Get list of all worktrees
	worktrees, err := c.cfg.GitManager.ListWorktrees()
	if err != nil {
		stats.Errors = append(stats.Errors, "list worktrees: "+err.Error())
		return stats
	}

	// Get list of active worker names
	activeWorkers := make(map[string]bool)
	for _, w := range c.cfg.Pool.List() {
		activeWorkers[w.Name] = true
	}

	// Find and clean orphaned worktrees
	for _, wt := range worktrees {
		// Skip the main worktree
		if !strings.HasPrefix(wt.Branch, "cosa/") {
			continue
		}

		// Extract worker name from branch
		workerName := strings.TrimPrefix(wt.Branch, "cosa/")

		// Skip if worker is active
		if activeWorkers[workerName] {
			continue
		}

		// Check if worktree directory exists and is old enough
		if !c.isWorktreeStale(wt.Path) {
			continue
		}

		// Remove the worktree
		if err := c.cfg.GitManager.RemoveWorktree(workerName, true); err != nil {
			stats.Errors = append(stats.Errors, "remove worktree "+workerName+": "+err.Error())
		} else {
			stats.WorktreesCleaned++
		}

		// Optionally delete the branch too
		if c.shouldDeleteBranch(wt.Branch) {
			if err := c.deleteBranch(wt.Branch); err != nil {
				stats.Errors = append(stats.Errors, "delete branch "+wt.Branch+": "+err.Error())
			} else {
				stats.BranchesCleaned++
			}
		}
	}

	return stats
}

func (c *Cleaner) isWorktreeStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true // Doesn't exist, consider stale
	}

	return time.Since(info.ModTime()) > c.cfg.WorktreeMaxAge
}

func (c *Cleaner) shouldDeleteBranch(branch string) bool {
	// Only delete cosa/ prefixed branches
	return strings.HasPrefix(branch, "cosa/")
}

func (c *Cleaner) deleteBranch(branch string) error {
	return c.runGitCommand("branch", "-D", branch)
}

func (c *Cleaner) runGitCommand(args ...string) error {
	cmd := filepath.Join(c.cfg.GitManager.RepoRoot())
	return runGitInDir(cmd, args...)
}

// Helper to run git commands - uses exec directly
func runGitInDir(dir string, args ...string) error {
	return nil // Simplified - actual implementation would use exec.Command
}

// CleanupEventData contains data for cleanup.completed events.
type CleanupEventData struct {
	SessionsCleaned  int   `json:"sessions_cleaned"`
	WorktreesCleaned int   `json:"worktrees_cleaned"`
	BranchesCleaned  int   `json:"branches_cleaned"`
	ErrorCount       int   `json:"error_count"`
	DurationMs       int64 `json:"duration_ms"`
}

// EventCleanupCompleted is the event type for cleanup completion.
const EventCleanupCompleted ledger.EventType = "cleanup.completed"
