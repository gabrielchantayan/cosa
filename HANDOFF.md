# Cosa Nostra - Agent Handoff Document

## Project Overview

Cosa Nostra is a mafia-themed multi-agent development orchestration system built in Go. It manages Claude Code workers in isolated git worktrees with a hierarchical role system, real-time TUI, and JSON-RPC 2.0 IPC.

## Current State

**Phases Completed: 1-4**
- Phase 1: Foundation (daemon, CLI, protocol, config, ledger)
- Phase 2: Single Worker (claude client, worker, git worktree, jobs, territory)
- Phase 3: Basic TUI (dashboard, themes, components)
- Phase 4: Multi-Worker Orchestration (roles, queue, pool, sessions, scheduler)

**Phases Remaining: 5-7**
- Phase 5: Code Review Flow
- Phase 6: Operations & Advanced
- Phase 7: Polish

## Architecture

### Package Structure
```
cosa/
├── cmd/
│   ├── cosa/main.go           # CLI client (cobra commands)
│   └── cosad/main.go          # Daemon entry point
├── internal/
│   ├── claude/                 # Claude Code CLI integration
│   │   ├── client.go          # Spawns claude process, manages session
│   │   ├── parser.go          # Parses stream-json output
│   │   └── session.go         # Session persistence for resume [Phase 4]
│   ├── config/config.go       # YAML config (~/.cosa/config.yaml)
│   ├── daemon/
│   │   ├── server.go          # Unix socket server, scheduler, pool/queue [Phase 4]
│   │   ├── client.go          # Client for CLI to connect
│   │   └── handlers.go        # JSON-RPC method handlers
│   ├── git/worktree.go        # Git worktree operations
│   ├── job/
│   │   ├── job.go             # Job struct, Store, lifecycle
│   │   └── queue.go           # Priority queue with dependency resolution [Phase 4]
│   ├── ledger/ledger.go       # JSONL append-only event log
│   ├── protocol/rpc.go        # JSON-RPC 2.0 types and methods
│   ├── territory/territory.go # .cosa/ workspace management
│   ├── tui/
│   │   ├── app.go             # Root Bubble Tea model
│   │   ├── component/         # WorkerList, JobList, Activity
│   │   ├── page/dashboard.go  # Main dashboard view
│   │   ├── styles/styles.go   # Lipgloss styles
│   │   └── theme/theme.go     # Color themes (noir, godfather, miami)
│   └── worker/
│       ├── worker.go          # Worker struct, state machine
│       ├── roles.go           # Role permissions system [Phase 4]
│       └── pool.go            # Worker pool with availability tracking [Phase 4]
├── bin/                        # Built binaries
├── go.mod
├── Makefile
└── HANDOFF.md                  # This file
```

### IPC Protocol

JSON-RPC 2.0 over Unix socket (`~/.cosa/cosa.sock`)

**Current Methods** (see `internal/protocol/rpc.go`):
- `status` - Get daemon status
- `shutdown` - Graceful shutdown
- `territory.init` - Initialize territory
- `territory.status` - Get territory info
- `worker.add` - Add worker with worktree
- `worker.list` - List workers
- `worker.status` - Get detailed worker info [Phase 4]
- `worker.remove` - Remove worker
- `job.add` - Create job (auto-queued for scheduler)
- `job.list` - List jobs
- `job.cancel` - Cancel job
- `job.assign` - Manually assign job to worker [Phase 4]
- `queue.status` - Get queue depth info [Phase 4]
- `subscribe` / `unsubscribe` - Real-time event subscription

### Key Patterns

**Worker State Machine** (`internal/worker/worker.go`):
```
idle -> working -> idle
     -> reviewing -> idle
     -> error
     -> stopped
```

**Job Lifecycle** (`internal/job/job.go`):
```
pending -> queued -> running -> completed
                            -> failed
                            -> review -> completed
        -> cancelled
```

**Claude Integration** (`internal/claude/client.go`):
- Spawns: `claude --print --dangerously-skip-permissions --output-format stream-json -p "<prompt>"`
- Parses NDJSON stream for events (init, text, tool_use, tool_result, result, error)
- Tracks session ID for `--resume` continuity

### Role Hierarchy

Defined in `internal/worker/worker.go`:
- `don` - Project owner (human)
- `consigliere` - Code reviewer
- `capo` - Team lead, delegates work
- `soldato` - Regular worker (default)
- `lookout` - Monitors for stuck workers
- `cleaner` - Cleans up resources

---

## What Needs to Be Implemented

### Phase 4: Multi-Worker Orchestration ✅ COMPLETE

**Goal:** Role hierarchy with parallel workers

**Files created:**

1. `internal/worker/roles.go` - Role permission system
   - `RolePermissions` struct with CanDelegate, CanReview, CanAssignJobs, CanSupervise
   - Permission map for all roles (Don, Consigliere, Capo, Soldato, Lookout, Cleaner)
   - Helper functions: GetPermissions(), CanDelegate(), CanReview(), CanAssignJobs(), CanSupervise()

2. `internal/worker/pool.go` - Worker pool with availability tracking
   - Pool indexed by name and role
   - Methods: Add(), Remove(), Get(), GetByID(), List(), ListByRole()
   - Availability: GetAvailable(), GetAvailableByRole(), FindBestWorker()
   - Load-balanced selection preferring Soldatos over Capos

3. `internal/job/queue.go` - Priority queue with dependency resolution
   - Uses container/heap for min-heap ordering
   - Priority ordering: higher priority first, then FIFO by creation time
   - Dependency tracking with pending map
   - Methods: Enqueue(), Dequeue(), GetReady(), GetPending(), NotifyCompletion(), NotifyFailure()

4. `internal/claude/session.go` - Session persistence
   - SessionInfo struct with SessionID, WorkerID, WorkerName, timestamps
   - SessionStore persisting to ~/.cosa/sessions/
   - Methods: Save(), Load(), LoadByWorker(), LoadByWorkerName(), Delete(), Cleanup()

**Files modified:**

- `internal/protocol/rpc.go` - Added MethodJobAssign, MethodQueueStatus, WorkerDetailInfo, JobAssignParams, QueueStatusResult
- `internal/worker/worker.go` - Added OnJobComplete/OnJobFail callbacks
- `internal/daemon/server.go` - Replaced workers map with Pool, added Queue, SessionStore, scheduler goroutine (100ms tick)
- `internal/daemon/handlers.go` - Updated all handlers to use pool, added handleWorkerStatus, handleJobAssign, handleQueueStatus

**Verification needed (manual testing):**
- [ ] Multiple workers run concurrently
- [ ] Jobs distributed across idle workers
- [ ] High-priority jobs execute first
- [ ] Dependent jobs wait for dependencies
- [ ] Session IDs persist across daemon restart
- [ ] Capo workers can supervise soldatos

### Phase 5: Code Review Flow

**Goal:** Automated review with Consigliere

**Files to create:**

1. `internal/review/gate.go` - Pre-review checks
   - Run tests before review (territory.Config.TestCommand)
   - Run build (territory.Config.BuildCommand)
   - Gate review on passing checks

2. `internal/review/consigliere.go` - Review worker
   - Special worker that reviews completed jobs
   - Uses Claude to analyze changes
   - Provides structured feedback

3. `internal/review/decision.go` - Review outcomes
   - Approved -> auto-merge
   - Changes requested -> return to worker
   - Rejected -> mark job failed

4. `internal/git/merge.go` - Merge operations
   - Merge worker branch to base
   - Handle conflicts
   - Clean up worktree after merge

**Protocol additions:**
- `review.start` - Trigger review for job
- `review.approve` / `review.reject` - Review decisions

**Milestone verification:**
- Tests run before review
- Consigliere reviews completed work
- Approved changes auto-merge
- Rejections return to worker

### Phase 6: Operations & Advanced

**Goal:** Batch operations, handoffs, monitoring

**Files to create:**

1. `internal/job/operation.go` - Operation batching
   - Group related jobs into operations
   - Track operation-level progress
   - Rollback on failure

2. `internal/tui/page/operation.go` - Gantt-like view
   - Show operation timeline
   - Job dependencies visualization
   - Progress tracking

3. `internal/worker/handoff.go` - Context transfer
   - Summarize context when switching workers
   - Pass context to new worker session
   - Preserve important state

4. `internal/worker/monitor.go` - Stuck detection
   - Lookout role implementation
   - Detect workers stuck too long
   - Alert or restart stuck workers

5. `internal/worker/cleaner.go` - Resource cleanup
   - Cleaner role implementation
   - Remove stale worktrees
   - Clean old sessions

**Protocol additions:**
- `operation.create` / `operation.status` / `operation.list`
- `worker.handoff` - Trigger handoff

### Phase 7: Polish

**Goal:** Production quality

**Deliverables:**

1. Additional themes
   - OpenCode theme (clean, modern)
   - Custom theme support via config

2. Cost tracking
   - Parse cost from Claude output
   - Aggregate by worker/job/operation
   - Display in TUI

3. System notifications
   - Desktop notifications for completed jobs
   - Error alerts

4. Comprehensive error handling
   - Graceful degradation
   - Retry logic
   - Better error messages

5. Testing
   - Unit tests for all packages
   - Integration tests for daemon
   - TUI component tests

---

## Important Implementation Notes

### Claude Code Integration

The current `internal/claude/client.go` spawns Claude but doesn't fully handle all stream-json message types. The parser in `parser.go` handles basic types but may need updates based on actual Claude Code output format.

**Key flags used:**
```bash
claude --print --dangerously-skip-permissions --output-format stream-json -p "prompt"
```

For resume: add `--resume <session_id>`

### Git Worktree Management

Each worker gets a worktree at `.cosa/worktrees/<worker-name>` with branch `cosa/<worker-name>`. The territory stores the base branch for new worktrees.

### Event System

The ledger (`internal/ledger/ledger.go`) is append-only JSONL at `~/.cosa/events.jsonl`. It supports subscriptions for real-time updates to the TUI.

### TUI Architecture

The TUI uses Bubble Tea with:
- `App` as root model
- `Dashboard` as main page
- Components for worker list, job list, activity feed
- Styles based on current theme

Navigation: Tab/Shift-Tab between panels, j/k for list navigation, vim-style h/l for panel switching.

---

## Build & Test

```bash
cd cosa

# Build
make build

# Run daemon in foreground (for development)
make run-daemon

# In another terminal, test commands
./bin/cosa status
./bin/cosa territory init /path/to/git/repo
./bin/cosa worker add paulie
./bin/cosa job add "Fix the bug" -w paulie
./bin/cosa worker list
./bin/cosa job list

# Launch TUI
./bin/cosa tui

# Stop daemon
make stop
```

---

## Files That Need the Most Work

1. **`internal/worker/worker.go`** - Execute() method needs real Claude integration testing
2. **`internal/claude/parser.go`** - May need updates for actual Claude output format
3. **`internal/tui/app.go`** - Add queue status display, keyboard shortcuts for new features
4. **`cmd/cosa/`** - Add CLI commands for queue.status, worker.status, job.assign
5. **`internal/review/`** - New package for Phase 5 code review flow
6. **Unit tests** - Add tests for queue, pool, roles, session packages

---

## Dependencies

Current `go.mod` includes:
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/spf13/cobra` - CLI framework
- `github.com/google/uuid` - UUIDs
- `gopkg.in/yaml.v3` - Config parsing

May need to add for future phases:
- `github.com/charmbracelet/bubbles` - Additional TUI components (viewport, textinput, etc.)
