# Cosa Nostra - Potential Improvements

This document catalogs potential features, improvements, and fixes identified in the codebase.

---

## 1. Critical Security Fixes

### Path Traversal Vulnerability
**Location:** `internal/worker/pool.go:301`

Worker names can contain `../` sequences, potentially allowing directory traversal attacks when creating worker directories or files.

**Recommendation:** Validate and sanitize worker names to prevent path traversal.

### Git Command Injection Risk
**Location:** `internal/git/merge.go:78`

Branch names are passed to shell commands without proper escaping. Malicious branch names containing shell metacharacters could lead to command injection.

**Recommendation:** Use `--` argument separator and validate branch names against a whitelist pattern.

### File Permissions Too Permissive
**Location:** Various files

Sensitive data (session tokens, configuration) written with world-readable `0644` permissions.

**Recommendation:** Use `0600` for sensitive files to restrict access to owner only.

### Unqualified Binary Path
**Location:** `internal/claude/client.go`

Claude CLI invoked without absolute path, vulnerable to PATH injection attacks.

**Recommendation:** Use absolute path or validate binary location before execution.

---

## 2. Critical Bug Fixes

### Silent JSON Parsing Failures
**Location:** `internal/daemon/handlers.go`

Approximately 15 handlers ignore `json.Unmarshal` errors, leading to silent failures when clients send malformed JSON.

**Recommendation:** Check and return errors from all JSON parsing operations.

### Cost Aggregation Bug
**Location:** `internal/daemon/server.go:407-409`

The cost display only shows the last worker's cost instead of aggregating across all workers.

**Recommendation:** Sum costs across all workers for accurate total cost display.

### Silent Event Drops
**Location:** `internal/ledger/ledger.go:172-176`

Events are silently dropped if a subscriber's channel is full, with no logging or backpressure mechanism.

**Recommendation:** Log dropped events and consider implementing backpressure or buffering.

### Malformed Event Skipping
**Location:** `internal/ledger/ledger.go:195-197`

Corrupted ledger entries are silently skipped with no logging, making debugging difficult.

**Recommendation:** Log warnings for malformed entries with file position for debugging.

---

## 3. Performance Improvements

### Inefficient Job Queue
**Location:** `internal/job/queue.go:67-74`

Copies all jobs on every scheduler tick (100ms), causing unnecessary memory allocations and GC pressure.

**Recommendation:** Use indexed data structures or only process changed jobs.

### Full Ledger Reads
**Location:** `internal/ledger/ledger.go:205-218`

Reads entire events file for timestamp filtering with no indexing support.

**Recommendation:** Implement time-based indexing or maintain in-memory index of event positions.

### Sequential Git Diff Calls
**Location:** `internal/git/merge.go:28-56`

Makes 3 separate git diff calls that could be combined into a single operation.

**Recommendation:** Combine into single git command or parallelize calls.

### Worker Pool Linear Searches
**Location:** `internal/worker/pool.go`

Iterates over all workers on every scheduler tick to find available workers.

**Recommendation:** Maintain separate ready queue of available workers.

### Unbounded Session Store
**Location:** `internal/claude/session.go`

No cleanup or TTL mechanism for old sessions, leading to memory growth over time.

**Recommendation:** Implement session TTL and periodic cleanup of expired sessions.

### No Ledger Rotation
**Location:** `internal/ledger/`

Single `events.jsonl` file grows unbounded with no rotation strategy.

**Recommendation:** Implement log rotation based on size or time.

---

## 4. TUI Incomplete Features (High Priority)

### Operation Dialog
**Location:** `internal/tui/page/dashboard.go:508-510`

Pressing 'o' only logs a debug message - the operation dialog is not implemented.

**Recommendation:** Implement operation management dialog with start/stop/view capabilities.

### Search Interface
**Location:** `internal/tui/page/dashboard.go:514-516`

Pressing '/' only logs a debug message - search functionality is stubbed.

**Recommendation:** Implement searchable interface for jobs, workers, and logs.

### Command Palette
**Location:** `internal/tui/page/dashboard.go:520-522`

Command palette component exists but is never shown to users.

**Recommendation:** Wire up command palette trigger and populate with available commands.

### Help Overlay
**Location:** `internal/tui/page/dashboard.go:527-529`

Pressing '?' only logs - no help panel is displayed.

**Recommendation:** Implement help overlay showing available keybindings and commands.

### Chat Request Cancellation
**Location:** `internal/tui/app.go:316`

TODO comment indicates cancellation doesn't actually stop the Claude API call.

**Recommendation:** Implement proper context cancellation for Claude API requests.

---

## 5. TUI Improvements (Medium Priority)

### Responsive Layout
**Location:** `internal/tui/page/dashboard.go:161-162`

Hardcoded 30/70 split between panels with no user-adjustable sizing.

**Recommendation:** Implement resizable panes or configurable split ratios.

### Panel Title Truncation
**Location:** `internal/tui/styles/styles.go:222-257`

Panel titles silently dropped if too wide for container.

**Recommendation:** Use ellipsis truncation to indicate hidden content.

### Dialog Button Focus
**Location:** `internal/tui/component/dialog.go:330-355`

No visual focus indicator on currently selected dialog button.

**Recommendation:** Add distinct styling for focused button state.

### Dialog State Synchronization
**Location:** `internal/tui/page/dashboard.go:425-426`

`showDialog` boolean and `dialog.Visible()` could become desynchronized.

**Recommendation:** Use single source of truth for dialog visibility state.

### Error State Indicators
**Location:** Various TUI components

Failed jobs and workers only indicated in activity log, not inline.

**Recommendation:** Add visual badges/indicators on list items showing error states.

---

## 6. Code Quality Improvements

### Text Wrapping Duplication
**Location:** `internal/tui/component/input.go`, `internal/tui/component/textarea.go`

Similar text wrapping logic duplicated across components.

**Recommendation:** Extract to shared utility in `internal/tui/util/` package.

### List Component Duplication
**Location:** `internal/tui/component/workerlist.go`, `internal/tui/component/joblist.go`

WorkerList and JobList share significant structural patterns.

**Recommendation:** Use Go generics to create shared list component base.

### ANSI Manipulation Logging
**Location:** `internal/tui/` package

`ansi.Truncate()` and `ansi.Cut()` failures not logged, making styling issues hard to debug.

**Recommendation:** Add debug logging for ANSI manipulation edge cases.

---

## 7. Feature Gaps (from SPEC-VS-IMPLEMENTATION.md)

### Underboss Autonomous Orchestration
The Underboss role lacks automatic job distribution to workers based on capacity and specialization.

**Recommendation:** Implement intelligent job routing based on worker availability and task type.

### Capo Auto-Decomposition
Capo workers cannot automatically split large jobs into smaller sub-tasks.

**Recommendation:** Implement job decomposition logic with dependency tracking.

### Associate Ephemeral Spawning
Associate role exists but workers are not automatically spawned for one-off tasks.

**Recommendation:** Implement dynamic worker spawning based on job queue depth.

### Operations Persistence
Operations are stored in-memory only and lost on daemon restart.

**Recommendation:** Persist operation state to disk with recovery on startup.

---

## 8. New Feature Ideas

### Job Templates
Predefined job types (refactor, test, document, review) with standard prompts and configurations.

### Worker Affinity
Assign workers to specific types of tasks based on their demonstrated capabilities or explicit configuration.

### Job History Search
Search completed jobs by description, output content, or metadata.

### Cost Budgets
Set spending limits per worker, per operation, or globally with alerts and automatic pausing.

### Notification Integrations
Slack, Discord, or webhook notifications for job completion, failures, or budget alerts.

### Web Dashboard
Browser-based alternative to TUI for monitoring and management, especially useful for remote access.

### Multi-Project Support
Manage multiple territories (projects) simultaneously with context switching.

### Job Scheduling
Queue jobs for execution at specific times or on recurring schedules.

### Batch Job Improvements
Better operation tracking with progress visualization, dependency graphs, and aggregate statistics.

---

## Priority Matrix

| Priority | Category | Effort |
|----------|----------|--------|
| P0 | Critical Security Fixes | Medium |
| P0 | Critical Bug Fixes | Low-Medium |
| P1 | TUI Incomplete Features | Medium |
| P2 | Performance Improvements | Medium-High |
| P2 | TUI Improvements | Low-Medium |
| P3 | Code Quality | Low |
| P3 | Feature Gaps | High |
| P4 | New Features | Variable |
