package worker

import (
	"fmt"
	"sync"

	"cosa/internal/job"
)

// Pool manages a collection of workers with availability tracking.
type Pool struct {
	workers map[string]*Worker    // Keyed by name
	byRole  map[Role][]*Worker    // Indexed by role
	mu      sync.RWMutex
	onIdle  func(*Worker)         // Callback when worker becomes idle
}

// NewPool creates a new worker pool.
func NewPool() *Pool {
	return &Pool{
		workers: make(map[string]*Worker),
		byRole:  make(map[Role][]*Worker),
	}
}

// Add adds a worker to the pool.
func (p *Pool) Add(w *Worker) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.workers[w.Name]; exists {
		return fmt.Errorf("worker %q already exists", w.Name)
	}

	p.workers[w.Name] = w
	p.byRole[w.Role] = append(p.byRole[w.Role], w)

	return nil
}

// Remove removes a worker from the pool by name.
func (p *Pool) Remove(name string) (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, exists := p.workers[name]
	if !exists {
		return nil, fmt.Errorf("worker %q not found", name)
	}

	delete(p.workers, name)

	// Remove from role index
	workers := p.byRole[w.Role]
	for i, worker := range workers {
		if worker.Name == name {
			p.byRole[w.Role] = append(workers[:i], workers[i+1:]...)
			break
		}
	}

	return w, nil
}

// Get returns a worker by name.
func (p *Pool) Get(name string) (*Worker, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	w, ok := p.workers[name]
	return w, ok
}

// GetByID returns a worker by ID.
func (p *Pool) GetByID(id string) (*Worker, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, w := range p.workers {
		if w.ID == id {
			return w, true
		}
	}
	return nil, false
}

// List returns all workers.
func (p *Pool) List() []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*Worker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	return workers
}

// ListByRole returns all workers with the given role.
func (p *Pool) ListByRole(role Role) []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := p.byRole[role]
	result := make([]*Worker, len(workers))
	copy(result, workers)
	return result
}

// Count returns the total number of workers.
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// GetAvailable returns all idle workers.
func (p *Pool) GetAvailable() []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var available []*Worker
	for _, w := range p.workers {
		if w.GetStatus() == StatusIdle {
			available = append(available, w)
		}
	}
	return available
}

// GetAvailableByRole returns all idle workers with the given role.
func (p *Pool) GetAvailableByRole(role Role) []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var available []*Worker
	for _, w := range p.byRole[role] {
		if w.GetStatus() == StatusIdle {
			available = append(available, w)
		}
	}
	return available
}

// FindBestWorker selects the best available worker for a job.
// Selection criteria:
// 1. Must be idle
// 2. Must be a worker role (Soldato or Capo)
// 3. Prefer Soldato over Capo for regular work
// 4. Among same role, prefer worker with fewer completed jobs (load balancing)
func (p *Pool) FindBestWorker(j *job.Job) *Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *Worker
	var bestScore int = -1

	// Worker roles that can execute jobs
	workerRoles := []Role{RoleSoldato, RoleCapo}

	for _, role := range workerRoles {
		for _, w := range p.byRole[role] {
			if w.GetStatus() != StatusIdle {
				continue
			}

			// Calculate score: prefer workers with fewer completed jobs
			// Lower jobs completed = higher score (better candidate)
			score := 1000 - w.JobsCompleted

			// Prefer Soldatos over Capos for regular work
			if w.Role == RoleSoldato {
				score += 100
			}

			if score > bestScore {
				bestScore = score
				best = w
			}
		}
	}

	return best
}

// SetOnIdle sets the callback for when a worker becomes idle.
func (p *Pool) SetOnIdle(fn func(*Worker)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onIdle = fn
}

// NotifyIdle is called when a worker becomes idle.
func (p *Pool) NotifyIdle(w *Worker) {
	p.mu.RLock()
	fn := p.onIdle
	p.mu.RUnlock()

	if fn != nil {
		fn(w)
	}
}

// Exists checks if a worker with the given name exists.
func (p *Pool) Exists(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.workers[name]
	return exists
}

// StopAll stops all workers in the pool.
func (p *Pool) StopAll() {
	p.mu.RLock()
	workers := make([]*Worker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	p.mu.RUnlock()

	for _, w := range workers {
		w.Stop()
	}
}
