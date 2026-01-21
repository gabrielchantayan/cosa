// Package job implements job management for Cosa.
package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the state of a job.
type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
	StatusReview     Status = "review"
)

// Priority levels for jobs.
const (
	PriorityLow      = 1
	PriorityNormal   = 3
	PriorityHigh     = 4
	PriorityCritical = 5
)

// Job represents a unit of work to be executed by a worker.
type Job struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    int       `json:"priority"`
	Worker      string    `json:"worker,omitempty"`
	DependsOn   []string  `json:"depends_on,omitempty"`
	Operation   string    `json:"operation,omitempty"` // Parent operation ID

	CreatedAt   time.Time  `json:"created_at"`
	QueuedAt    *time.Time `json:"queued_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Worktree for this job (created when job starts, cleaned up after merge)
	Worktree string `json:"worktree,omitempty"` // Path to job's worktree
	Branch   string `json:"branch,omitempty"`   // Branch name for this job

	// Execution details
	SessionID string `json:"session_id,omitempty"` // Claude session ID
	Error     string `json:"error,omitempty"`
	Output    string `json:"output,omitempty"`

	// Cost tracking
	TotalCost   string `json:"total_cost,omitempty"`   // Cost for this job
	TotalTokens int    `json:"total_tokens,omitempty"` // Tokens used for this job

	// Review fields
	ReviewFeedback []string `json:"review_feedback,omitempty"` // Feedback from code review
	RevisionOf     string   `json:"revision_of,omitempty"`     // ID of job this is a revision of

	mu sync.RWMutex
}

// New creates a new job.
func New(description string) *Job {
	return &Job{
		ID:          uuid.New().String(),
		Description: description,
		Status:      StatusPending,
		Priority:    PriorityNormal,
		CreatedAt:   time.Now(),
	}
}

// SetPriority sets the job priority.
func (j *Job) SetPriority(priority int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Priority = priority
}

// SetDependencies sets job dependencies.
func (j *Job) SetDependencies(deps []string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.DependsOn = deps
}

// SetRevisionOf sets the ID of the job this is a revision of.
func (j *Job) SetRevisionOf(jobID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.RevisionOf = jobID
}

// SetReviewFeedback sets the review feedback for this job.
func (j *Job) SetReviewFeedback(feedback []string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ReviewFeedback = feedback
}

// SetWorktree sets the worktree path and branch for this job.
func (j *Job) SetWorktree(worktreePath, branchName string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Worktree = worktreePath
	j.Branch = branchName
}

// GetWorktree returns the worktree path for this job.
func (j *Job) GetWorktree() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Worktree
}

// GetBranch returns the branch name for this job.
func (j *Job) GetBranch() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Branch
}

// ClearWorktree clears the worktree and branch fields (used after cleanup).
func (j *Job) ClearWorktree() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Worktree = ""
	j.Branch = ""
}

// Queue marks the job as queued.
func (j *Job) Queue() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusQueued
	now := time.Now()
	j.QueuedAt = &now
}

// Start marks the job as running.
func (j *Job) Start(workerID, sessionID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusRunning
	j.Worker = workerID
	j.SessionID = sessionID
	now := time.Now()
	j.StartedAt = &now
}

// Complete marks the job as completed.
func (j *Job) Complete(output string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusCompleted
	j.Output = output
	now := time.Now()
	j.CompletedAt = &now
}

// Fail marks the job as failed.
func (j *Job) Fail(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusFailed
	j.Error = err
	now := time.Now()
	j.CompletedAt = &now
}

// Cancel marks the job as cancelled.
func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusCancelled
	now := time.Now()
	j.CompletedAt = &now
}

// MarkForReview marks the job as ready for review.
func (j *Job) MarkForReview() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusReview
}

// Reset resets a failed or cancelled job back to pending state so it can be re-queued.
// Returns an error if the job is not in a failed or cancelled state.
func (j *Job) Reset() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status != StatusFailed && j.Status != StatusCancelled {
		return fmt.Errorf("can only reset failed or cancelled jobs, current status: %s", j.Status)
	}
	j.Status = StatusPending
	j.Error = ""
	j.Worker = ""
	j.SessionID = ""
	j.QueuedAt = nil
	j.StartedAt = nil
	j.CompletedAt = nil
	return nil
}

// GetStatus returns the current job status.
func (j *Job) GetStatus() Status {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status
}

// IsTerminal returns true if the job is in a terminal state.
func (j *Job) IsTerminal() bool {
	status := j.GetStatus()
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}

// ToJSON serializes the job to JSON.
func (j *Job) ToJSON() ([]byte, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return json.Marshal(j)
}

// Store manages jobs with optional persistence.
type Store struct {
	jobs map[string]*Job
	path string // Directory for job persistence (empty = no persistence)
	mu   sync.RWMutex
}

// NewStore creates a new in-memory job store (no persistence).
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*Job),
	}
}

// NewPersistentStore creates a job store with disk persistence.
func NewPersistentStore(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create jobs directory: %w", err)
	}

	s := &Store{
		jobs: make(map[string]*Job),
		path: path,
	}

	if err := s.loadAll(); err != nil {
		return nil, fmt.Errorf("failed to load jobs: %w", err)
	}

	return s, nil
}

// Add adds a job to the store.
func (s *Store) Add(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	if s.path != "" {
		s.saveJob(job)
	}
}

// Get retrieves a job by ID.
func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

// Remove removes a job from the store.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	if s.path != "" {
		os.Remove(s.jobFilePath(id))
	}
}

// List returns all jobs.
func (s *Store) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// ListByStatus returns jobs with a given status.
func (s *Store) ListByStatus(status Status) []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var jobs []*Job
	for _, job := range s.jobs {
		if job.GetStatus() == status {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// ListByWorker returns jobs assigned to a worker.
func (s *Store) ListByWorker(workerID string) []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var jobs []*Job
	for _, job := range s.jobs {
		job.mu.RLock()
		if job.Worker == workerID {
			jobs = append(jobs, job)
		}
		job.mu.RUnlock()
	}
	return jobs
}

// Count returns the number of jobs.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

// CountByStatus returns the number of jobs with a given status.
func (s *Store) CountByStatus(status Status) int {
	return len(s.ListByStatus(status))
}

// Save persists a job to disk (if persistence is enabled).
func (s *Store) Save(job *Job) error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveJob(job)
}

// Persistence helpers

func (s *Store) jobFilePath(id string) string {
	return filepath.Join(s.path, id+".json")
}

func (s *Store) saveJob(job *Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(s.jobFilePath(job.ID), data, 0644)
}

func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.path, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue // Skip unparseable files
		}

		s.jobs[job.ID] = &job
	}

	return nil
}
