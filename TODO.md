# Cosa Nostra - Implementation TODO

## Phase 4: Multi-Worker Orchestration

### Files to Create

- [ ] `internal/worker/roles.go`
  - [ ] Define RolePermissions struct
  - [ ] Implement CanDelegate(role) bool
  - [ ] Implement CanReview(role) bool
  - [ ] Implement CanAssignJobs(role) bool

- [ ] `internal/worker/pool.go`
  - [ ] Define Pool struct with worker map
  - [ ] Implement AddWorker/RemoveWorker
  - [ ] Implement GetAvailable() - return idle workers
  - [ ] Implement AssignJob(job) - find best worker
  - [ ] Implement worker health monitoring

- [ ] `internal/job/queue.go`
  - [ ] Define PriorityQueue using heap
  - [ ] Implement Push/Pop with priority ordering
  - [ ] Implement dependency resolution
  - [ ] Implement GetReady() - jobs with satisfied deps

- [ ] `internal/claude/session.go`
  - [ ] Define SessionStore struct
  - [ ] Implement Save/Load session to disk
  - [ ] Implement session cleanup (old sessions)
  - [ ] Implement context summarization for handoffs

### Files to Modify

- [ ] `internal/daemon/server.go`
  - [ ] Replace simple workers map with Pool
  - [ ] Add job scheduler goroutine
  - [ ] Start scheduler on daemon start

- [ ] `internal/daemon/handlers.go`
  - [ ] Update handleJobAdd to use queue
  - [ ] Add handleWorkerStatus for detailed info
  - [ ] Add handleJobAssign for manual assignment

- [ ] `internal/protocol/rpc.go`
  - [ ] Add MethodWorkerStatus
  - [ ] Add MethodJobAssign
  - [ ] Add WorkerDetailInfo struct

### Verification
- [ ] Start daemon, add 3 workers
- [ ] Create 5 jobs, verify distribution
- [ ] Test job dependencies work
- [ ] Test session resume after restart

---

## Phase 5: Code Review Flow

### Files to Create

- [ ] `internal/review/gate.go`
  - [ ] Define Gate struct
  - [ ] Implement RunTests(worktree) (Result, error)
  - [ ] Implement RunBuild(worktree) (Result, error)
  - [ ] Implement CheckAll() - run all gates

- [ ] `internal/review/consigliere.go`
  - [ ] Define Consigliere worker type
  - [ ] Implement Review(job) ReviewResult
  - [ ] Build review prompt for Claude
  - [ ] Parse Claude's review response

- [ ] `internal/review/decision.go`
  - [ ] Define ReviewResult struct (approved, changes, rejected)
  - [ ] Define ReviewFeedback struct
  - [ ] Implement HandleApproval - trigger merge
  - [ ] Implement HandleRejection - return to worker

- [ ] `internal/git/merge.go`
  - [ ] Implement MergeWorkerBranch(workerName, baseBranch)
  - [ ] Implement HandleConflicts
  - [ ] Implement CleanupAfterMerge

### Files to Modify

- [ ] `internal/daemon/handlers.go`
  - [ ] Add handleReviewStart
  - [ ] Add handleReviewApprove
  - [ ] Add handleReviewReject

- [ ] `internal/protocol/rpc.go`
  - [ ] Add MethodReviewStart, MethodReviewApprove, MethodReviewReject
  - [ ] Add ReviewResult struct

- [ ] `internal/worker/worker.go`
  - [ ] Add transition to StatusReview
  - [ ] Handle review feedback

- [ ] `internal/job/job.go`
  - [ ] Add StatusReview state handling
  - [ ] Store review feedback

### Verification
- [ ] Complete a job, verify tests run
- [ ] Consigliere reviews and approves
- [ ] Verify auto-merge happens
- [ ] Test rejection flow

---

## Phase 6: Operations & Advanced

### Files to Create

- [ ] `internal/job/operation.go`
  - [ ] Define Operation struct (id, name, jobs[])
  - [ ] Define OperationStore
  - [ ] Implement Create/Get/List operations
  - [ ] Implement GetProgress(operation)
  - [ ] Implement Rollback(operation)

- [ ] `internal/tui/page/operation.go`
  - [ ] Define OperationView page
  - [ ] Render Gantt-style timeline
  - [ ] Show job dependencies as arrows
  - [ ] Display progress bars

- [ ] `internal/worker/handoff.go`
  - [ ] Implement SummarizeContext(worker) string
  - [ ] Implement TransferContext(from, to)
  - [ ] Use Claude to generate summary
  - [ ] Preserve key state across transfer

- [ ] `internal/worker/monitor.go`
  - [ ] Define Lookout struct
  - [ ] Implement CheckStuck() []Worker
  - [ ] Define stuck threshold (configurable)
  - [ ] Implement Alert/Restart stuck workers

- [ ] `internal/worker/cleaner.go`
  - [ ] Define Cleaner struct
  - [ ] Implement CleanStaleWorktrees()
  - [ ] Implement CleanOldSessions()
  - [ ] Schedule periodic cleanup

### Files to Modify

- [ ] `internal/daemon/handlers.go`
  - [ ] Add handleOperationCreate
  - [ ] Add handleOperationStatus
  - [ ] Add handleOperationList
  - [ ] Add handleWorkerHandoff

- [ ] `internal/protocol/rpc.go`
  - [ ] Add Operation-related methods
  - [ ] Add MethodWorkerHandoff

- [ ] `internal/tui/app.go`
  - [ ] Add 'o' key for operations view
  - [ ] Handle page switching

- [ ] `internal/daemon/server.go`
  - [ ] Start Lookout goroutine
  - [ ] Start Cleaner goroutine

### Verification
- [ ] Create operation with 3 jobs
- [ ] View operation in TUI
- [ ] Test worker handoff preserves context
- [ ] Verify stuck detection works

---

## Phase 7: Polish

### Additional Themes
- [ ] `internal/tui/theme/opencode.go`
  - [ ] Define clean, modern color palette
  - [ ] Light mode variant

- [ ] Config-based custom themes
  - [ ] Add theme config to config.yaml
  - [ ] Load custom colors from config

### Cost Tracking
- [ ] `internal/claude/cost.go`
  - [ ] Parse cost from Claude result
  - [ ] Define CostTracker struct
  - [ ] Aggregate by worker/job/operation

- [ ] Update TUI to display costs
  - [ ] Add cost column to job list
  - [ ] Show total in header

### System Notifications
- [ ] `internal/notify/notify.go`
  - [ ] Implement desktop notifications (platform-specific)
  - [ ] Notification settings in config
  - [ ] Notify on job complete/fail

### Error Handling
- [ ] Review all error paths
- [ ] Add retry logic for Claude calls
- [ ] Graceful degradation when Claude unavailable
- [ ] Better error messages in TUI

### Testing
- [ ] `internal/protocol/rpc_test.go`
- [ ] `internal/job/job_test.go`
- [ ] `internal/worker/worker_test.go`
- [ ] `internal/daemon/server_test.go`
- [ ] Integration test suite

### Documentation
- [ ] README.md with full usage guide
- [ ] Architecture diagram
- [ ] API documentation
- [ ] Contributing guide

---

## Quick Start for Next Agent

```bash
cd /Users/gabe/Documents/Programming/mob-try-2/cosa

# Build and verify current state
make build
./bin/cosa --help

# Start daemon for testing
./bin/cosa start -f

# In another terminal
./bin/cosa status
./bin/cosa territory init .
./bin/cosa worker add paulie
./bin/cosa worker list
./bin/cosa job add "Test job" -w paulie
./bin/cosa job list

# Launch TUI
./bin/cosa tui
```

## Key Files to Read First

1. `internal/protocol/rpc.go` - Understand the IPC protocol
2. `internal/daemon/server.go` - Main daemon loop
3. `internal/daemon/handlers.go` - How requests are handled
4. `internal/worker/worker.go` - Worker state machine
5. `internal/job/job.go` - Job lifecycle
6. `HANDOFF.md` - Full architecture documentation
