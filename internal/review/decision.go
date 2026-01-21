package review

import (
	"context"
	"fmt"
	"strings"

	"cosa/internal/git"
	"cosa/internal/job"
	"cosa/internal/ledger"
	"cosa/internal/worker"
)

// DecisionHandlerConfig configures the decision handler.
type DecisionHandlerConfig struct {
	GitManager *git.Manager
	JobStore   *job.Store
	Ledger     *ledger.Ledger
	BaseBranch string
}

// DecisionHandler handles review decisions (approve/reject).
type DecisionHandler struct {
	gitManager *git.Manager
	jobStore   *job.Store
	ledger     *ledger.Ledger
	baseBranch string
}

// NewDecisionHandler creates a new decision handler.
func NewDecisionHandler(cfg DecisionHandlerConfig) *DecisionHandler {
	return &DecisionHandler{
		gitManager: cfg.GitManager,
		jobStore:   cfg.JobStore,
		ledger:     cfg.Ledger,
		baseBranch: cfg.BaseBranch,
	}
}

// HandleApproval handles an approved review by merging the changes.
func (d *DecisionHandler) HandleApproval(ctx context.Context, j *job.Job, w *worker.Worker, result *ReviewResult) error {
	// Get the worker branch from the worker name
	workerBranch := fmt.Sprintf("cosa/%s", w.Name)

	// Log merge started
	d.ledger.Append(ledger.EventMergeStarted, ledger.MergeEventData{
		JobID:        j.ID,
		WorkerBranch: workerBranch,
		BaseBranch:   d.baseBranch,
	})

	// Check for conflicts first
	hasConflicts, conflictFiles, err := d.gitManager.HasConflicts(workerBranch, d.baseBranch)
	if err != nil {
		d.ledger.Append(ledger.EventMergeFailed, ledger.MergeEventData{
			JobID: j.ID,
			Error: err.Error(),
		})
		return fmt.Errorf("failed to check conflicts: %w", err)
	}

	if hasConflicts {
		d.ledger.Append(ledger.EventMergeFailed, ledger.MergeEventData{
			JobID:         j.ID,
			Error:         "merge conflicts detected",
			ConflictFiles: conflictFiles,
		})
		return fmt.Errorf("merge conflicts detected in files: %v", conflictFiles)
	}

	// Perform the merge
	mergeResult, err := d.gitManager.Merge(workerBranch, d.baseBranch)
	if err != nil {
		d.ledger.Append(ledger.EventMergeFailed, ledger.MergeEventData{
			JobID: j.ID,
			Error: err.Error(),
		})
		return fmt.Errorf("merge failed: %w", err)
	}

	if !mergeResult.Success {
		d.ledger.Append(ledger.EventMergeFailed, ledger.MergeEventData{
			JobID: j.ID,
			Error: mergeResult.Message,
		})
		return fmt.Errorf("merge failed: %s", mergeResult.Message)
	}

	// Log merge completed
	d.ledger.Append(ledger.EventMergeCompleted, ledger.MergeEventData{
		JobID:       j.ID,
		MergeCommit: mergeResult.MergeCommit,
	})

	// Mark job as completed
	j.Complete(result.Summary)

	return nil
}

// HandleRejection handles a rejected review by creating a revision job.
func (d *DecisionHandler) HandleRejection(ctx context.Context, j *job.Job, w *worker.Worker, result *ReviewResult) (*job.Job, error) {
	// Build feedback for the revision job
	feedback := buildRevisionFeedback(j, result)

	// Create a revision job
	revisionJob := job.New(fmt.Sprintf("Revision of: %s", j.Description))
	revisionJob.SetPriority(j.Priority + 1) // Higher priority for revisions
	revisionJob.SetRevisionOf(j.ID)
	revisionJob.SetReviewFeedback(result.MustFix)

	// Update the original job description to include feedback
	revisionJob.Description = feedback

	// Add to job store
	d.jobStore.Add(revisionJob)

	// Log revision job created
	d.ledger.Append(ledger.EventJobCreated, ledger.JobEventData{
		ID:          revisionJob.ID,
		Description: revisionJob.Description,
		Worker:      w.ID,
	})

	return revisionJob, nil
}

// buildRevisionFeedback constructs the feedback prompt for a revision job.
func buildRevisionFeedback(originalJob *job.Job, result *ReviewResult) string {
	var sb strings.Builder

	sb.WriteString("Please address the following review feedback:\n\n")
	sb.WriteString(fmt.Sprintf("Original task: %s\n\n", originalJob.Description))

	if result.Summary != "" {
		sb.WriteString(fmt.Sprintf("Review summary: %s\n\n", result.Summary))
	}

	if result.Feedback != "" {
		sb.WriteString(fmt.Sprintf("Feedback:\n%s\n\n", result.Feedback))
	}

	if len(result.MustFix) > 0 {
		sb.WriteString("Issues to fix:\n")
		for i, issue := range result.MustFix {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, issue))
		}
	}

	return sb.String()
}

