package job

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	description := "Test job description"
	j := New(description)

	if j.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if j.Description != description {
		t.Errorf("expected description %s, got %s", description, j.Description)
	}
	if j.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, j.Status)
	}
	if j.Priority != PriorityNormal {
		t.Errorf("expected priority %d, got %d", PriorityNormal, j.Priority)
	}
	if j.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestJob_SetPriority(t *testing.T) {
	j := New("test")
	j.SetPriority(PriorityCritical)
	if j.Priority != PriorityCritical {
		t.Errorf("expected priority %d, got %d", PriorityCritical, j.Priority)
	}
}

func TestJob_SetDependencies(t *testing.T) {
	j := New("test")
	deps := []string{"dep-1", "dep-2"}
	j.SetDependencies(deps)
	if len(j.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(j.DependsOn))
	}
}

func TestJob_SetRevisionOf(t *testing.T) {
	j := New("test")
	j.SetRevisionOf("original-job-id")
	if j.RevisionOf != "original-job-id" {
		t.Errorf("expected RevisionOf 'original-job-id', got '%s'", j.RevisionOf)
	}
}

func TestJob_SetReviewFeedback(t *testing.T) {
	j := New("test")
	feedback := []string{"Fix the bug", "Add tests"}
	j.SetReviewFeedback(feedback)
	if len(j.ReviewFeedback) != 2 {
		t.Errorf("expected 2 feedback items, got %d", len(j.ReviewFeedback))
	}
}

func TestJob_Queue(t *testing.T) {
	j := New("test")
	j.Queue()
	if j.Status != StatusQueued {
		t.Errorf("expected status %s, got %s", StatusQueued, j.Status)
	}
	if j.QueuedAt == nil {
		t.Error("expected non-nil QueuedAt")
	}
}

func TestJob_Start(t *testing.T) {
	j := New("test")
	workerID := "worker-123"
	sessionID := "session-456"
	j.Start(workerID, sessionID)

	if j.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, j.Status)
	}
	if j.Worker != workerID {
		t.Errorf("expected worker %s, got %s", workerID, j.Worker)
	}
	if j.SessionID != sessionID {
		t.Errorf("expected session %s, got %s", sessionID, j.SessionID)
	}
	if j.StartedAt == nil {
		t.Error("expected non-nil StartedAt")
	}
}

func TestJob_Complete(t *testing.T) {
	j := New("test")
	j.Start("worker", "session")
	output := "Job output here"
	j.Complete(output)

	if j.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, j.Status)
	}
	if j.Output != output {
		t.Errorf("expected output %s, got %s", output, j.Output)
	}
	if j.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

func TestJob_Fail(t *testing.T) {
	j := New("test")
	j.Start("worker", "session")
	errMsg := "something went wrong"
	j.Fail(errMsg)

	if j.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, j.Status)
	}
	if j.Error != errMsg {
		t.Errorf("expected error %s, got %s", errMsg, j.Error)
	}
	if j.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

func TestJob_Cancel(t *testing.T) {
	j := New("test")
	j.Queue()
	j.Cancel()

	if j.Status != StatusCancelled {
		t.Errorf("expected status %s, got %s", StatusCancelled, j.Status)
	}
	if j.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

func TestJob_MarkForReview(t *testing.T) {
	j := New("test")
	j.Start("worker", "session")
	j.MarkForReview()

	if j.Status != StatusReview {
		t.Errorf("expected status %s, got %s", StatusReview, j.Status)
	}
}

func TestJob_GetStatus(t *testing.T) {
	j := New("test")
	if j.GetStatus() != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, j.GetStatus())
	}
	j.Queue()
	if j.GetStatus() != StatusQueued {
		t.Errorf("expected status %s, got %s", StatusQueued, j.GetStatus())
	}
}

func TestJob_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		terminal bool
	}{
		{"pending", StatusPending, false},
		{"queued", StatusQueued, false},
		{"running", StatusRunning, false},
		{"review", StatusReview, false},
		{"completed", StatusCompleted, true},
		{"failed", StatusFailed, true},
		{"cancelled", StatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := New("test")
			j.Status = tt.status
			if j.IsTerminal() != tt.terminal {
				t.Errorf("IsTerminal() for %s: expected %v, got %v", tt.status, tt.terminal, j.IsTerminal())
			}
		})
	}
}

func TestJob_ToJSON(t *testing.T) {
	j := New("test job")
	j.SetPriority(PriorityHigh)

	data, err := j.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if decoded["description"] != "test job" {
		t.Error("description mismatch in JSON")
	}
	if int(decoded["priority"].(float64)) != PriorityHigh {
		t.Error("priority mismatch in JSON")
	}
}

func TestJob_ConcurrentAccess(t *testing.T) {
	j := New("concurrent test")
	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = j.GetStatus()
			_ = j.IsTerminal()
		}()
	}

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			j.SetPriority(n % 5)
		}(i)
	}

	wg.Wait()
}

// Store tests

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.Count() != 0 {
		t.Errorf("expected empty store, got %d jobs", s.Count())
	}
}

func TestStore_Add(t *testing.T) {
	s := NewStore()
	j := New("test")
	s.Add(j)

	if s.Count() != 1 {
		t.Errorf("expected 1 job, got %d", s.Count())
	}
}

func TestStore_Get(t *testing.T) {
	s := NewStore()
	j := New("test")
	s.Add(j)

	retrieved, ok := s.Get(j.ID)
	if !ok {
		t.Fatal("expected to find job")
	}
	if retrieved.ID != j.ID {
		t.Errorf("expected ID %s, got %s", j.ID, retrieved.ID)
	}

	_, ok = s.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent job")
	}
}

func TestStore_Remove(t *testing.T) {
	s := NewStore()
	j := New("test")
	s.Add(j)
	s.Remove(j.ID)

	if s.Count() != 0 {
		t.Errorf("expected 0 jobs after removal, got %d", s.Count())
	}

	_, ok := s.Get(j.ID)
	if ok {
		t.Error("job should not be found after removal")
	}
}

func TestStore_List(t *testing.T) {
	s := NewStore()
	j1 := New("job 1")
	j2 := New("job 2")
	s.Add(j1)
	s.Add(j2)

	jobs := s.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestStore_ListByStatus(t *testing.T) {
	s := NewStore()

	j1 := New("pending job")
	j2 := New("running job")
	j2.Start("worker", "session")
	j3 := New("completed job")
	j3.Complete("done")

	s.Add(j1)
	s.Add(j2)
	s.Add(j3)

	pending := s.ListByStatus(StatusPending)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending job, got %d", len(pending))
	}

	running := s.ListByStatus(StatusRunning)
	if len(running) != 1 {
		t.Errorf("expected 1 running job, got %d", len(running))
	}

	completed := s.ListByStatus(StatusCompleted)
	if len(completed) != 1 {
		t.Errorf("expected 1 completed job, got %d", len(completed))
	}
}

func TestStore_ListByWorker(t *testing.T) {
	s := NewStore()

	j1 := New("job 1")
	j1.Start("worker-1", "session-1")
	j2 := New("job 2")
	j2.Start("worker-1", "session-2")
	j3 := New("job 3")
	j3.Start("worker-2", "session-3")

	s.Add(j1)
	s.Add(j2)
	s.Add(j3)

	worker1Jobs := s.ListByWorker("worker-1")
	if len(worker1Jobs) != 2 {
		t.Errorf("expected 2 jobs for worker-1, got %d", len(worker1Jobs))
	}

	worker2Jobs := s.ListByWorker("worker-2")
	if len(worker2Jobs) != 1 {
		t.Errorf("expected 1 job for worker-2, got %d", len(worker2Jobs))
	}
}

func TestStore_CountByStatus(t *testing.T) {
	s := NewStore()

	for i := 0; i < 3; i++ {
		j := New("pending")
		s.Add(j)
	}

	for i := 0; i < 2; i++ {
		j := New("running")
		j.Start("worker", "session")
		s.Add(j)
	}

	if s.CountByStatus(StatusPending) != 3 {
		t.Errorf("expected 3 pending, got %d", s.CountByStatus(StatusPending))
	}
	if s.CountByStatus(StatusRunning) != 2 {
		t.Errorf("expected 2 running, got %d", s.CountByStatus(StatusRunning))
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j := New("concurrent job")
			s.Add(j)
		}()
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.List()
			_ = s.Count()
		}()
	}

	wg.Wait()

	if s.Count() != 100 {
		t.Errorf("expected 100 jobs, got %d", s.Count())
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityLow != 1 {
		t.Errorf("PriorityLow should be 1, got %d", PriorityLow)
	}
	if PriorityNormal != 3 {
		t.Errorf("PriorityNormal should be 3, got %d", PriorityNormal)
	}
	if PriorityHigh != 4 {
		t.Errorf("PriorityHigh should be 4, got %d", PriorityHigh)
	}
	if PriorityCritical != 5 {
		t.Errorf("PriorityCritical should be 5, got %d", PriorityCritical)
	}

	// Verify priority ordering
	if PriorityLow >= PriorityNormal {
		t.Error("PriorityLow should be less than PriorityNormal")
	}
	if PriorityNormal >= PriorityHigh {
		t.Error("PriorityNormal should be less than PriorityHigh")
	}
	if PriorityHigh >= PriorityCritical {
		t.Error("PriorityHigh should be less than PriorityCritical")
	}
}

func TestJob_StateTransitions(t *testing.T) {
	j := New("state transition test")

	// Pending -> Queued
	if j.GetStatus() != StatusPending {
		t.Fatal("job should start in pending state")
	}

	j.Queue()
	if j.GetStatus() != StatusQueued {
		t.Fatal("job should be queued")
	}

	// Queued -> Running
	j.Start("worker", "session")
	if j.GetStatus() != StatusRunning {
		t.Fatal("job should be running")
	}

	// Running -> Review
	j.MarkForReview()
	if j.GetStatus() != StatusReview {
		t.Fatal("job should be in review")
	}
}

func TestJob_TimestampProgression(t *testing.T) {
	j := New("timestamp test")

	created := j.CreatedAt

	// Small delay to ensure distinct timestamps
	time.Sleep(time.Millisecond)
	j.Queue()
	if j.QueuedAt == nil || !j.QueuedAt.After(created) {
		t.Error("QueuedAt should be after CreatedAt")
	}

	time.Sleep(time.Millisecond)
	j.Start("worker", "session")
	if j.StartedAt == nil || !j.StartedAt.After(*j.QueuedAt) {
		t.Error("StartedAt should be after QueuedAt")
	}

	time.Sleep(time.Millisecond)
	j.Complete("done")
	if j.CompletedAt == nil || !j.CompletedAt.After(*j.StartedAt) {
		t.Error("CompletedAt should be after StartedAt")
	}
}
