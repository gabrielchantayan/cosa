# Cosa Nostra: Spec vs Implementation Analysis

This document provides a comprehensive comparison between the specification (SPEC.md) and the current implementation state of the Cosa Nostra codebase.

---

## Executive Summary

The implementation has achieved **significant progress** beyond the original phased plan, with Phases 1-6 substantially complete and much of Phase 7 done. The core architecture closely follows the spec, with some notable enhancements (MCP server, interactive chat) and a few gaps (primarily in autonomous orchestration roles).

**Overall Completion: ~85%**

---

## Feature Comparison Matrix

| Feature | Spec Status | Implementation Status | Notes |
|---------|-------------|----------------------|-------|
| **Daemon Infrastructure** | Required | ✅ Complete | Unix socket, JSON-RPC 2.0 |
| **CLI Client** | Required | ✅ Complete | 40+ commands via Cobra |
| **TUI Dashboard** | Required | ✅ Complete | Bubble Tea with themes |
| **Worker System** | Required | ✅ Complete | Pool, roles, lifecycle |
| **Job Management** | Required | ✅ Complete | Priority queue, dependencies |
| **Claude Integration** | Required | ✅ Complete | Stream-json parsing, sessions |
| **Git Worktrees** | Required | ✅ Complete | Per-worker isolation |
| **Territory Management** | Required | ✅ Complete | Project workspaces |
| **Event Ledger** | Required | ✅ Complete | JSONL append-only log |
| **Code Review** | Required | ✅ Complete | Consigliere, gates, merge |
| **Operations** | Required | ✅ Complete | Batch jobs, progress |
| **Cost Tracking** | Required | ✅ Complete | Per-job/worker tracking |
| **Notifications** | Required | ✅ Complete | System notifications |
| **Lookout (Monitoring)** | Required | ✅ Complete | Stuck worker detection |
| **Cleaner** | Required | ✅ Complete | Resource cleanup |
| **Handoff System** | Required | ✅ Complete | Session summaries |
| **Standing Orders** | Required | ✅ Complete | Persistent instructions |
| **MCP Server** | Not in Spec | ✅ Added | Claude tool integration |
| **Underboss Chat** | Not in Spec | ✅ Added | Interactive AI conversation |
| **Dev Branch Workflow** | Not in Spec | ✅ Added | Staging before main |
| **Underboss (Auto-Orchestrator)** | Required | ❌ Partial | Not fully autonomous |
| **Capo (Auto-Delegation)** | Required | ❌ Partial | Manual delegation only |
| **Auto-Decomposition** | Required | ❌ Not Implemented | Manual job splitting |

---

## Detailed Comparison

### 1. Role Hierarchy

#### Spec Definition
| Role | Responsibilities | Model |
|------|------------------|-------|
| Don | Strategic decisions, final authority | Human |
| Underboss | Central orchestrator, job distribution, coordination | Opus |
| Capo | Territory management, job decomposition, conflict resolution | Opus |
| Soldato | Named long-lived workers, design, planning, implementation | Sonnet |
| Associate | Ephemeral workers, execute well-defined tasks | Sonnet |
| Consigliere | Code review, quality enforcement | Opus |
| Cleaner | Cleanup, garbage collection, patrols | Sonnet |
| Lookout | Monitoring, stuck worker detection, health checks | Sonnet |

#### Implementation State
- ✅ **Don**: Implicit (human user)
- ⚠️ **Underboss**: Defined but not autonomous; no auto-job-distribution; chat interface implemented
- ⚠️ **Capo**: Defined but no auto-decomposition; manual delegation only
- ✅ **Soldato**: Fully implemented with worktrees, job execution
- ⚠️ **Associate**: Role defined but ephemeral spawning not implemented
- ✅ **Consigliere**: Fully implemented with AI code review
- ✅ **Cleaner**: Implemented with resource cleanup
- ✅ **Lookout**: Implemented with stuck worker detection

**Gap Analysis**: The autonomous intelligence of Underboss and Capo roles is the main missing piece. Currently, job distribution and decomposition require human intervention rather than AI-driven decisions.

---

### 2. File Structure

#### Spec Definition
```
~/.config/cosa/
├── config.yaml           # Global configuration
├── themes/               # Color theme definitions
└── templates/            # Standing order templates

~/.cosa/
├── cosa.sock             # Unix socket for daemon
├── daemon.pid            # Daemon process ID
├── events.jsonl          # Append-only event log
├── sessions/             # Worker session data
└── logs/                 # Log files

<territory>/.cosa/
├── territory.yaml        # Territory-specific config
├── prompts/              # Custom system prompts
└── worktrees/            # Worker git worktrees
```

#### Implementation State
```
~/.config/cosa/
└── config.yaml           # ✅ Implemented

~/.cosa/
├── cosa.sock             # ✅ Implemented
├── daemon.pid            # ✅ Implemented
├── events.jsonl          # ✅ Implemented
├── sessions/             # ✅ Implemented
└── logs/                 # ⚠️ Partial (daemon.log only)

<territory>/.cosa/
├── territory.yaml        # ⚠️ Not separate (in memory)
├── prompts/              # ❌ Not implemented
└── worktrees/            # ✅ Implemented
```

**Gap Analysis**:
- Custom themes directory not implemented (hardcoded in code)
- Standing order templates directory not implemented
- Per-territory config file not persisted to YAML
- Custom prompts directory not implemented
- Detailed worker logs not implemented

---

### 3. Configuration System

#### Spec Definition
```yaml
version: 1
theme: noir
defaults:
  workers_per_territory: 6
  model:
    underboss: opus
    capo: opus
    soldato: sonnet
    associate: sonnet
    consigliere: opus
    cleaner: sonnet
    lookout: sonnet
daemon:
  socket: ~/.cosa/cosa.sock
  log_level: info
notifications:
  tui_alerts: true
  system_notifications: true
  terminal_bell: true
keyboard:
  style: vim
```

#### Implementation State
- ✅ Theme selection (noir, godfather, miami, opencode)
- ✅ Per-role model configuration
- ✅ Socket path configuration
- ⚠️ Log level: Not configurable (hardcoded)
- ✅ Notification settings
- ⚠️ Keyboard style: Hardcoded vim-style
- ❌ Version field: Not implemented
- ❌ workers_per_territory: Not configurable

**Gap Analysis**: Core configuration works; some refinement options missing.

---

### 4. Data Model

#### Job Structure

**Spec:**
```go
type Job struct {
    ID           string
    Title        string
    Description  string
    Status       JobStatus
    Priority     int
    Territory    string
    AssignedTo   string
    DependsOn    []string
    CreatedAt    time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
    ParentJob    string   // For decomposed jobs
    Operation    string   // Operation membership
}
```

**Implementation:**
```go
type Job struct {
    ID             string      // ✅
    Description    string      // ✅ (Title merged into Description)
    Status         Status      // ✅
    Priority       int         // ✅
    Worker         string      // ✅ (renamed from AssignedTo)
    DependsOn      []string    // ✅
    CreatedAt      time.Time   // ✅
    StartedAt      time.Time   // ✅
    CompletedAt    time.Time   // ✅
    SessionID      string      // ✅ Added
    ReviewFeedback []string    // ✅ Added
    RevisionOf     string      // ✅ Added
    TotalCost      string      // ✅ Added
    TotalTokens    int         // ✅ Added
    // ❌ Missing: Territory, ParentJob, Operation
}
```

**Gap Analysis**:
- No separate Title field (combined with Description)
- Territory field missing (stored elsewhere)
- ParentJob not implemented (no job decomposition)
- Operation field exists in operations but not in Job struct

---

### 5. Event Types

#### Spec Definition
```go
const (
    EventJobCreated     EventType = "job.created"
    EventJobAssigned    EventType = "job.assigned"
    EventJobStarted     EventType = "job.started"
    EventJobCompleted   EventType = "job.completed"
    EventJobFailed      EventType = "job.failed"
    EventJobBlocked     EventType = "job.blocked"
    EventWorkerStarted  EventType = "worker.started"
    EventWorkerStopped  EventType = "worker.stopped"
    EventWorkerHandoff  EventType = "worker.handoff"
    EventReviewStarted  EventType = "review.started"
    EventReviewApproved EventType = "review.approved"
    EventReviewRejected EventType = "review.rejected"
    EventMerged         EventType = "merged"
    EventEscalation     EventType = "escalation"
)
```

#### Implementation State
The implementation has **40+ event types**, significantly exceeding the spec:

- ✅ All spec events implemented
- ✅ Additional events: daemon.started, daemon.stopped, territory.init, etc.
- ✅ Review events expanded: review.changes_requested, review.coordinator.*
- ✅ Chat events: chat.started, chat.message, chat.ended
- ✅ Operation events: operation.created, operation.progress, etc.

---

### 6. CLI Commands

#### Spec Definition
```bash
cosa start/stop/status
cosa territory init/list/add
cosa family list/add/remove
cosa job add/list/show/assign
cosa operation create/status/cancel
cosa worker <name> message/handoff/status
cosa order set/list/clear
cosa logs
```

#### Implementation State

| Command Group | Spec | Implementation | Notes |
|---------------|------|----------------|-------|
| Daemon | start, stop, status | ✅ All | Plus restart |
| Territory | init, list, add | ✅ All + | Added: status, dev-branch |
| Family | list, add, remove | ✅ Renamed | Now under `worker` |
| Job | add, list, show, assign | ✅ All + | Added: cancel |
| Operation | create, status, cancel | ✅ All + | Added: list |
| Worker | message, handoff, status | ✅ All + | Added: add, list, remove, detail |
| Order | set, list, clear | ✅ All | |
| Logs | logs | ✅ | With -f follow |
| **New** | - | settings | list, get, set, path |
| **New** | - | review | start, status, list |
| **New** | - | chat | Interactive underboss |
| **New** | - | mcp-serve | MCP server |
| **New** | - | tui | Launch TUI |

**Gap Analysis**: All spec commands implemented; many additions for richer functionality.

---

### 7. TUI Layout

#### Spec Dashboard
```
┌─────────────────────────────────────────────────────────────────┐
│ COSA NOSTRA                                     ◉ 12 workers    │
├─────────────────────────────────────────────────────────────────┤
│ WORKERS                          │ JOBS                         │
│ ┌─────────────────────────────┐  │ ┌─────────────────────────┐  │
│ │ ● tony (capo)      Working  │  │ │ #142 Add auth flow  [3] │  │
│ │ ● paulie           Idle     │  │ │ #141 Fix login bug  [2] │  │
│ └─────────────────────────────┘  │ └─────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│ ACTIVITY                                                        │
└─────────────────────────────────────────────────────────────────┘
│ [j/k] Navigate  [Enter] Select  [n] New Job  [o] Operation  [?] │
└─────────────────────────────────────────────────────────────────┘
```

#### Implementation State
- ✅ Dashboard with Workers, Jobs, Activity panels
- ✅ Panel titles with styling
- ✅ Keyboard navigation (j/k, Tab, Enter)
- ✅ Worker detail view
- ✅ Operation progress view
- ✅ Chat page (new feature)
- ✅ Command palette (:)
- ✅ Modal dialogs
- ⚠️ Help overlay (?): Not fully implemented
- ⚠️ New job shortcut (n): Via command palette instead

---

### 8. Keyboard Navigation

#### Spec Definition
**Global:** ?, q, Esc, j/k, h/l, g/G, /, :
**Dashboard:** Tab, n, o, Enter, r
**Worker View:** i, e, Enter, Shift+Enter, H

#### Implementation State
- ✅ j/k: Navigate lists
- ✅ Tab/Shift-Tab: Cycle panels
- ✅ Enter: Select/drill down
- ✅ q/Esc: Back/quit
- ✅ :: Command palette
- ⚠️ /: Search not fully implemented
- ⚠️ h/l: Partial (some views)
- ⚠️ g/G: Not implemented
- ⚠️ ?: Help not implemented
- ⚠️ n/o: Via command palette

---

### 9. Color Themes

#### Spec Definition
Four themes: Noir, Godfather, Miami, OpenCode

#### Implementation State
- ✅ Noir (default)
- ✅ Godfather
- ✅ Miami
- ✅ OpenCode
- ❌ Custom themes via config file

All four themes implemented in `internal/tui/theme/theme.go`.

---

### 10. Workflows

#### Job Lifecycle

**Spec:**
```
Created → Assigned → In Progress → Review → Approved → Merged
                                         → Rejected → (back to worker)
```

**Implementation:**
```
pending → queued → running → review → completed (with merge)
                          → failed
                          → changes_requested → (revision cycle)
```

✅ Substantially matches spec with additions for revision cycles.

#### Code Review Flow

**Spec:**
1. Worker completes implementation
2. Tests pass (pre-review gate)
3. Consigliere reviews
4. Approve → Auto-merge / Reject → Return to worker

**Implementation:**
- ✅ Pre-review gates (tests, build, lint)
- ✅ Consigliere AI review
- ✅ Approve/Reject/Request Changes decisions
- ✅ Auto-merge workflow
- ✅ Review feedback storage
- ✅ Revision tracking

---

## Architectural Differences

### 1. Communication Architecture

**Spec Vision:**
```
Don (TUI/CLI) → Unix Socket → Daemon
                                  ↓
                             Underboss
                                  ↓
                    ┌─────────────┼─────────────┐
                 Capo A        Capo B        Capo C
                    ↓             ↓             ↓
              Soldatos       Soldatos       Soldatos
```

**Implementation Reality:**
```
Don (TUI/CLI) → Unix Socket → Daemon (direct orchestration)
                                  ↓
                         Worker Pool (flat)
                              ↓
                    ┌─────────┼─────────┐
                Worker A  Worker B  Worker C
```

**Key Difference:** The spec envisions a hierarchical AI orchestration where Underboss and Capos make autonomous decisions. The implementation has a flat pool where the daemon directly manages all workers, with humans making strategic decisions.

### 2. Autonomous vs Manual Orchestration

**Spec:**
- Underboss autonomously distributes jobs across territories
- Capos autonomously decompose large jobs
- Associates spawned on-demand automatically

**Implementation:**
- Jobs assigned manually or via simple scheduler
- No automatic job decomposition
- Associates must be created manually
- Underboss chat provides advice but doesn't auto-execute

### 3. Multi-Territory Support

**Spec:**
- Multiple territories with separate Capos
- Cross-territory coordination by Underboss
- Territory-specific configuration and prompts

**Implementation:**
- Territory system exists but single-territory focus
- No cross-territory job routing
- No per-territory Capo assignment

### 4. MCP Integration (Implementation Addition)

**Not in Spec:** The implementation adds an MCP (Model Context Protocol) server that allows Claude to directly interact with Cosa:

```
Claude Session → MCP Protocol → Cosa MCP Server → Daemon
```

This enables:
- Claude querying worker status
- Claude creating jobs
- Claude reading activity logs
- Real-time system information during conversations

### 5. Underboss Chat (Implementation Addition)

**Not in Spec:** Interactive conversation interface with the Underboss:

```
User → Chat UI → Claude (Underboss persona) → MCP Tools → System State
```

This provides a conversational way to:
- Discuss work strategy
- Get recommendations
- Create jobs via natural language
- Monitor progress

---

## Missing Features Summary

### Critical (Core Spec Features)

1. **Autonomous Underboss**
   - Auto-job distribution based on worker availability and skills
   - Cross-territory coordination
   - Escalation handling

2. **Auto-Decomposition**
   - Large job analysis and splitting
   - Subjob dependency generation
   - Parent job tracking

3. **Associate Spawning**
   - On-demand ephemeral worker creation
   - Auto-cleanup after task completion

### Non-Critical (Nice to Have)

1. **Custom Theme Files**
   - Load themes from ~/.config/cosa/themes/

2. **Standing Order Templates**
   - Predefined instruction sets

3. **Per-Territory Config Files**
   - Persistent territory.yaml

4. **Custom Prompts Directory**
   - Role-specific prompt customization

5. **Full Keyboard Shortcuts**
   - Search (/), Help (?), g/G navigation

---

## Implementation Additions Beyond Spec

1. **MCP Server** - Claude tool integration
2. **Underboss Chat** - Interactive AI conversation
3. **Dev Branch Workflow** - Staging before main merge
4. **Settings CLI** - Configuration management
5. **Review Coordinator** - Multi-stage review pipeline
6. **Extended Event Types** - 40+ event types
7. **Word Wrap Dialogs** - Improved TUI UX
8. **Command Palette** - Vim-like : command mode

---

## Recommendations for Completion

### High Priority

1. **Implement Autonomous Underboss Logic**
   - Add job distribution algorithm
   - Implement workload balancing
   - Add escalation handling

2. **Add Job Decomposition**
   - Integrate Claude for job analysis
   - Generate subjobs with dependencies
   - Track parent-child relationships

3. **Enable Associate Spawning**
   - Ephemeral worker creation
   - Auto-cleanup mechanism

### Medium Priority

4. **Complete Keyboard Navigation**
   - Add missing shortcuts (/, ?, g/G)
   - Document all bindings

5. **Add Custom Theme Support**
   - Load themes from files
   - Runtime theme switching

6. **Per-Territory Configuration**
   - Persist territory.yaml
   - Support custom prompts

### Low Priority

7. **Multi-Territory Orchestration**
   - Cross-territory job routing
   - Territory-specific Capo assignment

8. **Enhanced Logging**
   - Per-worker log files
   - Log level configuration

---

## Conclusion

The Cosa Nostra implementation is a **substantial and functional system** that achieves the core goals of the specification. The daemon architecture, worker management, job system, TUI, and code review flow are all working.

The main gap is the **autonomous AI orchestration layer** - the Underboss and Capo roles that should make intelligent decisions about work distribution and decomposition. Currently, these decisions fall to the human user.

The implementation also adds valuable features not in the spec (MCP server, chat interface) that enhance the system's capabilities.

**Estimated completion for full spec compliance: 85%**

---

*Generated by tony (soldato) - January 2026*
