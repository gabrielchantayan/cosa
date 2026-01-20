package job

import (
	"sync"
	"testing"
	"time"
)

func TestNewQueue(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	if q == nil {
		t.Fatal("expected non-nil queue")
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d items", q.Len())
	}
	if q.ReadyLen() != 0 {
		t.Errorf("expected 0 ready, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending, got %d", q.PendingLen())
	}
}

func TestQueue_Enqueue_NoDependencies(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	j := New("test job")
	store.Add(j)

	if err := q.Enqueue(j); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.ReadyLen() != 1 {
		t.Errorf("expected 1 ready job, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending jobs, got %d", q.PendingLen())
	}
}

func TestQueue_Enqueue_WithUnmetDependencies(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency job but don't complete it
	dep := New("dependency")
	store.Add(dep)

	// Create job that depends on dep
	j := New("dependent job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	if err := q.Enqueue(j); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.ReadyLen() != 0 {
		t.Errorf("expected 0 ready jobs, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending job, got %d", q.PendingLen())
	}
}

func TestQueue_Enqueue_WithMetDependencies(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create and complete dependency
	dep := New("dependency")
	dep.Complete("done")
	store.Add(dep)

	// Create job that depends on completed dep
	j := New("dependent job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	if err := q.Enqueue(j); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.ReadyLen() != 1 {
		t.Errorf("expected 1 ready job, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending jobs, got %d", q.PendingLen())
	}
}

func TestQueue_Dequeue(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	j := New("test job")
	store.Add(j)
	q.Enqueue(j)

	dequeued := q.Dequeue()
	if dequeued == nil {
		t.Fatal("expected non-nil job from dequeue")
	}
	if dequeued.ID != j.ID {
		t.Errorf("expected job ID %s, got %s", j.ID, dequeued.ID)
	}

	if q.ReadyLen() != 0 {
		t.Errorf("expected 0 ready jobs after dequeue, got %d", q.ReadyLen())
	}
}

func TestQueue_Dequeue_EmptyQueue(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	dequeued := q.Dequeue()
	if dequeued != nil {
		t.Errorf("expected nil from empty queue, got %v", dequeued)
	}
}

func TestQueue_Peek(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	j := New("test job")
	store.Add(j)
	q.Enqueue(j)

	peeked := q.Peek()
	if peeked == nil {
		t.Fatal("expected non-nil job from peek")
	}
	if peeked.ID != j.ID {
		t.Errorf("expected job ID %s, got %s", j.ID, peeked.ID)
	}

	// Queue should still have the job
	if q.ReadyLen() != 1 {
		t.Errorf("expected 1 ready job after peek, got %d", q.ReadyLen())
	}
}

func TestQueue_Peek_EmptyQueue(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	peeked := q.Peek()
	if peeked != nil {
		t.Errorf("expected nil from empty queue peek, got %v", peeked)
	}
}

func TestQueue_PriorityOrdering(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Add jobs with different priorities
	low := New("low priority")
	low.SetPriority(PriorityLow)
	store.Add(low)

	high := New("high priority")
	high.SetPriority(PriorityHigh)
	store.Add(high)

	normal := New("normal priority")
	normal.SetPriority(PriorityNormal)
	store.Add(normal)

	// Enqueue in random order
	q.Enqueue(low)
	q.Enqueue(high)
	q.Enqueue(normal)

	// Should dequeue in priority order (high first)
	first := q.Dequeue()
	if first.Priority != PriorityHigh {
		t.Errorf("expected high priority first, got %d", first.Priority)
	}

	second := q.Dequeue()
	if second.Priority != PriorityNormal {
		t.Errorf("expected normal priority second, got %d", second.Priority)
	}

	third := q.Dequeue()
	if third.Priority != PriorityLow {
		t.Errorf("expected low priority third, got %d", third.Priority)
	}
}

func TestQueue_FIFO_SamePriority(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Add jobs with same priority
	first := New("first")
	first.SetPriority(PriorityNormal)
	store.Add(first)

	// Ensure different created times
	time.Sleep(time.Millisecond)

	second := New("second")
	second.SetPriority(PriorityNormal)
	store.Add(second)

	q.Enqueue(first)
	q.Enqueue(second)

	// Should dequeue in FIFO order for same priority
	dequeued1 := q.Dequeue()
	if dequeued1.ID != first.ID {
		t.Errorf("expected first job, got %s", dequeued1.Description)
	}

	dequeued2 := q.Dequeue()
	if dequeued2.ID != second.ID {
		t.Errorf("expected second job, got %s", dequeued2.Description)
	}
}

func TestQueue_GetReady(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	j1 := New("job 1")
	j2 := New("job 2")
	store.Add(j1)
	store.Add(j2)
	q.Enqueue(j1)
	q.Enqueue(j2)

	ready := q.GetReady()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready jobs, got %d", len(ready))
	}
}

func TestQueue_GetPending(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency that's not completed
	dep := New("dependency")
	store.Add(dep)

	// Create jobs that depend on it
	j1 := New("job 1")
	j1.SetDependencies([]string{dep.ID})
	j2 := New("job 2")
	j2.SetDependencies([]string{dep.ID})
	store.Add(j1)
	store.Add(j2)

	q.Enqueue(j1)
	q.Enqueue(j2)

	pending := q.GetPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending jobs, got %d", len(pending))
	}
}

func TestQueue_NotifyCompletion(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency
	dep := New("dependency")
	store.Add(dep)

	// Create dependent job
	j := New("dependent job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	q.Enqueue(j)

	// Verify job is pending
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending job, got %d", q.PendingLen())
	}

	// Complete the dependency
	dep.Complete("done")

	// Notify queue
	q.NotifyCompletion(dep.ID)

	// Job should now be ready
	if q.ReadyLen() != 1 {
		t.Errorf("expected 1 ready job, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending jobs, got %d", q.PendingLen())
	}
}

func TestQueue_NotifyFailure(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency
	dep := New("dependency")
	store.Add(dep)

	// Create dependent job
	j := New("dependent job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	q.Enqueue(j)

	// Fail the dependency
	dep.Fail("error")

	// Notify queue
	q.NotifyFailure(dep.ID)

	// Dependent job should be removed from pending
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending jobs, got %d", q.PendingLen())
	}

	// And should be marked as failed
	if j.GetStatus() != StatusFailed {
		t.Errorf("expected dependent job to be failed, got %s", j.GetStatus())
	}
}

func TestQueue_NotifyFailure_Cascade(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create chain: A -> B -> C
	jobA := New("job A")
	store.Add(jobA)

	jobB := New("job B")
	jobB.SetDependencies([]string{jobA.ID})
	store.Add(jobB)

	jobC := New("job C")
	jobC.SetDependencies([]string{jobB.ID})
	store.Add(jobC)

	q.Enqueue(jobB)
	q.Enqueue(jobC)

	// Fail job A
	jobA.Fail("error")
	q.NotifyFailure(jobA.ID)

	// Both B and C should be failed
	if jobB.GetStatus() != StatusFailed {
		t.Errorf("expected job B to be failed, got %s", jobB.GetStatus())
	}
	if jobC.GetStatus() != StatusFailed {
		t.Errorf("expected job C to be failed, got %s", jobC.GetStatus())
	}
}

func TestQueue_Remove_FromPending(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency that's not completed
	dep := New("dependency")
	store.Add(dep)

	// Create dependent job
	j := New("dependent job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	q.Enqueue(j)

	// Remove the pending job
	removed := q.Remove(j.ID)
	if !removed {
		t.Error("expected job to be removed")
	}
	if q.PendingLen() != 0 {
		t.Errorf("expected 0 pending jobs, got %d", q.PendingLen())
	}
}

func TestQueue_Remove_FromReady(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	j := New("test job")
	store.Add(j)
	q.Enqueue(j)

	// Remove the ready job
	removed := q.Remove(j.ID)
	if !removed {
		t.Error("expected job to be removed")
	}
	if q.ReadyLen() != 0 {
		t.Errorf("expected 0 ready jobs, got %d", q.ReadyLen())
	}
}

func TestQueue_Remove_NotFound(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	removed := q.Remove("nonexistent")
	if removed {
		t.Error("expected false for nonexistent job removal")
	}
}

func TestQueue_Len(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Add ready job
	ready := New("ready")
	store.Add(ready)
	q.Enqueue(ready)

	// Add pending job
	dep := New("dep")
	store.Add(dep)
	pending := New("pending")
	pending.SetDependencies([]string{dep.ID})
	store.Add(pending)
	q.Enqueue(pending)

	if q.Len() != 2 {
		t.Errorf("expected total length 2, got %d", q.Len())
	}
	if q.ReadyLen() != 1 {
		t.Errorf("expected ready length 1, got %d", q.ReadyLen())
	}
	if q.PendingLen() != 1 {
		t.Errorf("expected pending length 1, got %d", q.PendingLen())
	}
}

func TestQueue_MultipleDependencies(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create two dependencies
	dep1 := New("dep 1")
	dep2 := New("dep 2")
	store.Add(dep1)
	store.Add(dep2)

	// Create job that depends on both
	j := New("multi-dep job")
	j.SetDependencies([]string{dep1.ID, dep2.ID})
	store.Add(j)

	q.Enqueue(j)

	// Should be pending
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending, got %d", q.PendingLen())
	}

	// Complete first dep
	dep1.Complete("done")
	q.NotifyCompletion(dep1.ID)

	// Still pending (second dep not done)
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending after first dep, got %d", q.PendingLen())
	}

	// Complete second dep
	dep2.Complete("done")
	q.NotifyCompletion(dep2.ID)

	// Now ready
	if q.ReadyLen() != 1 {
		t.Errorf("expected 1 ready after both deps, got %d", q.ReadyLen())
	}
}

func TestQueue_FailedDependency_BlocksJob(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency that fails
	dep := New("failing dep")
	dep.Fail("error")
	store.Add(dep)

	// Create job depending on failed dep
	j := New("blocked job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	q.Enqueue(j)

	// Should be pending (failed dep blocks it)
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending, got %d", q.PendingLen())
	}
	if q.ReadyLen() != 0 {
		t.Errorf("expected 0 ready, got %d", q.ReadyLen())
	}
}

func TestQueue_CancelledDependency_BlocksJob(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create dependency that's cancelled
	dep := New("cancelled dep")
	dep.Cancel()
	store.Add(dep)

	// Create job depending on cancelled dep
	j := New("blocked job")
	j.SetDependencies([]string{dep.ID})
	store.Add(j)

	q.Enqueue(j)

	// Should be pending (cancelled dep blocks it)
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending, got %d", q.PendingLen())
	}
}

func TestQueue_NonexistentDependency(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Create job depending on nonexistent dep
	j := New("orphan job")
	j.SetDependencies([]string{"nonexistent-id"})
	store.Add(j)

	q.Enqueue(j)

	// Should be pending (unknown dep blocks it)
	if q.PendingLen() != 1 {
		t.Errorf("expected 1 pending, got %d", q.PendingLen())
	}
}

func TestQueue_ConcurrentAccess(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)
	var wg sync.WaitGroup

	// Concurrent enqueues
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j := New("concurrent job")
			store.Add(j)
			q.Enqueue(j)
		}()
	}

	// Concurrent dequeues
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Dequeue()
		}()
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Len()
			_ = q.ReadyLen()
			_ = q.PendingLen()
			_ = q.GetReady()
			_ = q.GetPending()
		}()
	}

	wg.Wait()
}

// Test the heap implementation directly
func TestJobHeap_Basic(t *testing.T) {
	store := NewStore()
	q := NewQueue(store)

	// Add multiple jobs
	for i := 0; i < 10; i++ {
		j := New("job")
		j.SetPriority(i % 5)
		store.Add(j)
		q.Enqueue(j)
	}

	// Verify heap property maintained
	lastPriority := 100 // Start high
	for q.ReadyLen() > 0 {
		j := q.Dequeue()
		if j.Priority > lastPriority {
			t.Errorf("heap property violated: %d > %d", j.Priority, lastPriority)
		}
		lastPriority = j.Priority
	}
}
