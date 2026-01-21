package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"cosa/internal/job"
)

// ErrInvalidWorkerName is returned when a worker name contains invalid characters.
var ErrInvalidWorkerName = errors.New("invalid worker name")

// validWorkerNameRegex matches valid worker names: alphanumeric, hyphens, underscores.
// Must start with alphanumeric, 1-64 characters total.
var validWorkerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidateWorkerName checks if a worker name is valid and safe for use in file paths.
// Valid names contain only alphanumeric characters, hyphens, and underscores,
// must start with an alphanumeric character, and be 1-64 characters long.
func ValidateWorkerName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidWorkerName)
	}

	// Check for path traversal attempts
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("%w: name contains path traversal characters", ErrInvalidWorkerName)
	}

	// Check against regex pattern
	if !validWorkerNameRegex.MatchString(name) {
		return fmt.Errorf("%w: name must be 1-64 characters, start with alphanumeric, and contain only alphanumeric, hyphens, or underscores", ErrInvalidWorkerName)
	}

	return nil
}

// sanitizeWorkerNameForPath ensures the worker name is safe for file path construction.
// Returns an error if the name is invalid.
func sanitizeWorkerNameForPath(basePath, name string) (string, error) {
	if err := ValidateWorkerName(name); err != nil {
		return "", err
	}

	// Construct the path and verify it stays within basePath
	fullPath := filepath.Join(basePath, name+".json")
	cleanPath := filepath.Clean(fullPath)

	// Verify the path is still under basePath
	if !strings.HasPrefix(cleanPath, filepath.Clean(basePath)+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path escapes base directory", ErrInvalidWorkerName)
	}

	return cleanPath, nil
}

// WorkerInfo contains the persistent worker metadata.
type WorkerInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Role           Role     `json:"role"`
	Worktree       string   `json:"worktree"`
	Branch         string   `json:"branch"`
	StandingOrders []string `json:"standing_orders,omitempty"`
	SessionID      string   `json:"session_id,omitempty"`
	JobsCompleted  int      `json:"jobs_completed"`
	JobsFailed     int      `json:"jobs_failed"`
}

// Pool manages a collection of workers with availability tracking.
type Pool struct {
	workers map[string]*Worker // Keyed by name
	byRole  map[Role][]*Worker // Indexed by role
	path    string             // Directory for worker persistence (empty = no persistence)
	pending []WorkerInfo       // Workers loaded from disk, awaiting full initialization
	mu      sync.RWMutex
	onIdle  func(*Worker) // Callback when worker becomes idle
}

// NewPool creates a new in-memory worker pool (no persistence).
func NewPool() *Pool {
	return &Pool{
		workers: make(map[string]*Worker),
		byRole:  make(map[Role][]*Worker),
	}
}

// NewPersistentPool creates a worker pool with disk persistence.
// Note: Workers are loaded as metadata only; call InitializeWorker to fully initialize each.
func NewPersistentPool(path string) (*Pool, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workers directory: %w", err)
	}

	p := &Pool{
		workers: make(map[string]*Worker),
		byRole:  make(map[Role][]*Worker),
		path:    path,
	}

	if err := p.loadAll(); err != nil {
		return nil, fmt.Errorf("failed to load workers: %w", err)
	}

	return p, nil
}

// PendingWorkers returns workers loaded from disk that need initialization.
func (p *Pool) PendingWorkers() []WorkerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]WorkerInfo, len(p.pending))
	copy(result, p.pending)
	return result
}

// ClearPending clears the pending workers list after initialization.
func (p *Pool) ClearPending() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = nil
}

// Add adds a worker to the pool.
func (p *Pool) Add(w *Worker) error {
	// Validate worker name before adding
	if err := ValidateWorkerName(w.Name); err != nil {
		return fmt.Errorf("cannot add worker: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.workers[w.Name]; exists {
		return fmt.Errorf("worker %q already exists", w.Name)
	}

	p.workers[w.Name] = w
	p.byRole[w.Role] = append(p.byRole[w.Role], w)

	if p.path != "" {
		if err := p.saveWorker(w); err != nil {
			// Rollback on save failure
			delete(p.workers, w.Name)
			for i, worker := range p.byRole[w.Role] {
				if worker.Name == w.Name {
					p.byRole[w.Role] = append(p.byRole[w.Role][:i], p.byRole[w.Role][i+1:]...)
					break
				}
			}
			return fmt.Errorf("failed to persist worker: %w", err)
		}
	}

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

	if p.path != "" {
		if filePath, err := p.workerFilePath(name); err == nil {
			os.Remove(filePath)
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

// Save persists a worker to disk (if persistence is enabled).
func (p *Pool) Save(w *Worker) error {
	if p.path == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saveWorker(w)
}

// Persistence helpers

func (p *Pool) workerFilePath(name string) (string, error) {
	return sanitizeWorkerNameForPath(p.path, name)
}

func (p *Pool) saveWorker(w *Worker) error {
	info := WorkerInfo{
		ID:             w.ID,
		Name:           w.Name,
		Role:           w.Role,
		Worktree:       w.Worktree,
		Branch:         w.Branch,
		StandingOrders: w.StandingOrders,
		SessionID:      w.SessionID,
		JobsCompleted:  w.JobsCompleted,
		JobsFailed:     w.JobsFailed,
	}

	filePath, err := p.workerFilePath(w.Name)
	if err != nil {
		return fmt.Errorf("invalid worker name for persistence: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (p *Pool) loadAll() error {
	entries, err := os.ReadDir(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(p.path, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		var info WorkerInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue // Skip unparseable files
		}

		p.pending = append(p.pending, info)
	}

	return nil
}
