package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	// File should be created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected ledger file to be created")
	}
}

func TestOpen_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create existing file with content
	if err := os.WriteFile(path, []byte(`{"id":"1","type":"test"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open existing ledger: %v", err)
	}
	defer l.Close()

	// Should be able to append
	_, err = l.Append(EventDaemonStarted, nil)
	if err != nil {
		t.Fatalf("failed to append to existing ledger: %v", err)
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	err = l.Close()
	if err != nil {
		t.Errorf("failed to close ledger: %v", err)
	}

	// Should be safe to close multiple times
	err = l.Close()
	if err != nil {
		// This might fail, but shouldn't panic
	}
}

func TestAppend(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	event, err := l.Append(EventDaemonStarted, DaemonEventData{Version: "1.0.0", PID: 1234})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	if event.ID == "" {
		t.Error("expected event ID to be set")
	}
	if event.Type != EventDaemonStarted {
		t.Errorf("expected type %s, got %s", EventDaemonStarted, event.Type)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestAppend_NilData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	event, err := l.Append(EventDaemonStopped, nil)
	if err != nil {
		t.Fatalf("failed to append event with nil data: %v", err)
	}

	if event.Data != nil {
		t.Error("expected nil data in event")
	}
}

func TestAppend_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	l.Append(EventWorkerAdded, WorkerEventData{ID: "w1", Name: "paulie", Role: "soldato"})
	l.Close()

	// Read file contents
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected file to contain event data")
	}

	// Verify JSON is parseable
	var event Event
	if err := json.Unmarshal(content[:len(content)-1], &event); err != nil {
		t.Errorf("failed to parse written event: %v", err)
	}
}

func TestRead(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	// Write some events
	l.Append(EventDaemonStarted, DaemonEventData{Version: "1.0.0"})
	l.Append(EventWorkerAdded, WorkerEventData{Name: "paulie"})
	l.Append(EventJobCreated, JobEventData{ID: "job-1"})
	l.Close()

	// Read events
	events, err := Read(path)
	if err != nil {
		t.Fatalf("failed to read events: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	if events[0].Type != EventDaemonStarted {
		t.Errorf("expected first event type %s, got %s", EventDaemonStarted, events[0].Type)
	}
}

func TestRead_NonExistentFile(t *testing.T) {
	events, err := Read("/nonexistent/path/ledger.jsonl")
	if err != nil {
		t.Errorf("expected nil error for nonexistent file, got: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

func TestRead_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Write file with some malformed lines
	content := `{"id":"1","type":"daemon.started","timestamp":"2024-01-01T00:00:00Z"}
not valid json
{"id":"2","type":"worker.added","timestamp":"2024-01-01T00:00:01Z"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	events, err := Read(path)
	if err != nil {
		t.Fatalf("failed to read events: %v", err)
	}

	// Should skip malformed lines
	if len(events) != 2 {
		t.Errorf("expected 2 valid events, got %d", len(events))
	}
}

func TestReadSince(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	// Write events with some delay to ensure different timestamps
	l.Append(EventDaemonStarted, nil)
	time.Sleep(10 * time.Millisecond)
	cutoff := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	l.Append(EventWorkerAdded, nil)
	l.Append(EventJobCreated, nil)
	l.Close()

	events, err := ReadSince(path, cutoff)
	if err != nil {
		t.Fatalf("failed to read events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events after cutoff, got %d", len(events))
	}
}

func TestTail(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	// Write 5 events
	for i := 0; i < 5; i++ {
		l.Append(EventJobCreated, JobEventData{ID: string(rune('1' + i))})
	}
	l.Close()

	// Tail last 3
	events, err := Tail(path, 3)
	if err != nil {
		t.Fatalf("failed to tail events: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestTail_LessThanN(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}

	// Write 2 events
	l.Append(EventJobCreated, nil)
	l.Append(EventJobCompleted, nil)
	l.Close()

	// Ask for 10 - should return all 2
	events, err := Tail(path, 10)
	if err != nil {
		t.Fatalf("failed to tail events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestSubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	ch := make(chan Event, 10)
	l.Subscribe(ch)

	// Append an event
	l.Append(EventWorkerAdded, WorkerEventData{Name: "paulie"})

	// Should receive event
	select {
	case event := <-ch:
		if event.Type != EventWorkerAdded {
			t.Errorf("expected event type %s, got %s", EventWorkerAdded, event.Type)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestUnsubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	ch := make(chan Event, 10)
	l.Subscribe(ch)
	l.Unsubscribe(ch)

	// Append an event
	l.Append(EventWorkerAdded, nil)

	// Should NOT receive event
	select {
	case <-ch:
		t.Error("should not receive event after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	ch1 := make(chan Event, 10)
	ch2 := make(chan Event, 10)
	l.Subscribe(ch1)
	l.Subscribe(ch2)

	l.Append(EventJobCreated, nil)

	// Both should receive
	for _, ch := range []chan Event{ch1, ch2} {
		select {
		case event := <-ch:
			if event.Type != EventJobCreated {
				t.Errorf("expected event type %s, got %s", EventJobCreated, event.Type)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	}
}

func TestSubscribe_DropsIfFull(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	// Use unbuffered channel
	ch := make(chan Event)
	l.Subscribe(ch)

	// This should not block even though no one is reading
	done := make(chan bool)
	go func() {
		l.Append(EventJobCreated, nil)
		done <- true
	}()

	select {
	case <-done:
		// Expected - append should not block
	case <-time.After(time.Second):
		t.Error("append blocked on full subscriber channel")
	}
}

func TestConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	numEvents := 100

	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.Append(EventJobCreated, JobEventData{ID: string(rune('0' + i%10))})
		}(i)
	}

	wg.Wait()

	// Verify all events were written
	l.Close()
	events, err := Read(path)
	if err != nil {
		t.Fatalf("failed to read events: %v", err)
	}

	if len(events) != numEvents {
		t.Errorf("expected %d events, got %d", numEvents, len(events))
	}
}

func TestEventTypes(t *testing.T) {
	// Verify event type constants
	types := []EventType{
		EventDaemonStarted,
		EventDaemonStopped,
		EventTerritoryInit,
		EventWorkerAdded,
		EventWorkerStarted,
		EventWorkerStopped,
		EventWorkerRemoved,
		EventWorkerError,
		EventWorkerStuck,
		EventWorkerMessage,
		EventJobCreated,
		EventJobQueued,
		EventJobStarted,
		EventJobCompleted,
		EventJobFailed,
		EventJobCancelled,
		EventClaudeMessage,
		EventClaudeToolCall,
		EventClaudeResult,
		EventCostRecord,
		EventReviewStarted,
		EventReviewApproved,
		EventReviewRejected,
		EventGateStarted,
		EventGatePassed,
		EventGateFailed,
		EventMergeStarted,
		EventMergeCompleted,
		EventMergeFailed,
	}

	for _, et := range types {
		if string(et) == "" {
			t.Errorf("event type should not be empty")
		}
	}
}

func TestEventDataStructures(t *testing.T) {
	// Test that data structures can be serialized/deserialized
	tests := []struct {
		name string
		data interface{}
	}{
		{"DaemonEventData", DaemonEventData{Version: "1.0.0", PID: 1234}},
		{"WorkerEventData", WorkerEventData{ID: "w1", Name: "paulie", Role: "soldato", Worktree: "/path", Error: ""}},
		{"JobEventData", JobEventData{ID: "j1", Description: "test job", Worker: "w1", Error: ""}},
		{"ClaudeEventData", ClaudeEventData{SessionID: "s1", Worker: "w1", Job: "j1", Message: "hello", Tool: "Bash"}},
		{"ReviewEventData", ReviewEventData{JobID: "j1", WorkerID: "w1", Summary: "looks good"}},
		{"GateEventData", GateEventData{JobID: "j1", WorkerID: "w1", GateName: "test", Duration: 1000}},
		{"MergeEventData", MergeEventData{JobID: "j1", WorkerBranch: "feature", BaseBranch: "main", MergeCommit: "abc123"}},
		{"CostEventData", CostEventData{JobID: "j1", WorkerID: "w1", WorkerName: "paulie", Cost: "$0.50", Tokens: 1000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.data)
			if err != nil {
				t.Errorf("failed to marshal %s: %v", tt.name, err)
			}
			if len(data) == 0 {
				t.Errorf("expected non-empty JSON for %s", tt.name)
			}
		})
	}
}
