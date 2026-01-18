# Cosa Nostra: Agentic Development System

**A mafia-themed, TUI-based multi-agent development orchestration tool**

Inspired by Steve Yegge's Gas Town, Cosa Nostra ("Our Thing") provides a fully immersive mafia-themed interface for orchestrating multiple Claude Code agents working in parallel on your codebase.

---

## Overview

### Vision
Cosa Nostra is a personal productivity tool that manages a "Family" of AI agents working on your code. You are the Don - the workers handle the execution while you make the strategic decisions.

### Goals
- Orchestrate multiple Claude Code CLI sessions working in parallel
- Provide production-ready code through mandatory code review gates
- Offer a rich TUI experience modeled after OpenCode's Bubble Tea implementation
- Support multiple projects ("Territories") simultaneously
- Enable fully autonomous operation with human-in-the-loop escalation

### Non-Goals
- Commercial product features (user management, billing, etc.)
- External issue tracker integration (standalone system)
- Cloud/remote operation (local-only, macOS focus)

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Don (You)                                │
│                    TUI / CLI Client                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                         Unix Socket
                    (~/.cosa/cosa.sock)
                              │
┌─────────────────────────────────────────────────────────────────┐
│                      Cosa Daemon                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                     Underboss                                ││
│  │            (Central Orchestrator Agent)                      ││
│  └─────────────────────────────────────────────────────────────┘│
│         │                    │                    │              │
│  ┌──────┴───────┐    ┌──────┴───────┐    ┌──────┴───────┐      │
│  │  Territory A │    │  Territory B │    │  Territory N │      │
│  │  ┌─────────┐ │    │  ┌─────────┐ │    │  ┌─────────┐ │      │
│  │  │  Capo   │ │    │  │  Capo   │ │    │  │  Capo   │ │      │
│  │  └────┬────┘ │    │  └────┬────┘ │    │  └────┬────┘ │      │
│  │       │      │    │       │      │    │       │      │      │
│  │  ┌────┴────┐ │    │  ┌────┴────┐ │    │  ┌────┴────┐ │      │
│  │  │Soldatos │ │    │  │Soldatos │ │    │  │Soldatos │ │      │
│  │  │ + Assoc │ │    │  │ + Assoc │ │    │  │ + Assoc │ │      │
│  │  └─────────┘ │    │  └─────────┘ │    │  └─────────┘ │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│                                                                  │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐     │
│  │  Consigliere   │  │    Cleaner     │  │    Lookout     │     │
│  │ (Code Review)  │  │   (Cleanup)    │  │  (Monitoring)  │     │
│  └────────────────┘  └────────────────┘  └────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### Role Hierarchy

| Role | Gas Town Equivalent | Responsibilities | Model |
|------|---------------------|------------------|-------|
| **Don** | User | Strategic decisions, final authority | Human |
| **Underboss** | Mayor | Central orchestrator, job distribution, coordination | Opus |
| **Capo** | Crew Lead | Territory management, job decomposition, conflict resolution | Opus |
| **Soldato** | Crew Member | Named long-lived workers, design, planning, implementation | Sonnet |
| **Associate** | Polecat | Ephemeral workers, execute well-defined tasks | Sonnet |
| **Consigliere** | N/A (new) | Code review, quality enforcement | Opus |
| **Cleaner** | Deacon | Cleanup, garbage collection, patrols | Sonnet |
| **Lookout** | Witness | Monitoring, stuck worker detection, health checks | Sonnet |

### Core Concepts

#### Territory (Rig)
A project workspace. Each Territory has:
- A git repository root
- Its own Capo and crew of Soldatos
- Worker worktrees in `.cosa/worktrees/<worker-name>/`
- Territory-specific configuration and system prompts

#### The Ledger (Beads)
The universal work tracking system. All jobs flow through the Ledger as events in a JSONL append-only log.

#### Job (Bead)
A unit of work with:
- Unique ID
- Status (pending, in_progress, completed, blocked, failed)
- Assigned worker
- Priority
- Dependencies (can block other jobs)
- Timestamps

#### Operation (Convoy)
A batch of related jobs executed together. Operations have:
- Visual timeline/progress indicator
- Gantt-like view of parallel work
- Dependency tree visualization

#### Standing Orders
Persistent instructions assigned to a worker that execute automatically on every session startup.

---

## Technical Design

### Language & Framework
- **Language**: Go
- **TUI Framework**: Bubble Tea + Lipgloss (OpenCode-style)
- **CLI**: Cobra
- **Storage**: JSONL (append-only event log)
- **IPC**: Unix socket (`~/.cosa/cosa.sock`)
- **Git**: Native git + git worktrees (one per worker)

### Claude Code Integration

Workers interact with Claude Code CLI using:

```bash
claude --print --dangerously-skip-permissions --format json --output-format stream-json
```

Key flags:
- `--print`: Non-interactive mode
- `--dangerously-skip-permissions`: Full autonomy for workers
- `--format json`: Structured input
- `--output-format stream-json`: Parseable output

Session management:
- Store session IDs per worker
- Use `--resume <session-id>` for continuity
- "Handoff" = worker summarizes context, then fresh session with summary injected

### File Structure

```
~/.config/cosa/
├── config.yaml           # Global configuration
├── themes/               # Color theme definitions
│   ├── noir.yaml
│   ├── godfather.yaml
│   ├── miami.yaml
│   └── opencode.yaml
└── templates/            # Standing order templates

~/.cosa/
├── cosa.sock             # Unix socket for daemon
├── daemon.pid            # Daemon process ID
├── events.jsonl          # Append-only event log (The Ledger)
├── sessions/             # Worker session data
│   └── <worker-id>/
│       ├── session.json  # Claude session ID, context
│       └── handoff.md    # Last handoff summary
└── logs/
    ├── daemon.log
    └── workers/

<territory>/.cosa/
├── territory.yaml        # Territory-specific config
├── prompts/              # Custom system prompts
│   ├── territory.md      # Territory-wide context
│   └── roles/            # Role-specific prompts
└── worktrees/            # Worker git worktrees
    └── <worker-name>/
```

### Configuration

**Global config (`~/.config/cosa/config.yaml`):**

```yaml
version: 1

theme: noir  # noir | godfather | miami | opencode

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
  log_level: info  # debug | info | warn | error

notifications:
  tui_alerts: true
  system_notifications: true
  terminal_bell: true

keyboard:
  style: vim  # vim | emacs | custom
```

**Territory config (`<project>/.cosa/territory.yaml`):**

```yaml
name: my-project
branch: main  # Target branch for merges

crew:
  capo: tony
  soldatos:
    - paulie
    - silvio
    - christopher
    - bobby
  max_associates: 10

limits:
  max_concurrent_workers: 8

prompts:
  territory: prompts/territory.md
  roles:
    soldato: prompts/soldato.md
```

### Data Model

**Event (Ledger entry):**

```go
type Event struct {
    ID        string    `json:"id"`
    Type      EventType `json:"type"`
    Timestamp time.Time `json:"timestamp"`
    Territory string    `json:"territory,omitempty"`
    Worker    string    `json:"worker,omitempty"`
    Job       string    `json:"job,omitempty"`
    Data      any       `json:"data"`
}

type EventType string
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
    // ... etc
)
```

**Job:**

```go
type Job struct {
    ID           string     `json:"id"`
    Title        string     `json:"title"`
    Description  string     `json:"description"`
    Status       JobStatus  `json:"status"`
    Priority     int        `json:"priority"`  // Higher = more important
    Territory    string     `json:"territory"`
    AssignedTo   string     `json:"assigned_to,omitempty"`
    DependsOn    []string   `json:"depends_on,omitempty"`
    CreatedAt    time.Time  `json:"created_at"`
    StartedAt    *time.Time `json:"started_at,omitempty"`
    CompletedAt  *time.Time `json:"completed_at,omitempty"`
    ParentJob    string     `json:"parent_job,omitempty"`  // For decomposed jobs
    Operation    string     `json:"operation,omitempty"`   // Operation membership
}
```

**Worker:**

```go
type Worker struct {
    ID             string       `json:"id"`
    Name           string       `json:"name"`
    Role           Role         `json:"role"`
    Territory      string       `json:"territory"`
    Status         WorkerStatus `json:"status"`
    CurrentJob     string       `json:"current_job,omitempty"`
    SessionID      string       `json:"session_id,omitempty"`
    StandingOrders []string     `json:"standing_orders,omitempty"`
    WorktreePath   string       `json:"worktree_path,omitempty"`
}
```

---

## User Interface

### TUI Layout

**Dashboard View (default):**

```
┌─────────────────────────────────────────────────────────────────┐
│ COSA NOSTRA                                     ◉ 12 workers    │
├─────────────────────────────────────────────────────────────────┤
│ WORKERS                          │ JOBS                         │
│ ┌─────────────────────────────┐  │ ┌─────────────────────────┐  │
│ │ ● tony (capo)      Working  │  │ │ #142 Add auth flow  [3] │  │
│ │ ● paulie           Idle     │  │ │ #141 Fix login bug  [2] │  │
│ │ ● silvio           Working  │  │ │ #140 Update tests   [1] │  │
│ │ ○ christopher      Review   │  │ │ #139 Refactor API   [1] │  │
│ │ ● bobby            Working  │  │ │ ...                     │  │
│ └─────────────────────────────┘  │ └─────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│ ACTIVITY                                                        │
│ 14:32 silvio completed #138 - Add user profile endpoint         │
│ 14:31 consigliere approved #137 - Database migration            │
│ 14:28 paulie started #141 - Fix login bug                       │
│ 14:25 tony decomposed #142 into 3 subjobs                       │
└─────────────────────────────────────────────────────────────────┘
│ [j/k] Navigate  [Enter] Select  [n] New Job  [o] Operation  [?] │
└─────────────────────────────────────────────────────────────────┘
```

**Worker Detail View:**

```
┌─────────────────────────────────────────────────────────────────┐
│ SILVIO                                              ● Working   │
├─────────────────────────────────────────────────────────────────┤
│ Current: #138 - Add user profile endpoint                       │
│ Branch: silvio/job-138-user-profile                             │
│ Started: 12 minutes ago                                         │
├─────────────────────────────────────────────────────────────────┤
│ RECENT ACTIVITY                                                 │
│ • Created src/handlers/profile.go                               │
│ • Modified src/routes/api.go                                    │
│ • Running tests...                                              │
├─────────────────────────────────────────────────────────────────┤
│ SESSION OUTPUT                                                  │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ I'll add the user profile endpoint. First, let me check    │ │
│ │ the existing handler patterns...                           │ │
│ │                                                            │ │
│ │ I found the pattern in src/handlers/auth.go. I'll create  │ │
│ │ a similar structure for the profile handler.              │ │
│ │                                                            │ │
│ │ [Creating src/handlers/profile.go]                         │ │
│ └─────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│ MESSAGE TO SILVIO:                                              │
│ > _                                                             │
└─────────────────────────────────────────────────────────────────┘
│ [Esc] Back  [Enter] Send  [Shift+Enter] Newline  [e] Editor    │
└─────────────────────────────────────────────────────────────────┘
```

**Operation Progress View:**

```
┌─────────────────────────────────────────────────────────────────┐
│ OPERATION: Auth System Overhaul                    ████░░ 67%   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ #142 Add auth flow          ████████████████████░░░░ paulie     │
│   ├─ #142a Session mgmt     ████████████████████████ ✓          │
│   ├─ #142b Token refresh    ████████████████░░░░░░░░ silvio     │
│   └─ #142c Logout flow      ░░░░░░░░░░░░░░░░░░░░░░░░ blocked    │
│                                                                 │
│ #143 Update auth tests      ████████████░░░░░░░░░░░░ bobby      │
│                                                                 │
│ #144 Auth documentation     ░░░░░░░░░░░░░░░░░░░░░░░░ pending    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ Timeline: [=========>        ] ETA: ~15 min                     │
└─────────────────────────────────────────────────────────────────┘
```

### Color Themes

**Noir (default):**
- Background: #1a1a1a (dark gray)
- Primary: #ff4444 (red)
- Accent: #ffd700 (gold)
- Text: #e0e0e0 (light gray)
- Success: #44ff44 (green)
- Warning: #ffaa00 (orange)

**Godfather:**
- Background: #2d2416 (dark brown)
- Primary: #8b4513 (saddle brown)
- Accent: #daa520 (goldenrod)
- Text: #f5deb3 (wheat)
- Success: #556b2f (olive)
- Warning: #cd853f (peru)

**Miami (Modern Crime):**
- Background: #0a0a1a (dark blue)
- Primary: #ff1493 (deep pink)
- Accent: #00ffff (cyan)
- Text: #ffffff (white)
- Success: #00ff00 (lime)
- Warning: #ff6600 (orange)

**OpenCode:**
- Match OpenCode's default theme

### Keyboard Navigation

**Global:**
- `?` - Help
- `q` - Quit / Back
- `Esc` - Cancel / Back
- `j/k` - Navigate down/up
- `h/l` - Navigate left/right
- `g/G` - Go to top/bottom
- `/` - Search
- `:` - Command palette

**Dashboard:**
- `Tab` - Cycle focus between panels
- `n` - New job
- `o` - New operation
- `Enter` - Drill into selected item
- `r` - Refresh

**Worker View:**
- `i` - Enter message input
- `e` - Open $EDITOR for message
- `Enter` - Send message (in input mode)
- `Shift+Enter` - Newline (in input mode)
- `H` - Trigger handoff

### CLI Commands

```bash
# Daemon management
cosa start                    # Start daemon (launches wizard if first run)
cosa stop                     # Stop daemon gracefully
cosa status                   # Show daemon and family status

# TUI
cosa tui                      # Launch TUI (default command)
cosa                          # Same as `cosa tui`

# Territory management
cosa territory init           # Initialize current directory as territory
cosa territory list           # List all territories
cosa territory add <path>     # Add existing project as territory

# Family management
cosa family list              # List all workers across territories
cosa family add <territory> <name>  # Add worker to territory
cosa family remove <territory> <name>

# Job management
cosa job add "<description>"  # Add job (natural language)
cosa job add --file spec.md   # Add job from markdown file
cosa job list                 # List jobs
cosa job show <id>            # Show job details
cosa job assign <id> <worker> # Assign job to worker

# Operations
cosa operation create "<name>" --jobs <id1,id2,...>
cosa operation status <id>
cosa operation cancel <id>

# Worker interaction
cosa worker <name> message "<text>"  # Send message to worker
cosa worker <name> handoff           # Trigger handoff
cosa worker <name> status            # Show worker status

# Standing orders
cosa order set <worker> "<instructions>"
cosa order list <worker>
cosa order clear <worker>

# Logs and debugging
cosa logs                     # Stream activity log
cosa logs --worker <name>     # Filter to specific worker
cosa logs -v                  # Verbose output
```

---

## Workflows

### Job Lifecycle

```
┌─────────┐     ┌──────────┐     ┌───────────┐     ┌──────────┐
│ Created │ ──> │ Assigned │ ──> │ In Progress│ ──> │ Review   │
└─────────┘     └──────────┘     └───────────┘     └──────────┘
                                                         │
                     ┌───────────────────────────────────┤
                     │                                   │
                     v                                   v
              ┌──────────┐                        ┌──────────┐
              │ Rejected │ ───> back to worker    │ Approved │
              └──────────┘                        └──────────┘
                                                         │
                                                         v
                                                  ┌──────────┐
                                                  │  Merged  │
                                                  └──────────┘
```

1. **Job Created**: Natural language or markdown file parsed into job
2. **Job Assigned**: Underboss or Capo assigns to appropriate worker
3. **In Progress**: Worker creates branch, implements solution
4. **Review**: Tests pass → Consigliere reviews code
5. **Approved/Rejected**: Auto-merge if approved, back to worker if rejected
6. **Merged**: Code merged to target branch, job complete

### Auto-Decomposition

When a job is too large for a single worker:

1. Capo (or Underboss for cross-territory jobs) analyzes the job
2. Creates subjobs with dependencies
3. Subjobs assigned to Soldatos or Associates
4. Parent job tracks completion of all children
5. Parent completes when all children complete

### Code Review Flow

1. Worker completes implementation
2. Worker runs tests (pre-review gate)
3. If tests pass → Consigliere spawned on-demand
4. Consigliere reviews for:
   - Correctness (does it solve the problem?)
   - Style (follows project patterns)
   - Security (no vulnerabilities)
   - Performance (no obvious issues)
   - Test coverage (appropriate tests)
5. Consigliere can:
   - **Approve**: Auto-merge to target branch
   - **Approve with comments**: Merge but note improvements for future
   - **Fix minor issues**: Make small fixes directly, then approve
   - **Reject**: Send back to worker with detailed feedback

### Error Handling

**Worker Errors:**
- Workers can self-escalate when stuck
- Lookout monitors for stuck workers (no progress for configurable time)
- Escalation levels:
  - **Warning**: Logged, worker continues
  - **Error**: Worker paused, Capo notified
  - **Critical**: Worker stopped, Don notified

**API Errors (Rate limits, etc.):**
- Queue work, don't lose state
- Exponential backoff with retry
- Continue processing other work while waiting

**Merge Conflicts:**
- Detected worker escalates to Capo
- Capo attempts resolution
- If Capo cannot resolve, escalates to Don

### Handoff Process

1. Worker receives handoff signal (manual or automatic)
2. Worker summarizes:
   - Current job status
   - Key decisions made
   - Files touched
   - Open questions
3. Summary saved to `~/.cosa/sessions/<worker>/handoff.md`
4. New Claude session started
5. Summary injected as context
6. Worker continues with fresh context window

---

## System Roles Detail

### Underboss

**Responsibilities:**
- Central coordination across all territories
- Job distribution and load balancing
- Cross-territory dependencies
- Escalation handling from Capos

**Standing Orders (default):**
- Monitor job queue across territories
- Balance workload between territories
- Handle cross-territory job requests

### Capo

**Responsibilities:**
- Manage one territory
- Decompose large jobs for their crew
- Resolve merge conflicts within territory
- Spawn Associates as needed

**Standing Orders (suggested template):**
- Review completed work from Soldatos
- Triage incoming jobs by priority
- Keep crew productive

### Consigliere

**Responsibilities:**
- On-demand code review
- Quality enforcement
- Can make minor fixes directly

**Spawning:**
- Created when work is ready for review
- Destroyed after review complete
- Pool mode possible for high throughput

### Cleaner

**Responsibilities:**
- Periodic patrols (configurable interval)
- Clean up stale worktrees
- Remove orphaned branches
- Garbage collect old sessions
- Run custom cleanup plugins

### Lookout

**Responsibilities:**
- Monitor worker health
- Detect stuck workers (no progress)
- Detect crashed sessions
- Alert on anomalies

---

## Quality Gates

### Pre-Review (automated)
- [ ] Tests pass
- [ ] Lint passes (external tools/hooks)
- [ ] Build succeeds

### Consigliere Review
- [ ] Code correctness
- [ ] Style adherence
- [ ] Security check
- [ ] Performance review
- [ ] Test coverage appropriate

### Pre-Merge (automated)
- [ ] Branch up to date with target
- [ ] No merge conflicts
- [ ] All checks pass

---

## Cost Tracking

### Per-Job Tracking
Each job records:
- Total tokens used (input + output)
- Estimated cost based on model rates
- Worker sessions involved

### Dashboard Display
- Real-time cost accumulator
- Cost per completed job
- Daily/weekly cost summaries
- Cost by territory
- Cost by worker

---

## Notifications

### Channels
1. **TUI Alerts**: Visual indicator in TUI when focused
2. **System Notifications**: macOS notifications when TUI backgrounded
3. **Terminal Bell**: Audio alert for critical events

### Event Types
- Job completed
- Review needed (Consigliere decision required)
- Escalation to Don
- Worker error
- Operation complete
- Budget threshold reached

---

## Future Considerations

The following are explicitly out of scope for v1 but noted for future:

- **Plugin system**: Custom Cleaner plugins, review plugins
- **External integrations**: GitHub Issues, Linear, etc.
- **Remote operation**: Run daemon on remote server
- **Team features**: Multiple Dons, shared families
- **Voice input**: Speech-to-text for commands
- **Mobile companion**: Monitor family from phone

---

## Appendix: Mafia Terminology Glossary

| Term | Technical Meaning |
|------|-------------------|
| The Family | The entire system of workers |
| Territory | A project/repository workspace |
| The Ledger | Event log / work tracking system |
| Job | A unit of work / task |
| Operation | Batch of related jobs (convoy) |
| Standing Orders | Persistent instructions for a worker |
| Handoff | Context transfer between sessions |
| Made Man | Promoted worker (future feature) |
| The Commission | Multi-territory coordination (future) |
| Going to the mattresses | Emergency mode / all hands (future) |

---

## Open Questions

None remaining - spec is complete based on interview.

---

*Generated by spec-interview skill*
