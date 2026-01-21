package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cosa/internal/claude"
	"cosa/internal/job"
	"cosa/internal/ledger"
	"cosa/internal/protocol"
	"cosa/internal/territory"
	"cosa/internal/worker"
)

// Territory management handlers

func (s *Server) handleTerritoryInit(req *protocol.Request) *protocol.Response {
	var params struct {
		Path string `json:"path"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Path == "" {
		wd, _ := os.Getwd()
		params.Path = wd
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.territory != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrTerritoryExists, "territory already initialized", nil)
		return resp
	}

	t, err := territory.Init(params.Path)
	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrTerritoryExists, err.Error(), nil)
		return resp
	}

	s.territory = t
	s.initReviewCoordinator()
	s.ledger.Append(ledger.EventTerritoryInit, map[string]string{"path": t.Path})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{
		"status": "initialized",
		"path":   s.territory.Path,
	})
	return resp
}

func (s *Server) handleTerritoryStatus(req *protocol.Request) *protocol.Response {
	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	if t == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "territory not initialized", nil)
		return resp
	}

	resp, _ := protocol.NewResponse(req.ID, map[string]interface{}{
		"path":                t.Path,
		"repo_root":           t.RepoRoot,
		"base_branch":         t.BaseBranch,
		"dev_branch":          t.Config.DevBranch,
		"merge_target_branch": t.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch),
	})
	return resp
}

func (s *Server) handleTerritoryAdd(req *protocol.Request) *protocol.Response {
	var params struct {
		Path string `json:"path"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Path == "" {
		wd, _ := os.Getwd()
		params.Path = wd
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.territory != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrTerritoryExists, "territory already active", nil)
		return resp
	}

	t, err := territory.Load(params.Path)
	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, err.Error(), nil)
		return resp
	}

	s.territory = t
	s.initReviewCoordinator()
	s.ledger.Append(ledger.EventTerritoryInit, map[string]string{"path": t.Path})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{
		"status": "registered",
		"path":   s.territory.RepoRoot,
	})
	return resp
}

func (s *Server) handleTerritoryList(req *protocol.Request) *protocol.Response {
	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	territories := []map[string]interface{}{}
	if t != nil {
		territories = append(territories, map[string]interface{}{
			"path":        t.Path,
			"repo_root":   t.RepoRoot,
			"base_branch": t.BaseBranch,
			"active":      true,
		})
	}

	resp, _ := protocol.NewResponse(req.ID, map[string]interface{}{
		"territories": territories,
	})
	return resp
}

func (s *Server) handleTerritorySetDevBranch(req *protocol.Request) *protocol.Response {
	var params protocol.TerritorySetDevBranchParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	s.mu.Lock()
	t := s.territory
	s.mu.Unlock()

	if t == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "territory not initialized", nil)
		return resp
	}

	// Set or clear the dev branch
	var err error
	if params.Branch == "" {
		err = t.ClearDevBranch()
	} else {
		err = t.SetDevBranch(params.Branch)
	}

	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	// Reinitialize the review coordinator with the new merge target
	s.initReviewCoordinator()

	resp, _ := protocol.NewResponse(req.ID, protocol.TerritorySetDevBranchResult{
		DevBranch:         t.Config.DevBranch,
		MergeTargetBranch: t.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch),
	})
	return resp
}

// Worker management handlers

func (s *Server) handleWorkerAdd(req *protocol.Request) *protocol.Response {
	var params protocol.WorkerAddParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Name == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "name is required", nil)
		return resp
	}

	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	if t == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "territory not initialized", nil)
		return resp
	}

	// Check if worker already exists
	if s.pool.Exists(params.Name) {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "worker already exists", nil)
		return resp
	}

	// Create worktree
	wt, err := t.CreateWorkerWorktree(params.Name)
	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	// Determine role
	role := worker.Role(params.Role)
	if role == "" {
		role = worker.RoleSoldato
	}

	// Try to restore session if available
	var sessionID string
	if sess, err := s.sessions.LoadByWorkerName(params.Name); err == nil {
		sessionID = sess.SessionID
	}

	// Create worker with completion callbacks
	w := worker.New(worker.Config{
		Name:     params.Name,
		Role:     role,
		Worktree: wt,
		ClaudeConfig: claude.ClientConfig{
			Binary:   s.cfg.Claude.Binary,
			Model:    s.cfg.Claude.Model,
			MaxTurns: s.cfg.Claude.MaxTurns,
		},
		OnEvent: func(e worker.Event) {
			s.ledger.Append(ledger.EventType("worker."+e.Type), e)
		},
		OnJobComplete:     s.onJobComplete,
		OnJobFail:         s.onJobFail,
		MergeTargetBranch: t.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch),
	})

	// Restore session ID if available
	if sessionID != "" {
		w.SessionID = sessionID
	}

	// Add to pool
	if err := s.pool.Add(w); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	// Start worker
	w.Start()

	// Log event
	s.ledger.Append(ledger.EventWorkerAdded, ledger.WorkerEventData{
		ID:       w.ID,
		Name:     w.Name,
		Role:     string(w.Role),
		Worktree: w.Worktree,
	})

	resp, _ := protocol.NewResponse(req.ID, protocol.WorkerInfo{
		ID:       w.ID,
		Name:     w.Name,
		Role:     string(w.Role),
		Status:   string(w.GetStatus()),
		Worktree: w.Worktree,
	})
	return resp
}

func (s *Server) handleWorkerList(req *protocol.Request) *protocol.Response {
	poolWorkers := s.pool.List()
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

	resp, _ := protocol.NewResponse(req.ID, workers)
	return resp
}

func (s *Server) handleWorkerRemove(req *protocol.Request) *protocol.Response {
	var params struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	w, err := s.pool.Remove(params.Name)
	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	// Save session before removing worker
	if w.SessionID != "" {
		s.sessions.Save(&claude.SessionInfo{
			SessionID:  w.SessionID,
			WorkerID:   w.ID,
			WorkerName: w.Name,
			CreatedAt:  w.CreatedAt,
			LastUsed:   time.Now(),
		})
	}

	// Stop worker
	w.Stop()

	// Remove worktree
	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	if t != nil {
		t.RemoveWorkerWorktree(params.Name, params.Force)
	}

	// Log event
	s.ledger.Append(ledger.EventWorkerRemoved, ledger.WorkerEventData{
		ID:   w.ID,
		Name: w.Name,
	})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "removed"})
	return resp
}

// Job management handlers

func (s *Server) handleJobAdd(req *protocol.Request) *protocol.Response {
	var params protocol.JobAddParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Description == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "description is required", nil)
		return resp
	}

	// Create job
	j := job.New(params.Description)
	if params.Priority > 0 {
		j.SetPriority(params.Priority)
	}
	if len(params.DependsOn) > 0 {
		j.SetDependencies(params.DependsOn)
	}

	// Add to store
	s.jobs.Add(j)

	// Log event
	s.ledger.Append(ledger.EventJobCreated, ledger.JobEventData{
		ID:          j.ID,
		Description: j.Description,
	})

	// If worker specified, assign directly to that worker
	if params.Worker != "" {
		w, exists := s.pool.Get(params.Worker)
		if !exists {
			// Try by ID
			w, exists = s.pool.GetByID(params.Worker)
		}

		if exists && w.GetStatus() == worker.StatusIdle {
			j.Queue()
			s.jobs.Save(j) // Persist queued state
			s.ledger.Append(ledger.EventJobQueued, ledger.JobEventData{
				ID:          j.ID,
				Description: j.Description,
				Worker:      w.ID,
				WorkerName:  w.Name,
			})

			go func() {
				// Create job worktree before starting
				if err := s.createJobWorktree(j); err != nil {
					s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
						ID:          j.ID,
						Description: j.Description,
						Worker:      w.ID,
						WorkerName:  w.Name,
						Error:       fmt.Sprintf("failed to create job worktree: %v", err),
					})
					j.Fail(fmt.Sprintf("failed to create job worktree: %v", err))
					s.jobs.Save(j)
					return
				}

				s.ledger.Append(ledger.EventJobStarted, ledger.JobEventData{
					ID:          j.ID,
					Description: j.Description,
					Worker:      w.ID,
					WorkerName:  w.Name,
				})
				if err := w.ExecuteInWorktree(j, j.GetWorktree()); err != nil {
					s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
						ID:          j.ID,
						Description: j.Description,
						Worker:      w.ID,
						WorkerName:  w.Name,
						Error:       err.Error(),
					})
				}
			}()
		} else {
			// Worker not available, add to queue for scheduler
			s.queue.Enqueue(j)
		}
	} else {
		// No worker specified, add to queue for scheduler
		s.queue.Enqueue(j)
	}

	resp, _ := protocol.NewResponse(req.ID, protocol.JobInfo{
		ID:          j.ID,
		Description: j.Description,
		Status:      string(j.GetStatus()),
		Priority:    j.Priority,
		CreatedAt:   j.CreatedAt.Unix(),
	})
	return resp
}

func (s *Server) handleJobList(req *protocol.Request) *protocol.Response {
	jobs := s.jobs.List()
	infos := make([]protocol.JobInfo, 0, len(jobs))

	for _, j := range jobs {
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

	resp, _ := protocol.NewResponse(req.ID, infos)
	return resp
}

func (s *Server) handleJobCancel(req *protocol.Request) *protocol.Response {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	j, exists := s.jobs.Get(params.ID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "job not found", nil)
		return resp
	}

	// Remove from queue if pending
	s.queue.Remove(j.ID)

	j.Cancel()
	s.ledger.Append(ledger.EventJobCancelled, ledger.JobEventData{ID: j.ID})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "cancelled"})
	return resp
}

func (s *Server) handleJobAssign(req *protocol.Request) *protocol.Response {
	var params protocol.JobAssignParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.JobID == "" || params.WorkerID == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "job_id and worker_id are required", nil)
		return resp
	}

	j, exists := s.jobs.Get(params.JobID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "job not found", nil)
		return resp
	}

	w, exists := s.pool.GetByID(params.WorkerID)
	if !exists {
		w, exists = s.pool.Get(params.WorkerID)
	}
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	if w.GetStatus() != worker.StatusIdle {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "worker is not idle", nil)
		return resp
	}

	// Remove from queue and assign
	s.queue.Remove(j.ID)
	j.Queue()

	s.ledger.Append(ledger.EventJobQueued, ledger.JobEventData{
		ID:          j.ID,
		Description: j.Description,
		Worker:      w.ID,
		WorkerName:  w.Name,
	})

	go func() {
		// Create job worktree before starting
		if err := s.createJobWorktree(j); err != nil {
			s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
				ID:          j.ID,
				Description: j.Description,
				Worker:      w.ID,
				WorkerName:  w.Name,
				Error:       fmt.Sprintf("failed to create job worktree: %v", err),
			})
			j.Fail(fmt.Sprintf("failed to create job worktree: %v", err))
			s.jobs.Save(j)
			return
		}

		s.ledger.Append(ledger.EventJobStarted, ledger.JobEventData{
			ID:          j.ID,
			Description: j.Description,
			Worker:      w.ID,
			WorkerName:  w.Name,
		})
		if err := w.ExecuteInWorktree(j, j.GetWorktree()); err != nil {
			s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
				ID:          j.ID,
				Description: j.Description,
				Worker:      w.ID,
				WorkerName:  w.Name,
				Error:       err.Error(),
			})
		}
	}()

	resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "assigned"})
	return resp
}

func (s *Server) handleWorkerStatus(req *protocol.Request) *protocol.Response {
	var params struct {
		Name string `json:"name"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	w, exists := s.pool.Get(params.Name)
	if !exists {
		w, exists = s.pool.GetByID(params.Name)
	}
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	info := protocol.WorkerDetailInfo{
		ID:            w.ID,
		Name:          w.Name,
		Role:          string(w.Role),
		Status:        string(w.GetStatus()),
		Worktree:      w.Worktree,
		Branch:        w.Branch,
		SessionID:     w.SessionID,
		JobsCompleted: w.JobsCompleted,
		JobsFailed:    w.JobsFailed,
		CreatedAt:     w.CreatedAt.Unix(),
	}
	if j := w.GetCurrentJob(); j != nil {
		info.CurrentJob = j.ID
	}

	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

func (s *Server) handleQueueStatus(req *protocol.Request) *protocol.Response {
	result := protocol.QueueStatusResult{
		Ready:   s.queue.ReadyLen(),
		Pending: s.queue.PendingLen(),
		Running: s.jobs.CountByStatus(job.StatusRunning),
		Total:   s.jobs.Count(),
	}

	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

func (s *Server) handleWorkerDetail(req *protocol.Request) *protocol.Response {
	var params struct {
		Name string `json:"name"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	w, exists := s.pool.Get(params.Name)
	if !exists {
		w, exists = s.pool.GetByID(params.Name)
	}
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	info := protocol.WorkerDetailInfo{
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

	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

func (s *Server) handleWorkerMessage(req *protocol.Request) *protocol.Response {
	var params struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Message == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "message is required", nil)
		return resp
	}

	w, exists := s.pool.Get(params.Name)
	if !exists {
		w, exists = s.pool.GetByID(params.Name)
	}
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	if err := w.SendMessage(params.Message); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, err.Error(), nil)
		return resp
	}

	s.ledger.Append(ledger.EventWorkerMessage, ledger.WorkerEventData{
		ID:   w.ID,
		Name: w.Name,
	})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "sent"})
	return resp
}

func (s *Server) handleJobStatus(req *protocol.Request) *protocol.Response {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	j, exists := s.jobs.Get(params.ID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "job not found", nil)
		return resp
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

	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

func (s *Server) handleJobSetPriority(req *protocol.Request) *protocol.Response {
	var params protocol.JobSetPriorityParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.JobID == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "job_id is required", nil)
		return resp
	}

	if params.Priority < 1 || params.Priority > 5 {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "priority must be between 1 and 5", nil)
		return resp
	}

	j, exists := s.jobs.Get(params.JobID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "job not found", nil)
		return resp
	}

	j.SetPriority(params.Priority)
	s.jobs.Save(j)

	s.ledger.Append(ledger.EventType("job.priority_changed"), ledger.JobEventData{
		ID:       j.ID,
		Priority: j.Priority,
	})

	resp, _ := protocol.NewResponse(req.ID, protocol.JobInfo{
		ID:          j.ID,
		Description: j.Description,
		Status:      string(j.GetStatus()),
		Priority:    j.Priority,
		CreatedAt:   j.CreatedAt.Unix(),
	})
	return resp
}

// loadExistingTerritory tries to load an existing territory from the working directory
func (s *Server) loadExistingTerritory(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.territory != nil {
		return nil
	}

	t, err := territory.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load territory: %w", err)
	}

	s.territory = t
	s.initReviewCoordinator()
	return nil
}

// Review management handlers

func (s *Server) handleReviewStart(req *protocol.Request) *protocol.Response {
	var params protocol.ReviewStartParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.JobID == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "job_id is required", nil)
		return resp
	}

	// Get the job
	j, exists := s.jobs.Get(params.JobID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "job not found", nil)
		return resp
	}

	// Get the worker
	w, exists := s.pool.GetByID(j.Worker)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "worker not found", nil)
		return resp
	}

	// Check if coordinator is initialized
	s.mu.RLock()
	coord := s.reviewCoordinator
	s.mu.RUnlock()

	if coord == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "review coordinator not initialized", nil)
		return resp
	}

	// Start the review
	if err := coord.StartReview(s.ctx, j, w); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	resp, _ := protocol.NewResponse(req.ID, map[string]string{
		"status": "review_started",
		"job_id": j.ID,
	})
	return resp
}

func (s *Server) handleReviewStatus(req *protocol.Request) *protocol.Response {
	var params protocol.ReviewStatusParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.JobID == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "job_id is required", nil)
		return resp
	}

	s.mu.RLock()
	coord := s.reviewCoordinator
	s.mu.RUnlock()

	if coord == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "review coordinator not initialized", nil)
		return resp
	}

	status, exists := coord.GetReviewStatus(params.JobID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrReviewNotFound, "review not found", nil)
		return resp
	}

	result := protocol.ReviewStatusResult{
		JobID:      status.JobID,
		WorkerID:   status.WorkerID,
		WorkerName: status.WorkerName,
		Phase:      string(status.Phase),
		Decision:   string(status.Decision),
		Summary:    status.Summary,
		Feedback:   status.Feedback,
		Error:      status.Error,
		StartedAt:  status.StartedAt.Unix(),
	}

	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

func (s *Server) handleReviewList(req *protocol.Request) *protocol.Response {
	s.mu.RLock()
	coord := s.reviewCoordinator
	s.mu.RUnlock()

	if coord == nil {
		resp, _ := protocol.NewResponse(req.ID, protocol.ReviewListResult{
			Reviews: []protocol.ReviewStatusResult{},
		})
		return resp
	}

	reviews := coord.GetActiveReviews()
	results := make([]protocol.ReviewStatusResult, 0, len(reviews))

	for _, status := range reviews {
		results = append(results, protocol.ReviewStatusResult{
			JobID:      status.JobID,
			WorkerID:   status.WorkerID,
			WorkerName: status.WorkerName,
			Phase:      string(status.Phase),
			Decision:   string(status.Decision),
			Summary:    status.Summary,
			Feedback:   status.Feedback,
			Error:      status.Error,
			StartedAt:  status.StartedAt.Unix(),
		})
	}

	resp, _ := protocol.NewResponse(req.ID, protocol.ReviewListResult{
		Reviews: results,
	})
	return resp
}
