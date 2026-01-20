# Underboss Chat in TUI

## Overview

Add a dedicated chat interface to the Cosa TUI for conversing with The Underboss, the AI second-in-command that coordinates the development organization. This brings the existing `cosa chat` functionality into the TUI with enhanced integration, real-time worker visibility, and AI-driven job management capabilities.

## Goals

1. **Unified Interface** - All interactions happen within the TUI without switching to separate CLI commands
2. **Context While Chatting** - See worker activity and status while discussing strategy with The Underboss
3. **Workflow Integration** - Create and manage jobs directly through natural language conversation
4. **Convenience** - Easier access to The Underboss than running a separate command

## Non-Goals

- Replacing the `cosa chat` CLI command (it will continue to work)
- Adding cost tracking or rate limiting
- Chat history export (ledger already tracks important actions)
- Complex @ mention or special syntax in input

---

## Feature Specification

### Layout & Navigation

#### Page Structure
- **Separate full-page view** accessible from dashboard via `c` keybinding
- **70/30 split layout**:
  - Left/center (70%): Chat area with message history and input
  - Right (30%): Worker sidebar showing status at a glance

#### Navigation
- `c` from dashboard: Enter chat page
- `Escape` (when input is empty): Return to dashboard
- `?`: Show context-aware help overlay with chat-specific keybindings
- `:`: Open command palette with full TUI command access

### Chat Interface

#### Message Display
- **Color-coded messages**: Distinct colors/styles for user messages vs Underboss responses
- **Scrollable history**: Support both vim-style (j/k when not in input mode) and mouse scrolling
- **Markdown rendering**: Stream raw text during generation, re-render with full markdown formatting when response completes

#### Input Area
- Multi-line support via Shift+Enter for newlines
- Enter sends message
- Plain text only (no special @ mentions or syntax)
- Focus on input by default when entering page

#### Loading & Cancellation
- **Animated spinner** displayed while waiting for Underboss response
- **Escape to cancel**: Press Escape while waiting to cancel the pending response

#### Error Handling
- Display error messages inline in the chat where the response would have been
- No retry mechanism - user can simply send another message

### Session Management

#### Persistence
- **Persistent session** that survives page navigation within TUI
- **Resume daemon session** on TUI restart - if daemon has an active chat session, resume it
- Session indicator in header showing "Resumed session" vs "New session"

#### Session Control
- **Clear/new session** available via command palette
- No automatic session expiration

### Worker Sidebar

#### Display
- Compact list of all workers
- Each entry shows: worker name + current status + current job title (if working)
- Real-time updates when workers change state or jobs are created/completed

#### Interaction
- **Informational only** - no click/select interaction
- Use Escape to return to dashboard for worker interactions

### Underboss Capabilities

#### Personality & Tone
- **Full mafia character**: Speaks like a mob boss with slang ("capisce?", "fuggedaboutit", references to "the family", "making things happen", etc.)
- Displayed as **"The Underboss"** in chat interface

#### Tool Access via MCP
The Underboss has access to tools through an MCP server exposed inline by the daemon:

**Read-only queries (everything queryable):**
- `cosa_list_workers` - Get all workers and their current states
- `cosa_get_worker` - Get detailed info on a specific worker
- `cosa_list_jobs` - Get job queue with status
- `cosa_get_job` - Get detailed job info
- `cosa_list_activity` - Get recent activity events
- `cosa_list_territories` - Get territory information
- `cosa_list_operations` - Get operation status
- `cosa_get_costs` - Get cost/token usage statistics

**Write operations (jobs only):**
- `cosa_create_job` - Create a new job with any priority/settings
- `cosa_cancel_job` - Cancel a pending or in-progress job
- `cosa_set_job_priority` - Update job priority

**No constraints** on job creation - The Underboss can create any job with any priority.

#### Tool Feedback
- Tool calls and results displayed **inline** as part of the chat message flow
- Sidebar updates in real-time when state changes occur

### Implementation Approach

#### Chat Backend
- Keep existing Claude Code with `--resume` approach
- Add MCP server capability inline in the daemon
- Configure Claude Code to connect to daemon's MCP endpoint for tool access

#### MCP Server Integration
- Daemon exposes MCP endpoint (stdio or socket-based for Claude Code)
- Tools use `cosa_` prefix naming convention
- No separate MCP server process - integrated directly in daemon

#### System Prompt Update
Update The Underboss system prompt to:
1. Embrace full mafia character with appropriate slang and mannerisms
2. Describe available MCP tools and when to use them
3. Maintain helpfulness while staying in character

---

## UI Mockup

```
┌─ Cosa ─────────────────────────────────────────────────────────────────────┐
│ Chat with The Underboss                          [New session] [?] Help    │
├────────────────────────────────────────────────────┬───────────────────────┤
│                                                    │ WORKERS               │
│  You:                                              │                       │
│  Hey, what's the status of the family?             │ ● tony                │
│                                                    │   working: fix auth   │
│  The Underboss:                                    │                       │
│  Ay, good to hear from ya, boss. Let me check     │ ● paulie              │
│  on the crew...                                    │   idle                │
│                                                    │                       │
│  *checks worker status*                            │ ○ silvio              │
│                                                    │   stopped             │
│  Alright, here's the situation. We got Tony        │                       │
│  workin' on that authentication thing - good       │ ● christopher         │
│  soldier, that one. Paulie's sittin' around        │   working: add tests  │
│  waitin' for orders, capisce? Silvio's takin'     │                       │
│  a break. Christopher's handlin' the test          │───────────────────────│
│  coverage like a pro.                              │ JOBS                  │
│                                                    │                       │
│  You need me to put anyone to work? I can          │ 3 pending             │
│  make things happen.                               │ 2 in progress         │
│                                                    │ 12 completed          │
│                                                    │                       │
│                                                    │                       │
│                                                    │                       │
├────────────────────────────────────────────────────┴───────────────────────┤
│ > Create a job to refactor the database module                        [⏎] │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Keyboard Reference

| Key | Action |
|-----|--------|
| `c` | Enter chat from dashboard |
| `Escape` | Return to dashboard (when input empty) / Cancel pending response |
| `Enter` | Send message |
| `Shift+Enter` | Insert newline in message |
| `j/k` | Scroll chat history (when not in input mode) |
| `Mouse scroll` | Scroll chat history |
| `?` | Show help overlay |
| `:` | Open command palette |

---

## Technical Design

### New Files

1. **`internal/tui/page/chat.go`** - Chat page component
   - Message history viewport
   - Input textarea
   - Worker sidebar integration
   - Loading state management

2. **`internal/daemon/mcp.go`** - MCP server implementation
   - Tool definitions for Cosa management
   - Integration with daemon RPC methods
   - stdio transport for Claude Code

### Modified Files

1. **`internal/tui/app.go`**
   - Add chat page to page routing
   - Add `c` keybinding to navigate to chat
   - Maintain chat page state across navigation

2. **`internal/daemon/server.go`**
   - Initialize MCP server
   - Add MCP endpoint for Claude Code connection

3. **`internal/daemon/chat.go`**
   - Update system prompt for full mafia character
   - Configure MCP server connection for Claude Code sessions

### Data Flow

```
User types message in TUI
        ↓
TUI calls ChatSend RPC to daemon
        ↓
Daemon's ChatSession.Send() resumes Claude Code with message
        ↓
Claude Code connects to daemon's MCP server for tools
        ↓
Claude executes tools → daemon processes → returns results
        ↓
Claude generates response
        ↓
Response streamed back to daemon
        ↓
Daemon sends response to TUI
        ↓
TUI renders markdown and updates display
        ↓
Sidebar updates if state changed
```

### MCP Tool Definitions

```go
// Read-only tools
cosa_list_workers() -> []Worker
cosa_get_worker(name: string) -> Worker
cosa_list_jobs(status?: string) -> []Job
cosa_get_job(id: string) -> Job
cosa_list_activity(limit?: int) -> []Event
cosa_list_territories() -> []Territory
cosa_list_operations() -> []Operation
cosa_get_costs() -> CostSummary

// Write tools (jobs only)
cosa_create_job(description: string, priority?: int, territory?: string) -> Job
cosa_cancel_job(id: string) -> void
cosa_set_job_priority(id: string, priority: int) -> void
```

---

## Testing Considerations

1. **Unit tests** for MCP tool handlers
2. **Integration tests** for chat + tool execution flow
3. **TUI tests** for page navigation and rendering
4. **Manual testing** for:
   - Chat session persistence across page navigation
   - Session resume on TUI restart
   - Real-time sidebar updates
   - Markdown rendering
   - Escape cancellation during response

---

## Open Questions

None - all requirements have been clarified during the interview.
