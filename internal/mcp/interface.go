package mcp

import (
	"cosa/internal/job"
	"cosa/internal/protocol"
)

// DaemonInterface provides access to daemon functionality from MCP tools.
// This interface abstracts the daemon server for tool handlers.
type DaemonInterface interface {
	// Worker operations
	ListWorkers() []protocol.WorkerInfo
	GetWorker(name string) (*protocol.WorkerDetailInfo, error)

	// Job operations
	ListJobs(status string) []protocol.JobInfo
	GetJob(id string) (*protocol.JobInfo, error)
	CreateJob(description string, priority int, territory string) (*job.Job, error)
	CancelJob(id string) error
	SetJobPriority(id string, priority int) error

	// Activity and status
	ListActivity(limit int) []ActivityEntry
	GetQueueStatus() *protocol.QueueStatusResult

	// Territory info
	ListTerritories() []TerritoryInfo

	// Operation status
	ListOperations() []protocol.OperationInfo

	// Cost summary
	GetCosts() *CostSummary
}

// ActivityEntry represents an activity log entry.
type ActivityEntry struct {
	Time    string `json:"time"`
	Worker  string `json:"worker,omitempty"`
	Message string `json:"message"`
}

// TerritoryInfo represents territory information.
type TerritoryInfo struct {
	Path       string `json:"path"`
	BaseBranch string `json:"base_branch"`
	DevBranch  string `json:"dev_branch,omitempty"`
}

// CostSummary represents a cost summary.
type CostSummary struct {
	TotalCost   string `json:"total_cost"`
	TotalTokens int    `json:"total_tokens"`
	ByWorker    []WorkerCost `json:"by_worker,omitempty"`
}

// WorkerCost represents cost per worker.
type WorkerCost struct {
	Name   string `json:"name"`
	Cost   string `json:"cost"`
	Tokens int    `json:"tokens"`
}
