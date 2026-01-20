package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolHandler handles a tool call and returns the result.
type ToolHandler func(args json.RawMessage, daemon DaemonInterface) CallToolResult

// ToolRegistry manages tool definitions and handlers.
type ToolRegistry struct {
	tools    []Tool
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry with Cosa tools.
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools:    make([]Tool, 0),
		handlers: make(map[string]ToolHandler),
	}
	r.registerCosaTools()
	return r
}

// Tools returns all registered tools.
func (r *ToolRegistry) Tools() []Tool {
	return r.tools
}

// Call invokes a tool by name.
func (r *ToolRegistry) Call(name string, args json.RawMessage, daemon DaemonInterface) (CallToolResult, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return ToolError(fmt.Sprintf("unknown tool: %s", name)), fmt.Errorf("unknown tool: %s", name)
	}
	return handler(args, daemon), nil
}

func (r *ToolRegistry) register(tool Tool, handler ToolHandler) {
	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
}

func (r *ToolRegistry) registerCosaTools() {
	// cosa_list_workers - List all workers
	r.register(
		Tool{
			Name:        "cosa_list_workers",
			Description: "List all workers in the Cosa organization with their status and current job",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		handleListWorkers,
	)

	// cosa_get_worker - Get worker details
	r.register(
		Tool{
			Name:        "cosa_get_worker",
			Description: "Get detailed information about a specific worker",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {
						Type:        "string",
						Description: "Name of the worker",
					},
				},
				Required: []string{"name"},
			},
		},
		handleGetWorker,
	)

	// cosa_list_jobs - List jobs
	r.register(
		Tool{
			Name:        "cosa_list_jobs",
			Description: "List jobs in the system, optionally filtered by status",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"status": {
						Type:        "string",
						Description: "Filter by job status",
						Enum:        []string{"pending", "queued", "running", "completed", "failed", "cancelled"},
					},
				},
			},
		},
		handleListJobs,
	)

	// cosa_get_job - Get job details
	r.register(
		Tool{
			Name:        "cosa_get_job",
			Description: "Get detailed information about a specific job",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id": {
						Type:        "string",
						Description: "Job ID",
					},
				},
				Required: []string{"id"},
			},
		},
		handleGetJob,
	)

	// cosa_list_activity - List recent activity
	r.register(
		Tool{
			Name:        "cosa_list_activity",
			Description: "List recent activity events in the system",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit": {
						Type:        "integer",
						Description: "Maximum number of events to return (default 20)",
					},
				},
			},
		},
		handleListActivity,
	)

	// cosa_list_territories - List territories
	r.register(
		Tool{
			Name:        "cosa_list_territories",
			Description: "List configured territories (git repositories) in Cosa",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		handleListTerritories,
	)

	// cosa_list_operations - List operations
	r.register(
		Tool{
			Name:        "cosa_list_operations",
			Description: "List all operations and their status",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		handleListOperations,
	)

	// cosa_get_costs - Get cost summary
	r.register(
		Tool{
			Name:        "cosa_get_costs",
			Description: "Get a summary of costs and token usage",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		handleGetCosts,
	)

	// cosa_create_job - Create a new job
	r.register(
		Tool{
			Name:        "cosa_create_job",
			Description: "Create a new job for workers to execute",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"description": {
						Type:        "string",
						Description: "Description of the job to be done",
					},
					"priority": {
						Type:        "integer",
						Description: "Priority level 1-5 (1=highest, 5=lowest, default 3)",
					},
					"territory": {
						Type:        "string",
						Description: "Optional territory to assign the job to",
					},
				},
				Required: []string{"description"},
			},
		},
		handleCreateJob,
	)

	// cosa_cancel_job - Cancel a job
	r.register(
		Tool{
			Name:        "cosa_cancel_job",
			Description: "Cancel a pending or running job",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id": {
						Type:        "string",
						Description: "Job ID to cancel",
					},
				},
				Required: []string{"id"},
			},
		},
		handleCancelJob,
	)

	// cosa_set_job_priority - Update job priority
	r.register(
		Tool{
			Name:        "cosa_set_job_priority",
			Description: "Update the priority of a job",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id": {
						Type:        "string",
						Description: "Job ID",
					},
					"priority": {
						Type:        "integer",
						Description: "New priority level 1-5 (1=highest, 5=lowest)",
					},
				},
				Required: []string{"id", "priority"},
			},
		},
		handleSetJobPriority,
	)

	// cosa_queue_status - Get queue status
	r.register(
		Tool{
			Name:        "cosa_queue_status",
			Description: "Get the current job queue status",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		handleQueueStatus,
	)
}

// Tool handlers

func handleListWorkers(_ json.RawMessage, daemon DaemonInterface) CallToolResult {
	workers := daemon.ListWorkers()
	if len(workers) == 0 {
		return ToolSuccess("No workers currently in the organization.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d workers:\n\n", len(workers)))

	for _, w := range workers {
		status := w.Status
		job := "idle"
		if w.CurrentJobDesc != "" {
			job = w.CurrentJobDesc
		}
		sb.WriteString(fmt.Sprintf("• %s (%s) - %s: %s\n", w.Name, w.Role, status, job))
	}

	return ToolSuccess(sb.String())
}

func handleGetWorker(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolError(fmt.Sprintf("invalid arguments: %v", err))
	}

	worker, err := daemon.GetWorker(params.Name)
	if err != nil {
		return ToolError(fmt.Sprintf("worker not found: %s", params.Name))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Worker: %s\n", worker.Name))
	sb.WriteString(fmt.Sprintf("Role: %s\n", worker.Role))
	sb.WriteString(fmt.Sprintf("Status: %s\n", worker.Status))
	sb.WriteString(fmt.Sprintf("Branch: %s\n", worker.Branch))
	sb.WriteString(fmt.Sprintf("Jobs Completed: %d\n", worker.JobsCompleted))
	sb.WriteString(fmt.Sprintf("Jobs Failed: %d\n", worker.JobsFailed))
	if worker.TotalCost != "" {
		sb.WriteString(fmt.Sprintf("Total Cost: %s (%d tokens)\n", worker.TotalCost, worker.TotalTokens))
	}
	if worker.CurrentJob != "" {
		sb.WriteString(fmt.Sprintf("Current Job: %s\n", worker.CurrentJob))
	}

	return ToolSuccess(sb.String())
}

func handleListJobs(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		Status string `json:"status"`
	}
	json.Unmarshal(args, &params)

	jobs := daemon.ListJobs(params.Status)
	if len(jobs) == 0 {
		if params.Status != "" {
			return ToolSuccess(fmt.Sprintf("No jobs with status '%s'.", params.Status))
		}
		return ToolSuccess("No jobs in the system.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d jobs:\n\n", len(jobs)))

	for _, j := range jobs {
		worker := j.Worker
		if worker == "" {
			worker = "unassigned"
		}
		sb.WriteString(fmt.Sprintf("• [%s] %s (P%d) - %s: %s\n",
			j.ID[:8], j.Status, j.Priority, worker, truncate(j.Description, 50)))
	}

	return ToolSuccess(sb.String())
}

func handleGetJob(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolError(fmt.Sprintf("invalid arguments: %v", err))
	}

	job, err := daemon.GetJob(params.ID)
	if err != nil {
		return ToolError(fmt.Sprintf("job not found: %s", params.ID))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Job: %s\n", job.ID))
	sb.WriteString(fmt.Sprintf("Description: %s\n", job.Description))
	sb.WriteString(fmt.Sprintf("Status: %s\n", job.Status))
	sb.WriteString(fmt.Sprintf("Priority: %d\n", job.Priority))
	if job.Worker != "" {
		sb.WriteString(fmt.Sprintf("Worker: %s\n", job.Worker))
	}
	if len(job.DependsOn) > 0 {
		sb.WriteString(fmt.Sprintf("Depends On: %v\n", job.DependsOn))
	}

	return ToolSuccess(sb.String())
}

func handleListActivity(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		Limit int `json:"limit"`
	}
	json.Unmarshal(args, &params)

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	activities := daemon.ListActivity(limit)
	if len(activities) == 0 {
		return ToolSuccess("No recent activity.")
	}

	var sb strings.Builder
	sb.WriteString("Recent activity:\n\n")

	for _, a := range activities {
		if a.Worker != "" {
			sb.WriteString(fmt.Sprintf("• %s [%s] %s\n", a.Time, a.Worker, a.Message))
		} else {
			sb.WriteString(fmt.Sprintf("• %s %s\n", a.Time, a.Message))
		}
	}

	return ToolSuccess(sb.String())
}

func handleListTerritories(_ json.RawMessage, daemon DaemonInterface) CallToolResult {
	territories := daemon.ListTerritories()
	if len(territories) == 0 {
		return ToolSuccess("No territories configured.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d territories:\n\n", len(territories)))

	for _, t := range territories {
		sb.WriteString(fmt.Sprintf("• %s\n", t.Path))
		sb.WriteString(fmt.Sprintf("  Base branch: %s\n", t.BaseBranch))
		if t.DevBranch != "" {
			sb.WriteString(fmt.Sprintf("  Dev branch: %s\n", t.DevBranch))
		}
	}

	return ToolSuccess(sb.String())
}

func handleListOperations(_ json.RawMessage, daemon DaemonInterface) CallToolResult {
	operations := daemon.ListOperations()
	if len(operations) == 0 {
		return ToolSuccess("No operations.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d operations:\n\n", len(operations)))

	for _, o := range operations {
		sb.WriteString(fmt.Sprintf("• %s (%s)\n", o.Name, o.Status))
		sb.WriteString(fmt.Sprintf("  Progress: %d%% (%d/%d jobs)\n", o.Progress, o.CompletedJobs, o.TotalJobs))
		if o.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", o.Description))
		}
	}

	return ToolSuccess(sb.String())
}

func handleGetCosts(_ json.RawMessage, daemon DaemonInterface) CallToolResult {
	costs := daemon.GetCosts()
	if costs == nil {
		return ToolSuccess("No cost data available.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total Cost: %s\n", costs.TotalCost))
	sb.WriteString(fmt.Sprintf("Total Tokens: %d\n", costs.TotalTokens))

	if len(costs.ByWorker) > 0 {
		sb.WriteString("\nBy Worker:\n")
		for _, w := range costs.ByWorker {
			sb.WriteString(fmt.Sprintf("• %s: %s (%d tokens)\n", w.Name, w.Cost, w.Tokens))
		}
	}

	return ToolSuccess(sb.String())
}

func handleCreateJob(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Territory   string `json:"territory"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolError(fmt.Sprintf("invalid arguments: %v", err))
	}

	if params.Description == "" {
		return ToolError("description is required")
	}

	priority := params.Priority
	if priority < 1 || priority > 5 {
		priority = 3
	}

	job, err := daemon.CreateJob(params.Description, priority, params.Territory)
	if err != nil {
		return ToolError(fmt.Sprintf("failed to create job: %v", err))
	}

	return ToolSuccess(fmt.Sprintf("Job created: %s\nDescription: %s\nPriority: %d",
		job.ID, job.Description, job.Priority))
}

func handleCancelJob(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolError(fmt.Sprintf("invalid arguments: %v", err))
	}

	if err := daemon.CancelJob(params.ID); err != nil {
		return ToolError(fmt.Sprintf("failed to cancel job: %v", err))
	}

	return ToolSuccess(fmt.Sprintf("Job %s cancelled.", params.ID))
}

func handleSetJobPriority(args json.RawMessage, daemon DaemonInterface) CallToolResult {
	var params struct {
		ID       string `json:"id"`
		Priority int    `json:"priority"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolError(fmt.Sprintf("invalid arguments: %v", err))
	}

	if params.Priority < 1 || params.Priority > 5 {
		return ToolError("priority must be between 1 and 5")
	}

	if err := daemon.SetJobPriority(params.ID, params.Priority); err != nil {
		return ToolError(fmt.Sprintf("failed to set priority: %v", err))
	}

	return ToolSuccess(fmt.Sprintf("Job %s priority set to %d.", params.ID, params.Priority))
}

func handleQueueStatus(_ json.RawMessage, daemon DaemonInterface) CallToolResult {
	status := daemon.GetQueueStatus()
	if status == nil {
		return ToolSuccess("Queue status unavailable.")
	}

	var sb strings.Builder
	sb.WriteString("Queue Status:\n")
	sb.WriteString(fmt.Sprintf("• Ready: %d jobs\n", status.Ready))
	sb.WriteString(fmt.Sprintf("• Pending: %d jobs\n", status.Pending))
	sb.WriteString(fmt.Sprintf("• Running: %d jobs\n", status.Running))
	sb.WriteString(fmt.Sprintf("• Total: %d jobs\n", status.Total))

	return ToolSuccess(sb.String())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}
