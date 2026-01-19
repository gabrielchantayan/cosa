// Package ledger implements a JSONL append-only event log.
package ledger

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the type of event.
type EventType string

const (
	// Daemon events
	EventDaemonStarted  EventType = "daemon.started"
	EventDaemonStopped  EventType = "daemon.stopped"

	// Territory events
	EventTerritoryInit EventType = "territory.init"

	// Worker events
	EventWorkerAdded    EventType = "worker.added"
	EventWorkerStarted  EventType = "worker.started"
	EventWorkerStopped  EventType = "worker.stopped"
	EventWorkerRemoved  EventType = "worker.removed"
	EventWorkerError    EventType = "worker.error"
	EventWorkerStuck    EventType = "worker.stuck"
	EventWorkerMessage  EventType = "worker.message"

	// Job events
	EventJobCreated   EventType = "job.created"
	EventJobQueued    EventType = "job.queued"
	EventJobStarted   EventType = "job.started"
	EventJobCompleted EventType = "job.completed"
	EventJobFailed    EventType = "job.failed"
	EventJobCancelled EventType = "job.cancelled"

	// Claude events
	EventClaudeMessage  EventType = "claude.message"
	EventClaudeToolCall EventType = "claude.tool_call"
	EventClaudeResult   EventType = "claude.result"

	// Cost events
	EventCostRecord EventType = "cost.record"

	// Review events
	EventReviewStarted  EventType = "review.started"
	EventReviewApproved EventType = "review.approved"
	EventReviewRejected EventType = "review.rejected"

	// Gate events
	EventGateStarted EventType = "gate.started"
	EventGatePassed  EventType = "gate.passed"
	EventGateFailed  EventType = "gate.failed"

	// Merge events
	EventMergeStarted   EventType = "merge.started"
	EventMergeCompleted EventType = "merge.completed"
	EventMergeFailed    EventType = "merge.failed"
)

// Event represents a single event in the ledger.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Ledger manages the append-only event log.
type Ledger struct {
	path string
	file *os.File
	mu   sync.Mutex

	// Subscribers for real-time events
	subs   []chan<- Event
	subsMu sync.RWMutex
}

// Open opens or creates a ledger at the given path.
func Open(path string) (*Ledger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Ledger{
		path: path,
		file: file,
		subs: make([]chan<- Event, 0),
	}, nil
}

// Close closes the ledger file.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Append writes an event to the ledger.
func (l *Ledger) Append(eventType EventType, data interface{}) (*Event, error) {
	var rawData json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = d
	}

	event := &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      rawData,
	}

	line, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	if _, err := l.file.Write(append(line, '\n')); err != nil {
		l.mu.Unlock()
		return nil, err
	}
	l.mu.Unlock()

	// Notify subscribers
	l.notifySubscribers(*event)

	return event, nil
}

// Subscribe adds a channel to receive real-time events.
func (l *Ledger) Subscribe(ch chan<- Event) {
	l.subsMu.Lock()
	defer l.subsMu.Unlock()
	l.subs = append(l.subs, ch)
}

// Unsubscribe removes a channel from receiving events.
func (l *Ledger) Unsubscribe(ch chan<- Event) {
	l.subsMu.Lock()
	defer l.subsMu.Unlock()
	for i, sub := range l.subs {
		if sub == ch {
			l.subs = append(l.subs[:i], l.subs[i+1:]...)
			return
		}
	}
}

func (l *Ledger) notifySubscribers(event Event) {
	l.subsMu.RLock()
	defer l.subsMu.RUnlock()

	for _, ch := range l.subs {
		select {
		case ch <- event:
		default:
			// Drop if channel is full
		}
	}
}

// Read reads all events from the ledger file.
func Read(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // Skip malformed lines
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

// ReadSince reads events after the given timestamp.
func ReadSince(path string, since time.Time) ([]Event, error) {
	events, err := Read(path)
	if err != nil {
		return nil, err
	}

	var filtered []Event
	for _, e := range events {
		if e.Timestamp.After(since) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// Tail reads the last n events from the ledger.
func Tail(path string, n int) ([]Event, error) {
	events, err := Read(path)
	if err != nil {
		return nil, err
	}

	if len(events) <= n {
		return events, nil
	}
	return events[len(events)-n:], nil
}

// Common event data structures

// DaemonEventData contains data for daemon events.
type DaemonEventData struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

// WorkerEventData contains data for worker events.
type WorkerEventData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Worktree string `json:"worktree,omitempty"`
	Error    string `json:"error,omitempty"`
}

// JobEventData contains data for job events.
type JobEventData struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Worker      string `json:"worker,omitempty"`
	WorkerName  string `json:"worker_name,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ClaudeEventData contains data for Claude Code events.
type ClaudeEventData struct {
	SessionID string `json:"session_id"`
	Worker    string `json:"worker"`
	Job       string `json:"job"`
	Message   string `json:"message,omitempty"`
	Tool      string `json:"tool,omitempty"`
}

// ReviewEventData contains data for review events.
type ReviewEventData struct {
	JobID         string `json:"job_id"`
	WorkerID      string `json:"worker_id"`
	WorkerName    string `json:"worker_name,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Feedback      string `json:"feedback,omitempty"`
	RevisionJobID string `json:"revision_job_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// GateEventData contains data for gate events.
type GateEventData struct {
	JobID    string `json:"job_id"`
	WorkerID string `json:"worker_id"`
	GateName string `json:"gate_name,omitempty"`
	Output   string `json:"output,omitempty"`
	Duration int64  `json:"duration,omitempty"` // milliseconds
	Error    string `json:"error,omitempty"`
}

// MergeEventData contains data for merge events.
type MergeEventData struct {
	JobID         string   `json:"job_id"`
	WorkerBranch  string   `json:"worker_branch,omitempty"`
	BaseBranch    string   `json:"base_branch,omitempty"`
	MergeCommit   string   `json:"merge_commit,omitempty"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// CostEventData contains data for cost tracking events.
type CostEventData struct {
	JobID       string `json:"job_id"`
	WorkerID    string `json:"worker_id"`
	WorkerName  string `json:"worker_name"`
	Cost        string `json:"cost"`
	Tokens      int    `json:"tokens"`
	TotalCost   string `json:"total_cost"`   // Running total
	TotalTokens int    `json:"total_tokens"` // Running total
}
