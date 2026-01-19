package review

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cosa/internal/git"
	"cosa/internal/job"
	"cosa/internal/ledger"
	"cosa/internal/worker"
)

// ReviewPhase represents the current phase of a review.
type ReviewPhase string

const (
	PhaseGates      ReviewPhase = "gates"
	PhaseDiff       ReviewPhase = "diff"
	PhaseReview     ReviewPhase = "review"
	PhaseDecision   ReviewPhase = "decision"
	PhaseCompleted  ReviewPhase = "completed"
	PhaseFailed     ReviewPhase = "failed"
)

// ReviewStatus contains the current status of an active review.
type ReviewStatus struct {
	JobID       string      `json:"job_id"`
	WorkerID    string      `json:"worker_id"`
	WorkerName  string      `json:"worker_name"`
	Phase       ReviewPhase `json:"phase"`
	StartedAt   time.Time   `json:"started_at"`
	Decision    Decision    `json:"decision,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Feedback    string      `json:"feedback,omitempty"`
	Error       string      `json:"error,omitempty"`
	GatesPassed bool        `json:"gates_passed"`
}

// CoordinatorConfig configures the review coordinator.
type CoordinatorConfig struct {
	GitManager   *git.Manager
	JobStore     *job.Store
	JobQueue     *job.Queue
	Ledger       *ledger.Ledger
	ClaudeConfig ConsigliereConfig
	GateConfig   GateRunnerConfig
	BaseBranch   string
}

// Coordinator orchestrates the code review flow.
type Coordinator struct {
	gitManager      *git.Manager
	jobStore        *job.Store
	jobQueue        *job.Queue
	ledger          *ledger.Ledger
	gateRunner      *GateRunner
	consigliere     *Consigliere
	decisionHandler *DecisionHandler
	baseBranch      string

	activeReviews map[string]*ReviewStatus
	mu            sync.RWMutex
}

// NewCoordinator creates a new review coordinator.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	return &Coordinator{
		gitManager: cfg.GitManager,
		jobStore:   cfg.JobStore,
		jobQueue:   cfg.JobQueue,
		ledger:     cfg.Ledger,
		gateRunner: NewGateRunner(cfg.GateConfig),
		consigliere: NewConsigliere(cfg.ClaudeConfig),
		decisionHandler: NewDecisionHandler(DecisionHandlerConfig{
			GitManager: cfg.GitManager,
			JobStore:   cfg.JobStore,
			Ledger:     cfg.Ledger,
			BaseBranch: cfg.BaseBranch,
		}),
		baseBranch:    cfg.BaseBranch,
		activeReviews: make(map[string]*ReviewStatus),
	}
}

// StartReview begins the review process for a completed job.
func (c *Coordinator) StartReview(ctx context.Context, j *job.Job, w *worker.Worker) error {
	// Mark job for review
	j.MarkForReview()

	// Create review status
	status := &ReviewStatus{
		JobID:      j.ID,
		WorkerID:   w.ID,
		WorkerName: w.Name,
		Phase:      PhaseGates,
		StartedAt:  time.Now(),
	}

	c.mu.Lock()
	c.activeReviews[j.ID] = status
	c.mu.Unlock()

	// Log review started
	c.ledger.Append(ledger.EventReviewStarted, ledger.ReviewEventData{
		JobID:      j.ID,
		WorkerID:   w.ID,
		WorkerName: w.Name,
	})

	// Run the review flow
	go c.runReviewFlow(ctx, j, w, status)

	return nil
}

// runReviewFlow executes the complete review pipeline.
func (c *Coordinator) runReviewFlow(ctx context.Context, j *job.Job, w *worker.Worker, status *ReviewStatus) {
	defer func() {
		c.mu.Lock()
		delete(c.activeReviews, j.ID)
		c.mu.Unlock()
	}()

	// Phase 1: Run quality gates
	c.updatePhase(status, PhaseGates)
	c.ledger.Append(ledger.EventGateStarted, ledger.GateEventData{
		JobID:      j.ID,
		WorkerID:   w.ID,
	})

	gateResults, err := c.gateRunner.RunGates(ctx, j, w.Worktree)
	if err != nil {
		c.handleReviewError(j, status, fmt.Sprintf("gate runner error: %v", err))
		return
	}

	if !AllPassed(gateResults) {
		failed := FailedGates(gateResults)
		c.ledger.Append(ledger.EventGateFailed, ledger.GateEventData{
			JobID:      j.ID,
			WorkerID:   w.ID,
			GateName:   string(failed[0].Gate),
			Output:     failed[0].Output,
		})
		status.GatesPassed = false
		c.handleReviewError(j, status, fmt.Sprintf("quality gates failed: %s", GateResultsSummary(gateResults)))
		return
	}

	status.GatesPassed = true
	c.ledger.Append(ledger.EventGatePassed, ledger.GateEventData{
		JobID:    j.ID,
		WorkerID: w.ID,
	})

	// Phase 2: Get diff
	c.updatePhase(status, PhaseDiff)

	diff, err := c.gitManager.GetDiff(w.Worktree, c.baseBranch)
	if err != nil {
		c.handleReviewError(j, status, fmt.Sprintf("failed to get diff: %v", err))
		return
	}

	// Check if there are any changes
	if len(diff.FilesChanged) == 0 {
		c.handleReviewError(j, status, "no changes to review")
		return
	}

	// Phase 3: AI review
	c.updatePhase(status, PhaseReview)

	reviewCtx := &ReviewContext{
		Job:         j,
		Diff:        diff.Diff,
		GateResults: gateResults,
		BaseBranch:  c.baseBranch,
		WorkerName:  w.Name,
	}

	reviewResult, err := c.consigliere.Review(ctx, reviewCtx)
	if err != nil {
		c.handleReviewError(j, status, fmt.Sprintf("review failed: %v", err))
		return
	}

	status.Decision = reviewResult.Decision
	status.Summary = reviewResult.Summary
	status.Feedback = reviewResult.Feedback

	// Phase 4: Handle decision
	c.updatePhase(status, PhaseDecision)

	if reviewResult.Decision == DecisionApproved {
		if err := c.decisionHandler.HandleApproval(ctx, j, reviewResult); err != nil {
			c.handleReviewError(j, status, fmt.Sprintf("merge failed: %v", err))
			return
		}

		c.ledger.Append(ledger.EventReviewApproved, ledger.ReviewEventData{
			JobID:    j.ID,
			WorkerID: w.ID,
			Summary:  reviewResult.Summary,
		})
	} else {
		revisionJob, err := c.decisionHandler.HandleRejection(ctx, j, w, reviewResult)
		if err != nil {
			c.handleReviewError(j, status, fmt.Sprintf("failed to create revision job: %v", err))
			return
		}

		c.ledger.Append(ledger.EventReviewRejected, ledger.ReviewEventData{
			JobID:         j.ID,
			WorkerID:      w.ID,
			Summary:       reviewResult.Summary,
			Feedback:      reviewResult.Feedback,
			RevisionJobID: revisionJob.ID,
		})

		// Queue the revision job for the same worker
		c.jobQueue.Enqueue(revisionJob)
	}

	c.updatePhase(status, PhaseCompleted)
}

// handleReviewError handles errors during the review process.
func (c *Coordinator) handleReviewError(j *job.Job, status *ReviewStatus, errMsg string) {
	status.Error = errMsg
	status.Phase = PhaseFailed

	c.ledger.Append(ledger.EventReviewRejected, ledger.ReviewEventData{
		JobID: j.ID,
		Error: errMsg,
	})

	// Mark job as failed so it reaches a terminal state and doesn't block dependent jobs
	j.Fail(fmt.Sprintf("review failed: %s", errMsg))
}

// updatePhase updates the current phase of a review.
func (c *Coordinator) updatePhase(status *ReviewStatus, phase ReviewPhase) {
	c.mu.Lock()
	status.Phase = phase
	c.mu.Unlock()
}

// GetActiveReviews returns all currently active reviews.
func (c *Coordinator) GetActiveReviews() []ReviewStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	reviews := make([]ReviewStatus, 0, len(c.activeReviews))
	for _, status := range c.activeReviews {
		reviews = append(reviews, *status)
	}
	return reviews
}

// GetReviewStatus returns the status of a specific review.
func (c *Coordinator) GetReviewStatus(jobID string) (*ReviewStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status, ok := c.activeReviews[jobID]
	if !ok {
		return nil, false
	}
	return status, true
}
