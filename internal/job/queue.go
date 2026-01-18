package job

import (
	"container/heap"
	"sync"
)

// Queue is a priority queue with dependency resolution.
type Queue struct {
	heap    jobHeap         // Min-heap ordered by priority DESC, created_at ASC
	pending map[string]*Job // Jobs waiting for dependencies
	store   *Store          // For dependency lookups
	mu      sync.RWMutex
}

// NewQueue creates a new job queue.
func NewQueue(store *Store) *Queue {
	q := &Queue{
		heap:    make(jobHeap, 0),
		pending: make(map[string]*Job),
		store:   store,
	}
	heap.Init(&q.heap)
	return q
}

// Enqueue adds a job to the queue.
// Jobs with unmet dependencies go to pending, others go to the heap.
func (q *Queue) Enqueue(j *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.checkDependencies(j) {
		heap.Push(&q.heap, j)
	} else {
		q.pending[j.ID] = j
	}
	return nil
}

// Dequeue returns the highest-priority ready job, or nil if none available.
func (q *Queue) Dequeue() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.heap.Len() == 0 {
		return nil
	}

	j := heap.Pop(&q.heap).(*Job)
	return j
}

// Peek returns the highest-priority ready job without removing it.
func (q *Queue) Peek() *Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.heap.Len() == 0 {
		return nil
	}

	return q.heap[0]
}

// GetReady returns all jobs ready for execution.
func (q *Queue) GetReady() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	jobs := make([]*Job, len(q.heap))
	copy(jobs, q.heap)
	return jobs
}

// GetPending returns all jobs blocked on dependencies.
func (q *Queue) GetPending() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	jobs := make([]*Job, 0, len(q.pending))
	for _, j := range q.pending {
		jobs = append(jobs, j)
	}
	return jobs
}

// NotifyCompletion is called when a job completes successfully.
// It checks pending jobs to see if any can now be moved to ready.
func (q *Queue) NotifyCompletion(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check each pending job to see if it can now be executed
	for id, j := range q.pending {
		if q.checkDependencies(j) {
			delete(q.pending, id)
			heap.Push(&q.heap, j)
		}
	}
}

// NotifyFailure is called when a job fails.
// It cascades failure to all jobs that depend on the failed job.
func (q *Queue) NotifyFailure(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Find and mark all jobs that depend on the failed job
	failedJobs := []string{jobID}

	for len(failedJobs) > 0 {
		currentID := failedJobs[0]
		failedJobs = failedJobs[1:]

		for id, j := range q.pending {
			for _, dep := range j.DependsOn {
				if dep == currentID {
					j.Fail("dependency failed: " + currentID)
					delete(q.pending, id)
					failedJobs = append(failedJobs, j.ID)
					break
				}
			}
		}
	}
}

// Remove removes a job from the queue by ID.
// Returns true if the job was found and removed.
func (q *Queue) Remove(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check pending first
	if _, ok := q.pending[jobID]; ok {
		delete(q.pending, jobID)
		return true
	}

	// Search in heap
	for i, j := range q.heap {
		if j.ID == jobID {
			heap.Remove(&q.heap, i)
			return true
		}
	}

	return false
}

// Len returns the total number of jobs in the queue (ready + pending).
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.heap) + len(q.pending)
}

// ReadyLen returns the number of ready jobs.
func (q *Queue) ReadyLen() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.heap)
}

// PendingLen returns the number of pending jobs.
func (q *Queue) PendingLen() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.pending)
}

// checkDependencies returns true if all dependencies are in a terminal state.
// Must be called with lock held.
func (q *Queue) checkDependencies(j *Job) bool {
	if len(j.DependsOn) == 0 {
		return true
	}

	for _, depID := range j.DependsOn {
		dep, ok := q.store.Get(depID)
		if !ok {
			// Dependency doesn't exist, treat as not satisfied
			return false
		}
		if !dep.IsTerminal() {
			return false
		}
		// If dependency failed or was cancelled, this job shouldn't run
		status := dep.GetStatus()
		if status == StatusFailed || status == StatusCancelled {
			return false
		}
	}
	return true
}

// jobHeap implements heap.Interface for priority queue functionality.
// Jobs are ordered by: priority DESC (higher first), then created_at ASC (older first).
type jobHeap []*Job

func (h jobHeap) Len() int { return len(h) }

func (h jobHeap) Less(i, j int) bool {
	// Higher priority first
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	// Earlier created_at first (FIFO for same priority)
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}

func (h jobHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *jobHeap) Push(x interface{}) {
	*h = append(*h, x.(*Job))
}

func (h *jobHeap) Pop() interface{} {
	old := *h
	n := len(old)
	job := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return job
}
