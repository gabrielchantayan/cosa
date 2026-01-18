package worker

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Name: "test-worker",
		Role: RoleSoldato,
	}

	w := New(cfg)

	if w.ID == "" {
		t.Error("expected non-empty worker ID")
	}
	if w.Name != "test-worker" {
		t.Errorf("expected name 'test-worker', got '%s'", w.Name)
	}
	if w.Role != RoleSoldato {
		t.Errorf("expected role %s, got %s", RoleSoldato, w.Role)
	}
	if w.Status != StatusIdle {
		t.Errorf("expected status %s, got %s", StatusIdle, w.Status)
	}
	if w.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNew_DefaultRole(t *testing.T) {
	cfg := Config{
		Name: "worker",
		// No role specified
	}

	w := New(cfg)

	if w.Role != RoleSoldato {
		t.Errorf("expected default role %s, got %s", RoleSoldato, w.Role)
	}
}

func TestWorker_Start(t *testing.T) {
	w := New(Config{Name: "test"})

	err := w.Start()
	if err != nil {
		t.Fatalf("unexpected error starting worker: %v", err)
	}

	if w.GetStatus() != StatusIdle {
		t.Errorf("expected status %s after start, got %s", StatusIdle, w.GetStatus())
	}
}

func TestWorker_Start_AlreadyWorking(t *testing.T) {
	w := New(Config{Name: "test"})
	w.Status = StatusWorking

	err := w.Start()
	if err == nil {
		t.Error("expected error when starting a working worker")
	}
}

func TestWorker_Stop(t *testing.T) {
	w := New(Config{Name: "test"})
	w.Start()

	err := w.Stop()
	if err != nil {
		t.Fatalf("unexpected error stopping worker: %v", err)
	}

	if w.GetStatus() != StatusStopped {
		t.Errorf("expected status %s after stop, got %s", StatusStopped, w.GetStatus())
	}
}

func TestWorker_GetStatus(t *testing.T) {
	w := New(Config{Name: "test"})

	if w.GetStatus() != StatusIdle {
		t.Errorf("expected initial status %s, got %s", StatusIdle, w.GetStatus())
	}

	w.Status = StatusWorking
	if w.GetStatus() != StatusWorking {
		t.Errorf("expected status %s, got %s", StatusWorking, w.GetStatus())
	}
}

func TestWorker_GetCurrentJob(t *testing.T) {
	w := New(Config{Name: "test"})

	if w.GetCurrentJob() != nil {
		t.Error("expected nil current job initially")
	}
}

func TestWorker_UpdateActivity(t *testing.T) {
	w := New(Config{Name: "test"})

	before := time.Now()
	time.Sleep(time.Millisecond)

	w.UpdateActivity()

	lastActivity := w.GetLastActivity()
	if lastActivity.Before(before) {
		t.Error("LastActivityAt should be after the test started")
	}
}

func TestWorker_GetLastActivity(t *testing.T) {
	w := New(Config{Name: "test"})

	// Initially zero
	if !w.GetLastActivity().IsZero() {
		t.Error("expected zero last activity initially")
	}

	w.UpdateActivity()

	if w.GetLastActivity().IsZero() {
		t.Error("expected non-zero last activity after update")
	}
}

func TestWorker_IsStuck(t *testing.T) {
	w := New(Config{Name: "test"})

	// Idle worker is never stuck
	if w.IsStuck(time.Second) {
		t.Error("idle worker should not be stuck")
	}

	// Set worker as working
	w.Status = StatusWorking
	w.LastActivityAt = time.Now().Add(-2 * time.Hour)

	// Working worker with old activity is stuck
	if !w.IsStuck(time.Hour) {
		t.Error("working worker with old activity should be stuck")
	}

	// Update activity - should not be stuck anymore
	w.UpdateActivity()
	if w.IsStuck(time.Hour) {
		t.Error("worker with recent activity should not be stuck")
	}
}

func TestWorker_IsStuck_NoActivity(t *testing.T) {
	w := New(Config{Name: "test"})
	w.Status = StatusWorking
	// LastActivityAt is zero, should use CreatedAt

	// If worker just created, shouldn't be stuck
	if w.IsStuck(time.Hour) {
		t.Error("newly created working worker should not be stuck")
	}

	// Simulate old worker
	w.CreatedAt = time.Now().Add(-2 * time.Hour)
	if !w.IsStuck(time.Hour) {
		t.Error("old working worker with no activity should be stuck")
	}
}

func TestWorker_SetStandingOrders(t *testing.T) {
	w := New(Config{Name: "test"})

	orders := []string{"order 1", "order 2"}
	w.SetStandingOrders(orders)

	got := w.GetStandingOrders()
	if len(got) != 2 {
		t.Errorf("expected 2 orders, got %d", len(got))
	}
	if got[0] != "order 1" || got[1] != "order 2" {
		t.Error("orders don't match")
	}
}

func TestWorker_GetStandingOrders_Copy(t *testing.T) {
	w := New(Config{Name: "test"})
	w.SetStandingOrders([]string{"order 1"})

	// Get orders and modify
	orders := w.GetStandingOrders()
	orders[0] = "modified"

	// Original should be unchanged
	original := w.GetStandingOrders()
	if original[0] != "order 1" {
		t.Error("GetStandingOrders should return a copy")
	}
}

func TestWorker_AddStandingOrder(t *testing.T) {
	w := New(Config{Name: "test"})

	w.AddStandingOrder("order 1")
	w.AddStandingOrder("order 2")

	orders := w.GetStandingOrders()
	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}
}

func TestWorker_ClearStandingOrders(t *testing.T) {
	w := New(Config{Name: "test"})
	w.SetStandingOrders([]string{"order 1", "order 2"})

	w.ClearStandingOrders()

	orders := w.GetStandingOrders()
	if len(orders) != 0 {
		t.Errorf("expected 0 orders after clear, got %d", len(orders))
	}
}

func TestWorker_UpdateCost(t *testing.T) {
	w := New(Config{Name: "test"})

	w.UpdateCost("$1.50", 1500)

	if w.TotalCost != "$1.50" {
		t.Errorf("expected cost '$1.50', got '%s'", w.TotalCost)
	}
	if w.TotalTokens != 1500 {
		t.Errorf("expected 1500 tokens, got %d", w.TotalTokens)
	}
}

func TestWorker_Events(t *testing.T) {
	w := New(Config{Name: "test"})

	events := w.Events()
	if events == nil {
		t.Error("expected non-nil events channel")
	}
}

func TestWorker_ToJSON(t *testing.T) {
	w := New(Config{Name: "test-worker", Role: RoleCapo})

	data, err := w.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic check that it's valid JSON containing expected fields
	str := string(data)
	if len(str) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestWorker_ConcurrentAccess(t *testing.T) {
	w := New(Config{Name: "test"})
	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.GetStatus()
			_ = w.GetCurrentJob()
			_ = w.GetLastActivity()
			_ = w.GetStandingOrders()
		}()
	}

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.UpdateActivity()
		}()
	}

	wg.Wait()
}

func TestWorker_SendMessage_NotWorking(t *testing.T) {
	w := New(Config{Name: "test"})

	err := w.SendMessage("hello")
	if err == nil {
		t.Error("expected error when sending message to non-working worker")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants
	if StatusIdle != "idle" {
		t.Errorf("StatusIdle should be 'idle', got '%s'", StatusIdle)
	}
	if StatusWorking != "working" {
		t.Errorf("StatusWorking should be 'working', got '%s'", StatusWorking)
	}
	if StatusReviewing != "reviewing" {
		t.Errorf("StatusReviewing should be 'reviewing', got '%s'", StatusReviewing)
	}
	if StatusStopped != "stopped" {
		t.Errorf("StatusStopped should be 'stopped', got '%s'", StatusStopped)
	}
	if StatusError != "error" {
		t.Errorf("StatusError should be 'error', got '%s'", StatusError)
	}
}

func TestWorker_EventCallback(t *testing.T) {
	var receivedEvent *Event
	cfg := Config{
		Name: "test",
		OnEvent: func(e Event) {
			receivedEvent = &e
		},
	}

	w := New(cfg)
	w.Start()

	// The start event should have been emitted
	if receivedEvent == nil {
		t.Error("expected event callback to be called on start")
	}
	if receivedEvent.Type != "started" {
		t.Errorf("expected event type 'started', got '%s'", receivedEvent.Type)
	}
}

func TestWorker_StopEvent(t *testing.T) {
	var lastEvent *Event
	cfg := Config{
		Name: "test",
		OnEvent: func(e Event) {
			lastEvent = &e
		},
	}

	w := New(cfg)
	w.Start()
	w.Stop()

	if lastEvent == nil {
		t.Error("expected event callback to be called on stop")
	}
	if lastEvent.Type != "stopped" {
		t.Errorf("expected event type 'stopped', got '%s'", lastEvent.Type)
	}
}

func TestEvent_Fields(t *testing.T) {
	e := Event{
		Type:    "test",
		Worker:  "worker-123",
		Job:     "job-456",
		Message: "test message",
		Time:    time.Now(),
	}

	if e.Type != "test" {
		t.Errorf("Type mismatch: got %s", e.Type)
	}
	if e.Worker != "worker-123" {
		t.Errorf("Worker mismatch: got %s", e.Worker)
	}
	if e.Job != "job-456" {
		t.Errorf("Job mismatch: got %s", e.Job)
	}
	if e.Message != "test message" {
		t.Errorf("Message mismatch: got %s", e.Message)
	}
	if e.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}

func TestConfig_Defaults(t *testing.T) {
	// Config with minimal fields
	cfg := Config{
		Name: "minimal",
	}

	w := New(cfg)

	// Role should default to Soldato
	if w.Role != RoleSoldato {
		t.Errorf("expected default role %s, got %s", RoleSoldato, w.Role)
	}

	// Worker should start idle
	if w.Status != StatusIdle {
		t.Errorf("expected initial status %s, got %s", StatusIdle, w.Status)
	}

	// Should have generated UUID
	if w.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestWorker_Ephemeral(t *testing.T) {
	w := New(Config{Name: "ephemeral-test"})
	w.Ephemeral = true

	if !w.Ephemeral {
		t.Error("expected worker to be marked as ephemeral")
	}
}

func TestWorker_JobStats(t *testing.T) {
	w := New(Config{Name: "test"})

	// Initial stats should be zero
	if w.JobsCompleted != 0 {
		t.Errorf("expected 0 completed jobs, got %d", w.JobsCompleted)
	}
	if w.JobsFailed != 0 {
		t.Errorf("expected 0 failed jobs, got %d", w.JobsFailed)
	}

	// Simulate job completion (normally done internally)
	w.JobsCompleted = 5
	w.JobsFailed = 2

	if w.JobsCompleted != 5 {
		t.Errorf("expected 5 completed jobs, got %d", w.JobsCompleted)
	}
	if w.JobsFailed != 2 {
		t.Errorf("expected 2 failed jobs, got %d", w.JobsFailed)
	}
}
