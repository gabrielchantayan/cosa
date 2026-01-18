// Package job implements job and operation management for Cosa.
package job

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperationStatus represents the state of an operation.
type OperationStatus string

const (
	OperationStatusPending   OperationStatus = "pending"
	OperationStatusRunning   OperationStatus = "running"
	OperationStatusCompleted OperationStatus = "completed"
	OperationStatusFailed    OperationStatus = "failed"
	OperationStatusCancelled OperationStatus = "cancelled"
)

// Operation represents a batch of related jobs.
type Operation struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Status      OperationStatus `json:"status"`
	Jobs        []string        `json:"jobs"` // Job IDs

	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress tracking
	TotalJobs     int `json:"total_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`

	mu sync.RWMutex
}

// NewOperation creates a new operation.
func NewOperation(name string) *Operation {
	return &Operation{
		ID:        uuid.New().String(),
		Name:      name,
		Status:    OperationStatusPending,
		Jobs:      []string{},
		CreatedAt: time.Now(),
	}
}

// AddJob adds a job to the operation.
func (o *Operation) AddJob(jobID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Jobs = append(o.Jobs, jobID)
	o.TotalJobs = len(o.Jobs)
}

// AddJobs adds multiple jobs to the operation.
func (o *Operation) AddJobs(jobIDs []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Jobs = append(o.Jobs, jobIDs...)
	o.TotalJobs = len(o.Jobs)
}

// Start marks the operation as running.
func (o *Operation) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Status = OperationStatusRunning
	now := time.Now()
	o.StartedAt = &now
}

// Complete marks the operation as completed.
func (o *Operation) Complete() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Status = OperationStatusCompleted
	now := time.Now()
	o.CompletedAt = &now
}

// Fail marks the operation as failed.
func (o *Operation) Fail() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Status = OperationStatusFailed
	now := time.Now()
	o.CompletedAt = &now
}

// Cancel marks the operation as cancelled.
func (o *Operation) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Status = OperationStatusCancelled
	now := time.Now()
	o.CompletedAt = &now
}

// IncrementCompleted increments the completed job count.
func (o *Operation) IncrementCompleted() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.CompletedJobs++
	o.checkCompletion()
}

// IncrementFailed increments the failed job count.
func (o *Operation) IncrementFailed() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.FailedJobs++
	o.checkCompletion()
}

// checkCompletion checks if all jobs are done and updates status.
// Must be called with lock held.
func (o *Operation) checkCompletion() {
	if o.CompletedJobs+o.FailedJobs >= o.TotalJobs {
		now := time.Now()
		o.CompletedAt = &now
		if o.FailedJobs > 0 {
			o.Status = OperationStatusFailed
		} else {
			o.Status = OperationStatusCompleted
		}
	}
}

// GetStatus returns the operation's current status.
func (o *Operation) GetStatus() OperationStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.Status
}

// Progress returns the completion progress as a percentage (0-100).
func (o *Operation) Progress() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.TotalJobs == 0 {
		return 0
	}
	return (o.CompletedJobs + o.FailedJobs) * 100 / o.TotalJobs
}

// GetJobIDs returns a copy of the job IDs in this operation.
func (o *Operation) GetJobIDs() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]string, len(o.Jobs))
	copy(result, o.Jobs)
	return result
}

// GetInfo returns operation info without external locking.
func (o *Operation) GetInfo() (name, description string, status OperationStatus, jobs []string, total, completed, failed int, createdAt time.Time, startedAt, completedAt *time.Time) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	jobsCopy := make([]string, len(o.Jobs))
	copy(jobsCopy, o.Jobs)
	return o.Name, o.Description, o.Status, jobsCopy, o.TotalJobs, o.CompletedJobs, o.FailedJobs, o.CreatedAt, o.StartedAt, o.CompletedAt
}

// IsTerminal returns true if the operation is in a terminal state.
func (o *Operation) IsTerminal() bool {
	status := o.GetStatus()
	return status == OperationStatusCompleted || status == OperationStatusFailed || status == OperationStatusCancelled
}

// ToJSON serializes the operation to JSON.
func (o *Operation) ToJSON() ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return json.Marshal(o)
}

// OperationStore manages operations.
type OperationStore struct {
	operations map[string]*Operation
	mu         sync.RWMutex
}

// NewOperationStore creates a new operation store.
func NewOperationStore() *OperationStore {
	return &OperationStore{
		operations: make(map[string]*Operation),
	}
}

// Add adds an operation to the store.
func (s *OperationStore) Add(op *Operation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[op.ID] = op
}

// Get retrieves an operation by ID.
func (s *OperationStore) Get(id string) (*Operation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.operations[id]
	return op, ok
}

// Remove removes an operation from the store.
func (s *OperationStore) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.operations, id)
}

// List returns all operations.
func (s *OperationStore) List() []*Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ops := make([]*Operation, 0, len(s.operations))
	for _, op := range s.operations {
		ops = append(ops, op)
	}
	return ops
}

// ListByStatus returns operations with a given status.
func (s *OperationStore) ListByStatus(status OperationStatus) []*Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ops []*Operation
	for _, op := range s.operations {
		if op.GetStatus() == status {
			ops = append(ops, op)
		}
	}
	return ops
}

// Count returns the number of operations.
func (s *OperationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.operations)
}

// FindByJobID finds the operation containing a specific job.
func (s *OperationStore) FindByJobID(jobID string) (*Operation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, op := range s.operations {
		op.mu.RLock()
		for _, jID := range op.Jobs {
			if jID == jobID {
				op.mu.RUnlock()
				return op, true
			}
		}
		op.mu.RUnlock()
	}
	return nil, false
}
