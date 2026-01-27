# Cosa Nostra v1 - Complete System Documentation

> A mafia-themed multi-agent development orchestration system for parallel Claude Code workers

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Core Concepts](#3-core-concepts)
4. [Data Models](#4-data-models)
5. [API & Communication](#5-api--communication)
6. [Worker System](#6-worker-system)
7. [Job System](#7-job-system)
8. [Review System](#8-review-system)
9. [Terminal UI](#9-terminal-ui)
10. [Configuration](#10-configuration)
11. [Current Limitations & Pain Points](#11-current-limitations--pain-points)
12. [Improvement Recommendations for v2](#12-improvement-recommendations-for-v2)

---

## 1. Executive Summary

**Cosa Nostra** is a Go-based CLI application that orchestrates multiple Claude Code agents ("workers") operating in parallel on a single codebase. It uses a daemon-client architecture with Unix socket IPC, git worktrees for isolation, and a mafia-themed role hierarchy.

### Key Stats
- **Language**: Go 1.24.0
- **Lines of Code**: ~17,000 lines across ~60 source files
- **Architecture**: Daemon-based with JSON-RPC 2.0 over Unix sockets
- **Two Binaries**: `cosa` (CLI client) and `cosad` (daemon)

### Core Value Proposition
- Run multiple Claude Code instances in parallel without conflicts
- Automatic git worktree isolation per worker/job
- Job queue with priorities and dependencies
- Automated code review with quality gates
- Real-time TUI dashboard for monitoring
- Cost tracking and budget alerts

---

## 2. Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Interface                          │
├─────────────────────────────────────────────────────────────────┤
│  CLI Commands (cosa)          │    TUI Dashboard (cosa tui)     │
│  - cosa worker add            │    - Real-time worker status    │
│  - cosa job add               │    - Job queue visualization    │
│  - cosa review start          │    - Activity feed              │
│  - cosa chat                  │    - Interactive chat           │
└───────────────────────────────┴─────────────────────────────────┘
                                │
                         Unix Socket IPC
                         (JSON-RPC 2.0)
                                │
┌─────────────────────────────────────────────────────────────────┐
│                        Daemon (cosad)                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Server    │  │  Scheduler  │  │     Event Ledger        │  │
│  │  (handlers) │  │  (100ms)    │  │  (JSONL append-only)    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Worker Pool │  │  Job Queue  │  │   Review Coordinator    │  │
│  │             │  │  (priority) │  │  (gates + consigliere)  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Git Manager │  │  Territory  │  │      Notifier           │  │
│  │ (worktrees) │  │  (.cosa/)   │  │  (slack/discord/etc)    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                │
                    ┌───────────┼───────────┐
                    │           │           │
              ┌─────┴─────┐ ┌───┴───┐ ┌─────┴─────┐
              │  Worker 1 │ │ ... n │ │  Worker N │
              │  (claude) │ │       │ │  (claude) │
              │  worktree │ │       │ │  worktree │
              └───────────┘ └───────┘ └───────────┘
```

### Directory Structure

```
cosa/
├── cmd/
│   ├── cosa/main.go              # CLI client entry point (Cobra)
│   └── cosad/main.go             # Daemon entry point
├── internal/
│   ├── claude/                   # Claude Code CLI integration
│   │   ├── client.go             # Process spawning, stream parsing
│   │   ├── parser.go             # stream-json output parser
│   │   └── session.go            # Session persistence
│   ├── config/                   # YAML configuration
│   ├── daemon/                   # Server implementation
│   │   ├── server.go             # Unix socket server, scheduler
│   │   ├── handlers.go           # JSON-RPC method handlers
│   │   ├── handlers_operation.go # Operation handlers
│   │   ├── chat.go               # Interactive chat session
│   │   └── mcp_adapter.go        # MCP protocol adapter
│   ├── git/                      # Git operations
│   │   ├── worktree.go           # Worktree management
│   │   └── merge.go              # Merge and diff operations
│   ├── job/                      # Job management
│   │   ├── job.go                # Job struct and lifecycle
│   │   ├── queue.go              # Priority queue with deps
│   │   ├── operation.go          # Batch job operations
│   │   └── template.go           # Job templates
│   ├── ledger/                   # Event logging (JSONL)
│   ├── mcp/                      # Model Context Protocol
│   ├── notify/                   # Multi-channel notifications
│   ├── protocol/                 # JSON-RPC 2.0 types
│   ├── review/                   # Code review system
│   │   ├── coordinator.go        # Review orchestration
│   │   ├── consigliere.go        # AI code reviewer
│   │   ├── gate.go               # Quality gates (test/build)
│   │   └── decision.go           # Review decisions
│   ├── territory/                # Workspace management
│   ├── tui/                      # Terminal UI (Bubble Tea)
│   │   ├── app.go                # Root application
│   │   ├── page/                 # Dashboard, Chat, etc.
│   │   ├── component/            # Reusable UI components
│   │   ├── styles/               # Lipgloss styles
│   │   └── theme/                # Color themes
│   └── worker/                   # Worker management
│       ├── worker.go             # Worker state machine
│       ├── roles.go              # Role definitions
│       ├── pool.go               # Worker pool
│       └── lookout.go            # Health monitoring
├── bin/                          # Compiled binaries
├── Makefile                      # Build targets
└── go.mod                        # Dependencies
```

### Data Flow

```
User Request → CLI/TUI → Unix Socket → Daemon Handler
                                            │
                    ┌───────────────────────┴───────────────────────┐
                    │                                               │
              Job Created                                     Worker Command
                    │                                               │
              Added to Queue                                  Direct Execution
                    │                                               │
              Scheduler assigns                               Response returned
                    │
              Worker executes in worktree
                    │
              Job completes/fails
                    │
              ┌─────┴─────┐
              │           │
         Auto-Review   Direct Merge
              │           │
         Gate checks      │
              │           │
         AI Review        │
              │           │
         Decision ────────┘
              │
         Merge or Revision
```

---

## 3. Core Concepts

### 3.1 Mafia Hierarchy (Role System)

The system uses a mafia-themed organizational hierarchy:

| Role | Model | Responsibilities | Permissions |
|------|-------|------------------|-------------|
| **Don** | Human | Strategic decisions, final authority | All |
| **Underboss** | Opus | Central orchestrator, job distribution | Delegate, Assign, Supervise all |
| **Capo** | Opus | Territory management, job decomposition | Delegate to Soldatos |
| **Consigliere** | Opus | Code review, quality enforcement | Review only |
| **Soldato** | Sonnet | Named long-lived workers, implementation | Execute jobs |
| **Associate** | Sonnet | Ephemeral workers for one-off tasks | Execute single job |
| **Lookout** | Haiku | Health monitoring, stuck detection | Monitor only |
| **Cleaner** | Haiku | Resource cleanup, garbage collection | Cleanup only |

### 3.2 Territory

A "territory" is a Cosa-managed project workspace:

```
project/
├── .cosa/                    # Territory root
│   ├── territory.json        # Configuration
│   └── worktrees/            # Worker worktrees
│       ├── jobs/             # Job-specific worktrees
│       │   └── {job-id}/
│       └── {worker-name}/
├── src/                      # Project source
└── ...
```

### 3.3 Isolation Model

Each worker operates in isolation:
- **Git Worktree**: Separate working directory with own branch
- **Claude Session**: Independent conversation context
- **No Shared State**: Workers cannot interfere with each other

### 3.4 Job Lifecycle

```
pending → queued → running → completed
                      │          │
                      │     ┌────┴────┐
                      │     ↓         ↓
                      │   review   (merged)
                      │     │
                      └→ failed
                      │
                      └→ cancelled
```

### 3.5 Standing Orders

Workers can have "standing orders" - persistent instructions that apply to all jobs:
- Style guidelines
- Architecture patterns to follow
- Files to avoid modifying
- Testing requirements

---

## 4. Data Models

### 4.1 Job

```go
type Job struct {
    ID              string      // UUID
    Description     string      // Task description
    Status          Status      // pending|queued|running|completed|failed|cancelled|review
    Priority        int         // 1 (low) to 5 (critical)
    Worker          string      // Assigned worker ID
    DependsOn       []string    // Job IDs this depends on
    Operation       string      // Parent operation ID

    // Timing
    CreatedAt       time.Time
    QueuedAt        *time.Time
    StartedAt       *time.Time
    CompletedAt     *time.Time

    // Git
    Worktree        string      // Path to job's git worktree
    Branch          string      // Branch name

    // Claude
    SessionID       string      // Session for resumption

    // Results
    Error           string
    Output          string

    // Cost
    TotalCost       string
    TotalTokens     int

    // Review
    ReviewFeedback  []string
    RevisionOf      string      // Previous job if revision
}
```

### 4.2 Worker

```go
type Worker struct {
    ID                  string
    Name                string
    Role                Role        // don|underboss|capo|soldato|associate|...
    Status              Status      // idle|working|reviewing|stopped|error
    Worktree            string
    Branch              string
    CreatedAt           time.Time

    Ephemeral           bool        // Auto-cleanup after job
    StandingOrders      []string
    MergeTargetBranch   string

    CurrentJob          *Job
    SessionID           string

    // Stats
    JobsCompleted       int
    JobsFailed          int
    TotalCost           string
    TotalTokens         int
    LastActivityAt      time.Time
}
```

### 4.3 Operation

```go
type Operation struct {
    ID              string
    Name            string
    Description     string
    Status          OperationStatus  // pending|running|completed|failed|cancelled
    Jobs            []string         // Job IDs

    CreatedAt       time.Time
    StartedAt       *time.Time
    CompletedAt     *time.Time

    TotalJobs       int
    CompletedJobs   int
    FailedJobs      int
}
```

### 4.4 Template

```go
type Template struct {
    ID          string
    Name        string
    Description string
    Type        TemplateType     // refactor|test|document|review|custom
    Prompt      string           // With {{variable}} placeholders
    Priority    int
    Variables   []TemplateVar
    Tags        []string
    BuiltIn     bool
}

type TemplateVar struct {
    Name        string
    Description string
    Required    bool
    Default     string
}
```

### 4.5 Event (Ledger)

```go
type Event struct {
    ID          string
    Type        EventType
    Timestamp   time.Time
    Data        json.RawMessage
}

// Event types:
// daemon.started, daemon.stopped
// territory.init
// worker.added, worker.started, worker.stopped, worker.removed, worker.error, worker.stuck
// job.created, job.queued, job.started, job.completed, job.failed, job.cancelled
// review.started, review.approved, review.rejected, review.failed, review.phase
// gate.started, gate.passed, gate.failed
// merge.started, merge.completed, merge.failed
// cost.record
```

---

## 5. API & Communication

### 5.1 Protocol

- **Transport**: Unix domain socket (`~/.cosa/cosa.sock`)
- **Protocol**: JSON-RPC 2.0
- **Format**: Line-delimited JSON

### 5.2 API Endpoints (42 methods)

#### Daemon
| Method | Description |
|--------|-------------|
| `status` | Daemon status, uptime, counts, costs |
| `shutdown` | Graceful shutdown |

#### Territory
| Method | Description |
|--------|-------------|
| `territory.init` | Initialize territory in path |
| `territory.status` | Current territory details |
| `territory.list` | List all territories |
| `territory.add` | Add existing territory |
| `territory.setDevBranch` | Set/clear dev branch |

#### Workers
| Method | Description |
|--------|-------------|
| `worker.add` | Create new worker |
| `worker.list` | List all workers |
| `worker.status` | Quick worker status |
| `worker.detail` | Detailed worker info |
| `worker.remove` | Remove and cleanup |
| `worker.message` | Send message to active worker |

#### Jobs
| Method | Description |
|--------|-------------|
| `job.add` | Create new job |
| `job.list` | List jobs (with filters) |
| `job.status` | Get job details |
| `job.cancel` | Cancel pending/queued job |
| `job.assign` | Assign to specific worker |
| `job.reassign` | Retry failed job |
| `job.setPriority` | Adjust priority |

#### Queue
| Method | Description |
|--------|-------------|
| `queue.status` | Queue statistics |

#### Templates
| Method | Description |
|--------|-------------|
| `template.list` | List templates |
| `template.get` | Get template with prompt |
| `template.use` | Create job from template |

#### Reviews
| Method | Description |
|--------|-------------|
| `review.start` | Begin code review |
| `review.status` | Review progress |
| `review.list` | List active reviews |

#### Operations
| Method | Description |
|--------|-------------|
| `operation.create` | Create batch operation |
| `operation.status` | Operation progress |
| `operation.list` | List operations |
| `operation.cancel` | Cancel operation |

#### Orders
| Method | Description |
|--------|-------------|
| `order.set` | Set standing orders |
| `order.list` | Get standing orders |
| `order.clear` | Clear standing orders |

#### Chat
| Method | Description |
|--------|-------------|
| `chat.start` | Start chat with Underboss |
| `chat.send` | Send message |
| `chat.end` | End chat session |
| `chat.history` | Get message history |

#### Subscriptions
| Method | Description |
|--------|-------------|
| `subscribe` | Subscribe to events |
| `unsubscribe` | Unsubscribe |

### 5.3 Error Codes

```go
// Standard JSON-RPC
ParseError      = -32700
InvalidRequest  = -32600
MethodNotFound  = -32601
InvalidParams   = -32602
InternalError   = -32603

// Application-specific
ErrDaemonNotRunning  = -32000
ErrWorkerNotFound    = -32001
ErrJobNotFound       = -32002
ErrInvalidState      = -32003
ErrTerritoryExists   = -32004
ErrReviewNotFound    = -32005
ErrOperationNotFound = -32006
ErrGateFailed        = -32007
ErrMergeConflict     = -32008
ErrTemplateNotFound  = -32009
```

---

## 6. Worker System

### 6.1 Worker State Machine

```
         ┌─────────────────────────────────┐
         │                                 │
         ↓                                 │
    ┌─────────┐     job assigned     ┌─────────────┐
    │  idle   │─────────────────────→│   working   │
    └─────────┘                      └─────────────┘
         ↑                                 │
         │      job complete/fail          │
         └─────────────────────────────────┘
         │                                 │
         │                                 ↓
    ┌─────────┐                      ┌─────────────┐
    │ stopped │                      │  reviewing  │
    └─────────┘                      └─────────────┘
         ↑                                 │
         │                                 │
    ┌─────────┐                            │
    │  error  │←───────────────────────────┘
    └─────────┘     (on error)
```

### 6.2 Worker Pool

The pool manages worker lifecycle:
- **Registration**: Add worker with role and worktree
- **Assignment**: Match idle workers to ready jobs
- **Monitoring**: Track activity timestamps
- **Cleanup**: Remove workers and their resources

### 6.3 Claude Integration

Each worker spawns a Claude Code subprocess:

```go
type Client struct {
    cmd         *exec.Cmd
    stdin       io.WriteCloser
    events      chan Event
    sessionID   string
}
```

**Execution flags**:
```
claude --print --verbose --dangerously-skip-permissions --output-format stream-json
```

**Session resumption**: Sessions are persisted and can be resumed for follow-up work.

### 6.4 Lookout (Health Monitoring)

Background service that:
- Detects stuck workers (no activity for threshold)
- Sends notifications
- Can trigger remediation

---

## 7. Job System

### 7.1 Priority Queue

Jobs are queued with priority ordering:
- **Priority 5 (Critical)** first
- **Priority 1 (Low)** last
- **FIFO** within same priority

### 7.2 Dependency Resolution

Jobs can depend on other jobs:
```go
job1 := job.New("Create database schema")
job2 := job.New("Create API endpoints")
job2.SetDependencies([]string{job1.ID})  // job2 waits for job1
```

Dependency handling:
- Jobs go to "pending" if dependencies aren't met
- On job completion, dependent jobs become "ready"
- On job failure, dependents cascade to "failed"

### 7.3 Scheduler

The scheduler runs every 100ms:
1. Find "ready" jobs (queued, deps met)
2. Find idle workers (by role)
3. Match jobs to workers
4. Create job worktree
5. Start execution

### 7.4 Operations (Batch Jobs)

Operations group related jobs:
```go
op := job.NewOperation("Refactor authentication")
op.AddJobs([]string{job1.ID, job2.ID, job3.ID})
```

Progress tracked automatically:
- Total jobs, completed, failed
- Overall status derived from job statuses

### 7.5 Templates

Pre-defined job configurations:

**Built-in templates**:
- `refactor-file`, `refactor-module`, `refactor-function`
- `test-unit`, `test-integration`, `test-fix`
- `document-file`, `document-api`, `document-architecture`
- `review-code`, `review-security`, `review-performance`

**Template expansion**:
```
"Refactor the {{filename}} to use {{pattern}}"
→ "Refactor the auth.go to use dependency injection"
```

---

## 8. Review System

### 8.1 Quality Gates

Before AI review, automated checks run:

| Gate | Command | Purpose |
|------|---------|---------|
| Build | `go build ./...` | Compilation check |
| Test | `go test ./...` | Test suite |

Gate execution:
- 5-minute timeout
- Skips subsequent gates on failure
- Results logged to ledger

### 8.2 Consigliere (AI Reviewer)

If gates pass, the Consigliere (Opus) reviews:
1. Receives diff of changes
2. Analyzes for issues
3. Returns decision:
   - **Approve**: Merge to target branch
   - **Request Revision**: Create new job with feedback
   - **Reject**: Mark as review failed

### 8.3 Review Flow

```
Job Completed
      │
      ↓
┌─────────────┐
│ Build Gate  │───fail───→ Review Failed
└─────────────┘
      │ pass
      ↓
┌─────────────┐
│ Test Gate   │───fail───→ Review Failed
└─────────────┘
      │ pass
      ↓
┌─────────────┐
│ Generate    │
│ Diff        │
└─────────────┘
      │
      ↓
┌─────────────┐
│ Consigliere │
│ AI Review   │
└─────────────┘
      │
      ├──approve──→ Merge to target
      │
      ├──revise───→ Create revision job
      │
      └──reject───→ Review failed
```

---

## 9. Terminal UI

### 9.1 Framework

- **Bubble Tea**: Terminal UI framework
- **Lipgloss**: Declarative styling

### 9.2 Pages

| Page | Purpose |
|------|---------|
| **Dashboard** | 3-panel layout: Workers, Jobs, Activity |
| **Chat** | Conversational interface with Underboss |
| **Worker Detail** | Detailed worker view with session output |
| **Operation View** | Operation progress tracking |

### 9.3 Components

| Component | Purpose |
|-----------|---------|
| WorkerList | Worker status display |
| JobList | Job queue display |
| Activity | Event feed |
| Dialog | Modal input |
| Input | Text field |
| TextArea | Multi-line input |
| TemplateSelector | Template picker |
| CommandPalette | Command search |

### 9.4 Themes

Three built-in themes:
- **Noir** (default): Dark goldenrod, film noir aesthetic
- **Godfather**: Dark red + gold
- **Miami**: 80s vice aesthetic (pink + turquoise)

### 9.5 Key Bindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `c` | Open Chat |
| `n` | New Job |
| `t` | Template Selector |
| `o` | New Operation |
| `/` | Search |
| `:` | Command Palette |
| `j/k` | Navigate |
| `Tab` | Switch panels |
| `Enter` | Select |
| `Esc` | Close/Back |
| `r` | Refresh |
| `R` | Reassign failed job |

---

## 10. Configuration

### 10.1 Config File

Location: `~/.cosa/config.yaml`

```yaml
# Core
socket_path: ~/.cosa/cosa.sock
data_dir: ~/.cosa
log_level: info

# Claude
claude:
  binary: claude
  model: ""  # Use role defaults
  max_turns: 100
  chat_timeout: 300

# Workers
workers:
  max_concurrent: 5
  default_role: soldato

# Git
git:
  default_merge_branch: ""  # Use repo default

# TUI
tui:
  theme: noir
  refresh_rate: 100

# Notifications
notifications:
  tui_alerts: true
  system_notifications: false
  terminal_bell: false
  on_job_complete: true
  on_job_failed: true
  on_worker_stuck: true
  on_budget_alert: true

  budget:
    limit: 0  # No limit
    warning_threshold: 80

  slack:
    enabled: false
    webhook_url: ""

  discord:
    enabled: false
    webhook_url: ""

  webhook:
    enabled: false
    url: ""
    secret: ""

# Models per role
models:
  default: sonnet
  underboss: opus
  consigliere: opus
  capo: opus
  soldato: sonnet
  associate: sonnet
  lookout: haiku
  cleaner: haiku
```

### 10.2 Data Directories

```
~/.cosa/
├── cosa.sock          # Unix socket
├── cosad.pid          # Daemon PID
├── ledger.jsonl       # Event log
├── state.json         # Runtime state
├── config.yaml        # Configuration
├── jobs/              # Job persistence
├── workers/           # Worker persistence
├── sessions/          # Claude sessions
└── templates/         # Custom templates
```

---

## 11. Current Limitations & Pain Points

### 11.1 Architecture Issues

1. **Single Daemon**: All workers share one daemon process
   - No horizontal scaling
   - Daemon crash kills all workers
   - No distributed operation

2. **In-Memory State**: Most state is in-memory with JSON persistence
   - No transactional guarantees
   - Potential data loss on crash
   - No query capabilities

3. **Unix Socket Only**: Limited to local machine
   - Can't run workers on remote machines
   - No cloud deployment option

4. **Tight Claude Coupling**: Hard-coded to Claude Code CLI
   - Can't use other AI providers
   - Can't use Claude API directly (more efficient)

### 11.2 Worker Issues

1. **Worktree Overhead**: Creating/removing worktrees is slow
   - Each job creates full worktree
   - Cleanup can be unreliable

2. **Session Management**: Session resumption is fragile
   - Sessions can become stale
   - No automatic cleanup
   - Memory leaks over time

3. **No Streaming Output**: Can't see worker progress in real-time
   - Only see final result
   - Long jobs feel unresponsive

4. **Role Rigidity**: Roles are somewhat arbitrary
   - Permissions system not fully utilized
   - Model assignments feel arbitrary

### 11.3 Job Issues

1. **Limited Dependency Model**: Only "depends on" relationship
   - No parallel groups
   - No conditional execution
   - No retry policies

2. **No Job Persistence Recovery**: Jobs lost if daemon crashes mid-execution
   - No checkpoint/restart
   - No idempotency guarantees

3. **Queue Bottleneck**: Single queue for all workers
   - No worker-specific queues
   - No workload isolation

4. **Template System**: Basic variable substitution only
   - No conditionals
   - No loops
   - No computed values

### 11.4 Review Issues

1. **Binary Gates**: Pass/fail only
   - No warnings
   - No partial passes
   - No custom severity

2. **No Incremental Review**: Reviews entire diff every time
   - Expensive for large changes
   - No caching

3. **Revision Chain**: Revisions create new jobs
   - Loses context
   - Can loop indefinitely
   - No revision limit

### 11.5 UI Issues

1. **TUI Only**: No web UI option
   - Can't share dashboards
   - No mobile access
   - Limited visualization

2. **No Persistent History**: Activity feed is transient
   - Can't search past events
   - No analytics

3. **Limited Filtering**: Job/worker lists have basic filters
   - No complex queries
   - No saved views

### 11.6 Operational Issues

1. **No Logging**: Only ledger events
   - No debug logs
   - Hard to troubleshoot

2. **No Metrics**: No observability
   - No Prometheus/Grafana
   - No performance tracking

3. **No Backup/Restore**: Data is just files
   - No point-in-time recovery
   - No migration tools

---

## 12. Improvement Recommendations for v2

### 12.1 Architecture Redesign

#### Consider Event Sourcing + CQRS
```
Commands → Event Store → Projections → Read Models
                │
                └→ Event Handlers (workers, notifications, etc.)
```

Benefits:
- Full audit trail (already have ledger)
- Rebuild state from events
- Multiple read models for different views
- Better scaling

#### Consider Actor Model
```
Supervisor Actor
    │
    ├── Worker Actor 1 (isolated state)
    ├── Worker Actor 2 (isolated state)
    └── Worker Actor N (isolated state)
```

Benefits:
- Natural isolation
- Failure recovery
- Message-based communication
- Scalable

#### Consider gRPC + Protobuf
Instead of JSON-RPC over Unix sockets:
- Strongly typed contracts
- Efficient serialization
- Streaming support
- Code generation

### 12.2 Storage Improvements

#### Use SQLite for State
```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    description TEXT,
    status TEXT,
    priority INTEGER,
    worker_id TEXT,
    created_at TIMESTAMP,
    -- etc
);

CREATE TABLE events (
    id TEXT PRIMARY KEY,
    type TEXT,
    timestamp TIMESTAMP,
    data JSONB,
    -- indexes for queries
);
```

Benefits:
- ACID transactions
- Query capabilities
- Single file, portable
- No external dependencies

#### Keep JSONL for Audit Log
- Append-only is perfect for logs
- Easy to stream/tail
- Human readable
- Can rebuild SQLite from log

### 12.3 Worker Improvements

#### Direct API Integration
Instead of spawning `claude` CLI:
```go
client := anthropic.NewClient(apiKey)
response, err := client.CreateMessage(ctx, &anthropic.MessageRequest{
    Model: "claude-3-opus",
    Messages: messages,
    Tools: tools,
})
```

Benefits:
- No subprocess overhead
- Streaming responses
- Better error handling
- Direct cost access

#### Persistent Worker Pools
Instead of worktree-per-job:
- Long-running workers with stable worktrees
- Job branches within worker worktree
- Reuse Claude sessions
- Faster job startup

#### Worker Specialization
```go
type WorkerCapabilities struct {
    Languages    []string  // go, python, typescript
    Frameworks   []string  // react, django, gin
    TaskTypes    []string  // refactor, test, docs
}
```

Match jobs to workers by capability, not just availability.

### 12.4 Job System Improvements

#### Workflow Engine
```go
type Workflow struct {
    ID    string
    Steps []Step
}

type Step struct {
    ID       string
    Type     StepType  // job, gate, condition, parallel, wait
    Config   StepConfig
    OnSuccess string   // next step
    OnFailure string   // error handling step
}
```

Support:
- Parallel execution groups
- Conditional branching
- Retry policies
- Timeouts
- Manual approval gates

#### Job Checkpointing
```go
type Checkpoint struct {
    JobID     string
    StepIndex int
    State     json.RawMessage
    Timestamp time.Time
}
```

Enable:
- Resume from checkpoint on failure
- Idempotent execution
- Progress tracking

### 12.5 Review System Improvements

#### Incremental Review
- Cache file-level reviews
- Only re-review changed files
- Aggregate results

#### Review Policies
```yaml
reviews:
  auto_approve:
    - path: "docs/**"
    - path: "*.md"
  require_human:
    - path: "internal/security/**"
  max_revisions: 3
```

#### Multi-Reviewer
- Primary reviewer (Consigliere)
- Secondary reviewer (another model)
- Human final approval option

### 12.6 UI Improvements

#### Web Dashboard
- Real-time WebSocket updates
- Shareable links
- Mobile responsive
- Rich visualizations (Gantt charts, dependency graphs)

#### CLI Improvements
```bash
cosa watch           # Live tail of activity
cosa job logs <id>   # Stream job output
cosa diff <id>       # View job diff
cosa replay <id>     # Replay job execution
```

### 12.7 Observability

#### Structured Logging
```go
logger.Info("job started",
    "job_id", job.ID,
    "worker_id", worker.ID,
    "description", job.Description,
)
```

#### Metrics
```go
jobsTotal.WithLabels(status).Inc()
jobDuration.Observe(duration.Seconds())
workerUtilization.Set(activeWorkers / totalWorkers)
```

Export to Prometheus, visualize in Grafana.

#### Tracing
Distributed tracing for job execution:
```
Job Created → Queued → Assigned → Worker Started → Claude Session → Completed
    span 1      span 2    span 3       span 4          span 5        span 6
```

### 12.8 API Improvements

#### Versioned API
```
/v1/jobs
/v1/workers
/v2/jobs  # New version with breaking changes
```

#### OpenAPI/Swagger
Generate documentation and client SDKs.

#### Webhooks
```yaml
webhooks:
  - url: https://example.com/hook
    events:
      - job.completed
      - job.failed
    secret: xxx
```

### 12.9 Security Improvements

#### API Keys
```yaml
api_keys:
  - name: ci-bot
    key: cosa_xxx
    permissions:
      - jobs:read
      - jobs:create
```

#### Audit Log Signing
```go
type SignedEvent struct {
    Event     Event
    Signature string  // HMAC of event + previous signature
}
```

Tamper-evident audit trail.

### 12.10 Simplification Opportunities

1. **Remove Role Complexity**: Simplify to worker types (executor, reviewer)
2. **Remove Operation Abstraction**: Jobs with dependencies are sufficient
3. **Remove Standing Orders**: Use templates instead
4. **Remove Chat**: Focus on job-based interaction
5. **Remove MCP**: Direct tool calling is simpler

### 12.11 New Features to Consider

1. **Cost Budgets**: Per-job, per-worker, per-project budgets
2. **Job Scheduling**: Cron-like scheduled jobs
3. **Branch Strategies**: Support different git workflows
4. **Team Features**: Multiple users, permissions, shared dashboards
5. **Plugins**: Extensible gate/review system
6. **Cloud Sync**: Sync state across machines

---

## Summary

Cosa v1 is a sophisticated system with solid foundations:
- Clean daemon-client architecture
- Well-designed data models
- Comprehensive event logging
- Rich TUI

For v2, the main opportunities are:
1. **Better persistence** (SQLite vs JSON files)
2. **Direct Claude API** (vs CLI subprocess)
3. **Workflow engine** (vs simple queue)
4. **Observability** (logging, metrics, tracing)
5. **Simplification** (remove unused complexity)

The mafia theming is fun but the role system could be simplified. The core value - parallel isolated Claude workers - should remain central.
