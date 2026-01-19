// Package protocol implements JSON-RPC 2.0 types for Cosa IPC.
package protocol

import (
	"encoding/json"
)

// JSON-RPC 2.0 version constant
const JSONRPCVersion = "2.0"

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// RequestID can be a string or integer.
type RequestID struct {
	Str *string
	Num *int64
}

func (id *RequestID) MarshalJSON() ([]byte, error) {
	if id == nil {
		return []byte("null"), nil
	}
	if id.Str != nil {
		return json.Marshal(*id.Str)
	}
	if id.Num != nil {
		return json.Marshal(*id.Num)
	}
	return []byte("null"), nil
}

func (id *RequestID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.Str = &s
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		id.Num = &n
		return nil
	}
	return nil
}

// NewStringID creates a RequestID from a string.
func NewStringID(s string) *RequestID {
	return &RequestID{Str: &s}
}

// NewIntID creates a RequestID from an integer.
func NewIntID(n int64) *RequestID {
	return &RequestID{Num: &n}
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Application-specific error codes (-32000 to -32099 reserved for implementation)
const (
	ErrDaemonNotRunning   = -32000
	ErrWorkerNotFound     = -32001
	ErrJobNotFound        = -32002
	ErrInvalidState       = -32003
	ErrTerritoryExists    = -32004
	ErrReviewNotFound     = -32005
	ErrOperationNotFound  = -32006
	ErrGateFailed         = -32007
	ErrMergeConflict      = -32008
)

// NewRequest creates a new JSON-RPC request.
func NewRequest(id *RequestID, method string, params interface{}) (*Request, error) {
	var rawParams json.RawMessage
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		rawParams = p
	}
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}, nil
}

// NewNotification creates a JSON-RPC notification (no ID, no response expected).
func NewNotification(method string, params interface{}) (*Request, error) {
	return NewRequest(nil, method, params)
}

// NewResponse creates a successful JSON-RPC response.
func NewResponse(id *RequestID, result interface{}) (*Response, error) {
	var rawResult json.RawMessage
	if result != nil {
		r, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		rawResult = r
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  rawResult,
	}, nil
}

// NewErrorResponse creates an error JSON-RPC response.
func NewErrorResponse(id *RequestID, code int, message string, data interface{}) (*Response, error) {
	var rawData json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = d
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    rawData,
		},
	}, nil
}

// Method constants for Cosa protocol
const (
	// Daemon lifecycle
	MethodStatus   = "status"
	MethodShutdown = "shutdown"

	// Territory management
	MethodTerritoryInit         = "territory.init"
	MethodTerritoryStatus       = "territory.status"
	MethodTerritoryList         = "territory.list"
	MethodTerritoryAdd          = "territory.add"
	MethodTerritorySetDevBranch = "territory.setDevBranch"

	// Worker management
	MethodWorkerAdd     = "worker.add"
	MethodWorkerList    = "worker.list"
	MethodWorkerStatus  = "worker.status"
	MethodWorkerRemove  = "worker.remove"
	MethodWorkerMessage = "worker.message"
	MethodWorkerDetail  = "worker.detail"

	// Job management
	MethodJobAdd    = "job.add"
	MethodJobList   = "job.list"
	MethodJobStatus = "job.status"
	MethodJobCancel = "job.cancel"
	MethodJobAssign = "job.assign"

	// Queue management
	MethodQueueStatus = "queue.status"

	// Review management
	MethodReviewStart  = "review.start"
	MethodReviewStatus = "review.status"
	MethodReviewList   = "review.list"

	// Operation management
	MethodOperationCreate = "operation.create"
	MethodOperationStatus = "operation.status"
	MethodOperationList   = "operation.list"
	MethodOperationCancel = "operation.cancel"

	// Standing orders management
	MethodOrderSet   = "order.set"
	MethodOrderList  = "order.list"
	MethodOrderClear = "order.clear"

	// Handoff management
	MethodHandoffGenerate = "handoff.generate"

	// Chat with underboss
	MethodChatStart   = "chat.start"
	MethodChatSend    = "chat.send"
	MethodChatEnd     = "chat.end"
	MethodChatHistory = "chat.history"

	// Subscriptions for real-time updates
	MethodSubscribe   = "subscribe"
	MethodUnsubscribe = "unsubscribe"
)

// Notification types for real-time events
const (
	NotifyWorkerUpdated = "worker.updated"
	NotifyJobUpdated    = "job.updated"
	NotifyLogEntry      = "log.entry"
	NotifyActivity      = "activity"
	NotifyChatMessage   = "chat.message"
)

// StatusResult is the response for the status method.
type StatusResult struct {
	Running    bool   `json:"running"`
	Version    string `json:"version"`
	Uptime     int64  `json:"uptime"` // seconds
	Workers    int    `json:"workers"`
	ActiveJobs int    `json:"active_jobs"`
	Territory  string `json:"territory,omitempty"`
	TotalCost  string `json:"total_cost,omitempty"`   // Cumulative cost
	TotalTokens int   `json:"total_tokens,omitempty"` // Cumulative tokens
}

// TerritorySetDevBranchParams are parameters for territory.setDevBranch.
type TerritorySetDevBranchParams struct {
	Branch string `json:"branch"` // Empty string clears the dev branch
}

// TerritorySetDevBranchResult is the result of territory.setDevBranch.
type TerritorySetDevBranchResult struct {
	DevBranch         string `json:"dev_branch"`          // Current dev branch (empty if cleared)
	MergeTargetBranch string `json:"merge_target_branch"` // Effective merge target (dev or base)
}

// WorkerAddParams are parameters for worker.add.
type WorkerAddParams struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"` // defaults to "soldato"
}

// WorkerInfo describes a worker.
type WorkerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CurrentJob string `json:"current_job,omitempty"`
	Worktree  string `json:"worktree,omitempty"`
}

// JobAddParams are parameters for job.add.
type JobAddParams struct {
	Description string   `json:"description"`
	Priority    int      `json:"priority,omitempty"` // 1-5, default 3
	Worker      string   `json:"worker,omitempty"`   // assign to specific worker
	DependsOn   []string `json:"depends_on,omitempty"`
}

// JobInfo describes a job.
type JobInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Worker      string   `json:"worker,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	CreatedAt   int64    `json:"created_at"`
	StartedAt   int64    `json:"started_at,omitempty"`
	CompletedAt int64    `json:"completed_at,omitempty"`
}

// SubscribeParams for subscribing to events.
type SubscribeParams struct {
	Events []string `json:"events"` // event types to subscribe to, or ["*"] for all
}

// WorkerDetailInfo provides detailed information about a worker.
type WorkerDetailInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	CurrentJob    string `json:"current_job,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	Branch        string `json:"branch,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	JobsCompleted int    `json:"jobs_completed"`
	JobsFailed    int    `json:"jobs_failed"`
	TotalCost     string `json:"total_cost,omitempty"`
	TotalTokens   int    `json:"total_tokens,omitempty"`
	CreatedAt     int64  `json:"created_at"`
}

// JobAssignParams are parameters for job.assign.
type JobAssignParams struct {
	JobID    string `json:"job_id"`
	WorkerID string `json:"worker_id"`
}

// QueueStatusResult is the response for queue.status.
type QueueStatusResult struct {
	Ready   int `json:"ready"`   // Jobs ready for execution
	Pending int `json:"pending"` // Jobs waiting on dependencies
	Running int `json:"running"` // Jobs currently executing
	Total   int `json:"total"`   // Total jobs in system
}

// ReviewStartParams are parameters for review.start.
type ReviewStartParams struct {
	JobID string `json:"job_id"`
}

// ReviewStatusParams are parameters for review.status.
type ReviewStatusParams struct {
	JobID string `json:"job_id"`
}

// ReviewStatusResult is the response for review.status.
type ReviewStatusResult struct {
	JobID      string `json:"job_id"`
	WorkerID   string `json:"worker_id"`
	WorkerName string `json:"worker_name"`
	Phase      string `json:"phase"`
	Decision   string `json:"decision,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Feedback   string `json:"feedback,omitempty"`
	Error      string `json:"error,omitempty"`
	StartedAt  int64  `json:"started_at"`
}

// ReviewListResult is the response for review.list.
type ReviewListResult struct {
	Reviews []ReviewStatusResult `json:"reviews"`
}

// OperationCreateParams are parameters for operation.create.
type OperationCreateParams struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Jobs        []string `json:"jobs"` // Job IDs to include
}

// OperationStatusParams are parameters for operation.status.
type OperationStatusParams struct {
	ID string `json:"id"`
}

// OperationInfo describes an operation.
type OperationInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Status        string   `json:"status"`
	Jobs          []string `json:"jobs"`
	TotalJobs     int      `json:"total_jobs"`
	CompletedJobs int      `json:"completed_jobs"`
	FailedJobs    int      `json:"failed_jobs"`
	Progress      int      `json:"progress"` // 0-100
	CreatedAt     int64    `json:"created_at"`
	StartedAt     int64    `json:"started_at,omitempty"`
	CompletedAt   int64    `json:"completed_at,omitempty"`
}

// OperationListResult is the response for operation.list.
type OperationListResult struct {
	Operations []OperationInfo `json:"operations"`
}

// OperationCancelParams are parameters for operation.cancel.
type OperationCancelParams struct {
	ID string `json:"id"`
}

// OrderSetParams are parameters for order.set.
type OrderSetParams struct {
	Worker string   `json:"worker"` // Worker name
	Orders []string `json:"orders"` // Standing orders to set
}

// OrderListParams are parameters for order.list.
type OrderListParams struct {
	Worker string `json:"worker"` // Worker name
}

// OrderListResult is the response for order.list.
type OrderListResult struct {
	Worker string   `json:"worker"`
	Orders []string `json:"orders"`
}

// OrderClearParams are parameters for order.clear.
type OrderClearParams struct {
	Worker string `json:"worker"` // Worker name
}

// HandoffGenerateParams are parameters for handoff.generate.
type HandoffGenerateParams struct {
	Worker string `json:"worker"` // Worker name
}

// HandoffSummary is the response for handoff.generate.
type HandoffSummary struct {
	WorkerID      string   `json:"worker_id"`
	WorkerName    string   `json:"worker_name"`
	JobID         string   `json:"job_id,omitempty"`
	Status        string   `json:"status"`
	Decisions     []string `json:"decisions,omitempty"`
	FilesTouched  []string `json:"files_touched,omitempty"`
	OpenQuestions []string `json:"open_questions,omitempty"`
	CreatedAt     int64    `json:"created_at"`
}

// ChatStartParams are parameters for chat.start.
type ChatStartParams struct {
	SessionID string `json:"session_id,omitempty"` // Optional: resume existing session
}

// ChatStartResult is the response for chat.start.
type ChatStartResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Greeting  string `json:"greeting,omitempty"`
}

// ChatSendParams are parameters for chat.send.
type ChatSendParams struct {
	Message string `json:"message"`
}

// ChatSendResult is the response for chat.send.
type ChatSendResult struct {
	Response string `json:"response"`
}

// ChatHistoryResult is the response for chat.history.
type ChatHistoryResult struct {
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage represents a message in the chat history.
type ChatMessage struct {
	Role      string `json:"role"` // "user" or "assistant"
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}
