package daemon

import (
	"fmt"

	"cosa/internal/job"
	"cosa/internal/mcp"
	"cosa/internal/protocol"
)

// MCPAdapter wraps a Server and implements mcp.DaemonInterface.
type MCPAdapter struct {
	server *Server
}

// NewMCPAdapter creates a new MCP adapter for the daemon server.
func NewMCPAdapter(server *Server) *MCPAdapter {
	return &MCPAdapter{server: server}
}

// ListWorkers returns all workers.
func (a *MCPAdapter) ListWorkers() []protocol.WorkerInfo {
	poolWorkers := a.server.pool.List()
	workers := make([]protocol.WorkerInfo, 0, len(poolWorkers))
	for _, w := range poolWorkers {
		info := protocol.WorkerInfo{
			ID:       w.ID,
			Name:     w.Name,
			Role:     string(w.Role),
			Status:   string(w.GetStatus()),
			Worktree: w.Worktree,
		}
		if j := w.GetCurrentJob(); j != nil {
			info.CurrentJob = j.ID
			info.CurrentJobDesc = j.Description
		}
		workers = append(workers, info)
	}
	return workers
}

// GetWorker returns details for a specific worker.
func (a *MCPAdapter) GetWorker(name string) (*protocol.WorkerDetailInfo, error) {
	w, exists := a.server.pool.Get(name)
	if !exists {
		w, exists = a.server.pool.GetByID(name)
	}
	if !exists {
		return nil, fmt.Errorf("worker not found: %s", name)
	}

	info := &protocol.WorkerDetailInfo{
		ID:            w.ID,
		Name:          w.Name,
		Role:          string(w.Role),
		Status:        string(w.GetStatus()),
		Worktree:      w.Worktree,
		Branch:        w.Branch,
		SessionID:     w.SessionID,
		JobsCompleted: w.JobsCompleted,
		JobsFailed:    w.JobsFailed,
		TotalCost:     w.TotalCost,
		TotalTokens:   w.TotalTokens,
		CreatedAt:     w.CreatedAt.Unix(),
	}
	if j := w.GetCurrentJob(); j != nil {
		info.CurrentJob = j.ID
	}
	return info, nil
}

// ListJobs returns jobs, optionally filtered by status.
func (a *MCPAdapter) ListJobs(status string) []protocol.JobInfo {
	jobs := a.server.jobs.List()
	infos := make([]protocol.JobInfo, 0, len(jobs))

	for _, j := range jobs {
		// Filter by status if specified
		if status != "" && string(j.GetStatus()) != status {
			continue
		}

		info := protocol.JobInfo{
			ID:          j.ID,
			Description: j.Description,
			Status:      string(j.GetStatus()),
			Priority:    j.Priority,
			Worker:      j.Worker,
			DependsOn:   j.DependsOn,
			CreatedAt:   j.CreatedAt.Unix(),
		}
		if j.StartedAt != nil {
			info.StartedAt = j.StartedAt.Unix()
		}
		if j.CompletedAt != nil {
			info.CompletedAt = j.CompletedAt.Unix()
		}
		infos = append(infos, info)
	}
	return infos
}

// GetJob returns details for a specific job.
func (a *MCPAdapter) GetJob(id string) (*protocol.JobInfo, error) {
	j, exists := a.server.jobs.Get(id)
	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	info := &protocol.JobInfo{
		ID:          j.ID,
		Description: j.Description,
		Status:      string(j.GetStatus()),
		Priority:    j.Priority,
		Worker:      j.Worker,
		DependsOn:   j.DependsOn,
		CreatedAt:   j.CreatedAt.Unix(),
	}
	if j.StartedAt != nil {
		info.StartedAt = j.StartedAt.Unix()
	}
	if j.CompletedAt != nil {
		info.CompletedAt = j.CompletedAt.Unix()
	}
	return info, nil
}

// CreateJob creates a new job.
func (a *MCPAdapter) CreateJob(description string, priority int, territory string) (*job.Job, error) {
	j := job.New(description)
	if priority > 0 {
		j.SetPriority(priority)
	}

	// Add to store
	a.server.jobs.Add(j)

	// Add to queue for scheduler
	a.server.queue.Enqueue(j)

	return j, nil
}

// CancelJob cancels a job.
func (a *MCPAdapter) CancelJob(id string) error {
	j, exists := a.server.jobs.Get(id)
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	// Remove from queue if pending
	a.server.queue.Remove(j.ID)
	j.Cancel()
	return nil
}

// SetJobPriority updates a job's priority.
func (a *MCPAdapter) SetJobPriority(id string, priority int) error {
	j, exists := a.server.jobs.Get(id)
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	j.SetPriority(priority)
	a.server.jobs.Save(j)
	return nil
}

// ListActivity returns recent activity entries.
func (a *MCPAdapter) ListActivity(limit int) []mcp.ActivityEntry {
	// Activity from ledger is not directly accessible here
	// Return empty list for now - could be enhanced with ledger query method
	return []mcp.ActivityEntry{}
}

// GetQueueStatus returns the current queue status.
func (a *MCPAdapter) GetQueueStatus() *protocol.QueueStatusResult {
	return &protocol.QueueStatusResult{
		Ready:   a.server.queue.ReadyLen(),
		Pending: a.server.queue.PendingLen(),
		Running: a.server.jobs.CountByStatus(job.StatusRunning),
		Total:   a.server.jobs.Count(),
	}
}

// ListTerritories returns all territories.
func (a *MCPAdapter) ListTerritories() []mcp.TerritoryInfo {
	a.server.mu.RLock()
	t := a.server.territory
	a.server.mu.RUnlock()

	territories := make([]mcp.TerritoryInfo, 0)
	if t != nil {
		territories = append(territories, mcp.TerritoryInfo{
			Path:       t.Path,
			BaseBranch: t.BaseBranch,
			DevBranch:  t.Config.DevBranch,
		})
	}
	return territories
}

// ListOperations returns all operations.
func (a *MCPAdapter) ListOperations() []protocol.OperationInfo {
	ops := a.server.operations.List()
	infos := make([]protocol.OperationInfo, 0, len(ops))

	for _, op := range ops {
		infos = append(infos, protocol.OperationInfo{
			ID:            op.ID,
			Name:          op.Name,
			Description:   op.Description,
			Status:        string(op.Status),
			Jobs:          op.Jobs,
			TotalJobs:     op.TotalJobs,
			CompletedJobs: op.CompletedJobs,
			FailedJobs:    op.FailedJobs,
			Progress:      op.Progress(), // Call method instead of accessing field
			CreatedAt:     op.CreatedAt.Unix(),
		})
	}
	return infos
}

// GetCosts returns a cost summary.
func (a *MCPAdapter) GetCosts() *mcp.CostSummary {
	workers := a.server.pool.List()
	if len(workers) == 0 {
		return &mcp.CostSummary{
			TotalCost:   "$0.00",
			TotalTokens: 0,
		}
	}

	var totalTokens int
	byWorker := make([]mcp.WorkerCost, 0, len(workers))

	for _, w := range workers {
		totalTokens += w.TotalTokens
		if w.TotalCost != "" && w.TotalCost != "$0.00" {
			byWorker = append(byWorker, mcp.WorkerCost{
				Name:   w.Name,
				Cost:   w.TotalCost,
				Tokens: w.TotalTokens,
			})
		}
	}

	// Calculate approximate total cost from tokens.
	// NOTE: This is an approximation assuming Claude Sonnet pricing of ~$15/MTok for output.
	// Actual costs depend on the model used and current Anthropic pricing.
	// This should be replaced with actual cost tracking from Claude responses.
	totalCost := fmt.Sprintf("$%.2f", float64(totalTokens)/1000000*15)

	return &mcp.CostSummary{
		TotalCost:   totalCost,
		TotalTokens: totalTokens,
		ByWorker:    byWorker,
	}
}

// GetServer returns the underlying server for MCP CLI command.
func (a *MCPAdapter) GetServer() *Server {
	return a.server
}
