package daemon

import (
	"encoding/json"

	"cosa/internal/job"
	"cosa/internal/protocol"
)

// handleOperationCreate creates a new operation with the given jobs.
func (s *Server) handleOperationCreate(req *protocol.Request) *protocol.Response {
	var params protocol.OperationCreateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	if params.Name == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Name is required", nil)
		return resp
	}

	// Create operation
	op := job.NewOperation(params.Name)
	op.Description = params.Description

	// Add jobs to operation
	for _, jobID := range params.Jobs {
		j, exists := s.jobs.Get(jobID)
		if !exists {
			resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrJobNotFound, "Job not found: "+jobID, nil)
			return resp
		}
		op.AddJob(j.ID)
		j.Operation = op.ID
	}

	// Store operation
	s.operations.Add(op)

	// Start operation if it has jobs
	if len(op.Jobs) > 0 {
		op.Start()
	}

	info := operationToInfo(op)
	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

// handleOperationStatus returns the status of an operation.
func (s *Server) handleOperationStatus(req *protocol.Request) *protocol.Response {
	var params protocol.OperationStatusParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	op, exists := s.operations.Get(params.ID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrOperationNotFound, "Operation not found", nil)
		return resp
	}

	info := operationToInfo(op)
	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

// handleOperationList returns all operations.
func (s *Server) handleOperationList(req *protocol.Request) *protocol.Response {
	ops := s.operations.List()
	infos := make([]protocol.OperationInfo, len(ops))
	for i, op := range ops {
		infos[i] = operationToInfo(op)
	}

	result := protocol.OperationListResult{
		Operations: infos,
	}
	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

// handleOperationCancel cancels an operation and its jobs.
func (s *Server) handleOperationCancel(req *protocol.Request) *protocol.Response {
	var params protocol.OperationCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	op, exists := s.operations.Get(params.ID)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrOperationNotFound, "Operation not found", nil)
		return resp
	}

	// Cancel all pending/running jobs in the operation
	jobIDs := op.GetJobIDs()
	for _, jobID := range jobIDs {
		if j, exists := s.jobs.Get(jobID); exists {
			if !j.IsTerminal() {
				j.Cancel()
			}
		}
	}

	op.Cancel()

	info := operationToInfo(op)
	resp, _ := protocol.NewResponse(req.ID, info)
	return resp
}

// handleOrderSet sets standing orders for a worker.
func (s *Server) handleOrderSet(req *protocol.Request) *protocol.Response {
	var params protocol.OrderSetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	w, exists := s.pool.Get(params.Worker)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "Worker not found", nil)
		return resp
	}

	w.SetStandingOrders(params.Orders)

	result := protocol.OrderListResult{
		Worker: w.Name,
		Orders: w.GetStandingOrders(),
	}
	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

// handleOrderList lists standing orders for a worker.
func (s *Server) handleOrderList(req *protocol.Request) *protocol.Response {
	var params protocol.OrderListParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	w, exists := s.pool.Get(params.Worker)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "Worker not found", nil)
		return resp
	}

	result := protocol.OrderListResult{
		Worker: w.Name,
		Orders: w.GetStandingOrders(),
	}
	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

// handleOrderClear clears standing orders for a worker.
func (s *Server) handleOrderClear(req *protocol.Request) *protocol.Response {
	var params protocol.OrderClearParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	w, exists := s.pool.Get(params.Worker)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "Worker not found", nil)
		return resp
	}

	w.ClearStandingOrders()

	result := protocol.OrderListResult{
		Worker: w.Name,
		Orders: []string{},
	}
	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

// handleHandoffGenerate generates a handoff summary for a worker.
func (s *Server) handleHandoffGenerate(req *protocol.Request) *protocol.Response {
	var params protocol.HandoffGenerateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "Invalid params", nil)
		return resp
	}

	w, exists := s.pool.Get(params.Worker)
	if !exists {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrWorkerNotFound, "Worker not found", nil)
		return resp
	}

	// Generate handoff summary
	summary := protocol.HandoffSummary{
		WorkerID:   w.ID,
		WorkerName: w.Name,
		Status:     string(w.GetStatus()),
		CreatedAt:  w.CreatedAt.Unix(),
	}

	// Include current job info if working
	if job := w.GetCurrentJob(); job != nil {
		summary.JobID = job.ID
	}

	// Note: In a full implementation, we would analyze the session output
	// to extract decisions, files touched, and open questions.
	// For now, we return a basic summary.

	resp, _ := protocol.NewResponse(req.ID, summary)
	return resp
}

// operationToInfo converts an Operation to OperationInfo.
func operationToInfo(op *job.Operation) protocol.OperationInfo {
	name, description, status, jobs, total, completed, failed, createdAt, startedAt, completedAt := op.GetInfo()

	info := protocol.OperationInfo{
		ID:            op.ID,
		Name:          name,
		Description:   description,
		Status:        string(status),
		Jobs:          jobs,
		TotalJobs:     total,
		CompletedJobs: completed,
		FailedJobs:    failed,
		Progress:      op.Progress(),
		CreatedAt:     createdAt.Unix(),
	}

	if startedAt != nil {
		info.StartedAt = startedAt.Unix()
	}
	if completedAt != nil {
		info.CompletedAt = completedAt.Unix()
	}

	return info
}
