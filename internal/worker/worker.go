// Package worker implements worker management for Cosa.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"cosa/internal/claude"
	"cosa/internal/git"
	"cosa/internal/job"
)

// Status represents the state of a worker.
type Status string

const (
	StatusIdle     Status = "idle"
	StatusWorking  Status = "working"
	StatusReviewing Status = "reviewing"
	StatusStopped  Status = "stopped"
	StatusError    Status = "error"
)

// Role defines the worker's role in the hierarchy.
type Role string

const (
	RoleDon         Role = "don"         // Project owner (human)
	RoleUnderboss   Role = "underboss"   // Second in command, delegates to capos
	RoleConsigliere Role = "consigliere" // Code reviewer
	RoleCapo        Role = "capo"        // Team lead, delegates work
	RoleSoldato     Role = "soldato"     // Regular worker
	RoleAssociate   Role = "associate"   // Ephemeral worker for one-off tasks
	RoleLookout     Role = "lookout"     // Monitors for stuck workers
	RoleCleaner     Role = "cleaner"     // Cleans up resources
)

// Worker represents a Claude Code agent working in a git worktree.
type Worker struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	Status    Status    `json:"status"`
	Worktree  string    `json:"worktree"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`

	// Ephemeral marks the worker as temporary (for Associates)
	Ephemeral bool `json:"ephemeral,omitempty"`

	// Standing orders applied to all jobs for this worker
	StandingOrders []string `json:"standing_orders,omitempty"`

	// MergeTargetBranch is the branch where this worker's work will be merged.
	// This could be a dev/staging branch or the main branch.
	MergeTargetBranch string `json:"merge_target_branch,omitempty"`

	// Current job
	CurrentJob *job.Job `json:"current_job,omitempty"`

	// Claude session
	SessionID string `json:"session_id,omitempty"`

	// Stats
	JobsCompleted int `json:"jobs_completed"`
	JobsFailed    int `json:"jobs_failed"`

	// Cost tracking
	TotalCost   string `json:"total_cost,omitempty"`   // Cumulative cost for this worker
	TotalTokens int    `json:"total_tokens,omitempty"` // Cumulative tokens used

	// Last activity tracking for health monitoring
	LastActivityAt time.Time `json:"last_activity_at,omitempty"`

	// Internal state
	mu            sync.RWMutex
	client        *claude.Client
	ctx           context.Context
	cancel        context.CancelFunc
	events        chan Event
	onEvent       func(Event)
	onJobComplete func(*job.Job)
	onJobFail     func(*job.Job, error)
}

// Event represents a worker event.
type Event struct {
	Type     string      `json:"type"`
	Worker   string      `json:"worker"`
	Job      string      `json:"job,omitempty"`
	Message  string      `json:"message,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	Time     time.Time   `json:"time"`
}

// Config configures a worker.
type Config struct {
	Name              string
	Role              Role
	Worktree          *git.Worktree
	ClaudeConfig      claude.ClientConfig
	OnEvent           func(Event)
	OnJobComplete     func(*job.Job)
	OnJobFail         func(*job.Job, error)
	MergeTargetBranch string // Branch where work will be merged (dev branch or main)
}

// New creates a new worker.
func New(cfg Config) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	if cfg.Role == "" {
		cfg.Role = RoleSoldato
	}

	w := &Worker{
		ID:                uuid.New().String(),
		Name:              cfg.Name,
		Role:              cfg.Role,
		Status:            StatusIdle,
		CreatedAt:         time.Now(),
		MergeTargetBranch: cfg.MergeTargetBranch,
		ctx:               ctx,
		cancel:            cancel,
		events:            make(chan Event, 100),
		onEvent:           cfg.OnEvent,
		onJobComplete:     cfg.OnJobComplete,
		onJobFail:         cfg.OnJobFail,
	}

	if cfg.Worktree != nil {
		w.Worktree = cfg.Worktree.Path
		w.Branch = cfg.Worktree.Branch
		cfg.ClaudeConfig.Workdir = cfg.Worktree.Path
	}

	w.client = claude.NewClient(cfg.ClaudeConfig)

	return w
}

// Start starts the worker.
func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.Status != StatusIdle && w.Status != StatusStopped {
		return fmt.Errorf("worker is not idle")
	}

	w.Status = StatusIdle
	w.emitEvent("started", "Worker started")

	return nil
}

// Stop stops the worker.
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.cancel()
	if w.client != nil {
		w.client.Stop()
	}

	w.Status = StatusStopped
	w.emitEvent("stopped", "Worker stopped")

	return nil
}

// Execute runs a job on this worker.
func (w *Worker) Execute(j *job.Job) error {
	w.mu.Lock()
	if w.Status != StatusIdle {
		w.mu.Unlock()
		return fmt.Errorf("worker is not idle")
	}
	w.Status = StatusWorking
	w.CurrentJob = j
	w.mu.Unlock()

	w.emitEvent("job_started", fmt.Sprintf("Starting job: %s", j.Description))

	// Build prompt for Claude
	prompt := w.buildPrompt(j)

	// Start or resume Claude session
	var err error
	if w.SessionID != "" {
		err = w.client.Resume(w.ctx, w.SessionID, prompt)
	} else {
		err = w.client.Start(w.ctx, prompt)
	}

	if err != nil {
		w.handleJobFailure(j, err)
		return err
	}

	// Process events from Claude
	go w.processClaudeEvents(j)

	return nil
}

func (w *Worker) processClaudeEvents(j *job.Job) {
	defer func() {
		w.mu.Lock()
		w.Status = StatusIdle
		w.CurrentJob = nil
		w.mu.Unlock()
	}()

	for {
		select {
		case <-w.ctx.Done():
			w.handleJobFailure(j, w.ctx.Err())
			return

		case event, ok := <-w.client.Events():
			if !ok {
				// Channel closed, session ended
				if j.GetStatus() == job.StatusRunning {
					// Assume success if no error
					w.handleJobSuccess(j)
				}
				return
			}

			w.handleClaudeEvent(j, event)

		case <-w.client.Done():
			if j.GetStatus() == job.StatusRunning {
				w.handleJobSuccess(j)
			}
			return
		}
	}
}

func (w *Worker) handleClaudeEvent(j *job.Job, event claude.Event) {
	// Update activity timestamp for any event
	w.UpdateActivity()

	switch event.Type {
	case claude.EventInit:
		w.mu.Lock()
		w.SessionID = event.SessionID
		w.mu.Unlock()
		j.Start(w.ID, event.SessionID)

	case claude.EventAssistantText:
		w.emitEvent("message", event.Message)

	case claude.EventToolUse:
		w.emitEvent("tool_use", fmt.Sprintf("Using tool: %s", event.Tool.Name))

	case claude.EventToolResult:
		w.emitEvent("tool_result", fmt.Sprintf("Tool completed: %s", event.Tool.Name))

	case claude.EventResult:
		// Update cost tracking from result
		if event.Result != nil {
			if event.Result.TotalCost != "" || event.Result.TotalTokens > 0 {
				w.UpdateCost(event.Result.TotalCost, event.Result.TotalTokens)
			}
			if !event.Result.Success {
				w.handleJobFailure(j, fmt.Errorf("claude reported failure"))
			} else {
				w.handleJobSuccess(j)
			}
		} else {
			w.handleJobSuccess(j)
		}

	case claude.EventError:
		w.emitEvent("error", event.Error)
	}
}

func (w *Worker) handleJobSuccess(j *job.Job) {
	j.Complete("")
	w.mu.Lock()
	w.JobsCompleted++
	onComplete := w.onJobComplete
	w.mu.Unlock()
	w.emitEvent("job_completed", fmt.Sprintf("Completed job: %s", j.Description))

	if onComplete != nil {
		onComplete(j)
	}
}

func (w *Worker) handleJobFailure(j *job.Job, err error) {
	j.Fail(err.Error())
	w.mu.Lock()
	w.JobsFailed++
	w.Status = StatusError
	onFail := w.onJobFail
	w.mu.Unlock()
	w.emitEvent("job_failed", fmt.Sprintf("Job failed: %v", err))

	if onFail != nil {
		onFail(j, err)
	}
}

func (w *Worker) buildPrompt(j *job.Job) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are %s, a %s worker in the Cosa development team.\n\n", w.Name, w.Role))

	// Include standing orders if present
	w.mu.RLock()
	orders := w.StandingOrders
	w.mu.RUnlock()

	if len(orders) > 0 {
		sb.WriteString("## Standing Orders\n")
		for _, order := range orders {
			sb.WriteString("- " + order + "\n")
		}
		sb.WriteString("\n")
	}

	// Include review feedback if this is a revision job
	if len(j.ReviewFeedback) > 0 {
		sb.WriteString("## Previous Review Feedback\n")
		sb.WriteString("Address these issues from the previous review:\n")
		for _, feedback := range j.ReviewFeedback {
			sb.WriteString("- " + feedback + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("## Your Task\n%s\n\n", j.Description))
	sb.WriteString("Work in your designated worktree. Make commits as you go.\n")
	if w.MergeTargetBranch != "" {
		sb.WriteString(fmt.Sprintf("When finished, your work will be merged into the '%s' branch.\n", w.MergeTargetBranch))
	}
	sb.WriteString("When finished, summarize what you did.")

	return sb.String()
}

func (w *Worker) emitEvent(eventType, message string) {
	event := Event{
		Type:    eventType,
		Worker:  w.ID,
		Message: message,
		Time:    time.Now(),
	}

	if w.CurrentJob != nil {
		event.Job = w.CurrentJob.ID
	}

	select {
	case w.events <- event:
	default:
	}

	if w.onEvent != nil {
		w.onEvent(event)
	}
}

// Events returns the worker's event channel.
func (w *Worker) Events() <-chan Event {
	return w.events
}

// GetStatus returns the worker's current status.
func (w *Worker) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

// GetCurrentJob returns the worker's current job.
func (w *Worker) GetCurrentJob() *job.Job {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.CurrentJob
}

// ToJSON serializes the worker to JSON.
func (w *Worker) ToJSON() ([]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return json.Marshal(w)
}

// UpdateActivity updates the last activity timestamp.
func (w *Worker) UpdateActivity() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.LastActivityAt = time.Now()
}

// GetLastActivity returns the last activity timestamp.
func (w *Worker) GetLastActivity() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.LastActivityAt
}

// IsStuck returns true if the worker has been inactive longer than the threshold.
func (w *Worker) IsStuck(threshold time.Duration) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Only check working workers
	if w.Status != StatusWorking {
		return false
	}

	// If never had activity, use created time
	lastActivity := w.LastActivityAt
	if lastActivity.IsZero() {
		lastActivity = w.CreatedAt
	}

	return time.Since(lastActivity) > threshold
}

// SetStandingOrders sets the standing orders for this worker.
func (w *Worker) SetStandingOrders(orders []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.StandingOrders = orders
}

// GetStandingOrders returns the standing orders for this worker.
func (w *Worker) GetStandingOrders() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]string, len(w.StandingOrders))
	copy(result, w.StandingOrders)
	return result
}

// AddStandingOrder adds a standing order to this worker.
func (w *Worker) AddStandingOrder(order string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.StandingOrders = append(w.StandingOrders, order)
}

// ClearStandingOrders removes all standing orders.
func (w *Worker) ClearStandingOrders() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.StandingOrders = nil
}

// UpdateCost updates the cost tracking fields.
func (w *Worker) UpdateCost(cost string, tokens int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.TotalCost = cost
	w.TotalTokens = tokens
}

// SendMessage sends a message to the worker's active Claude session.
func (w *Worker) SendMessage(message string) error {
	w.mu.RLock()
	client := w.client
	status := w.Status
	w.mu.RUnlock()

	if status != StatusWorking {
		return fmt.Errorf("worker is not actively working")
	}

	if client == nil {
		return fmt.Errorf("no active session")
	}

	return client.SendInput(message)
}
