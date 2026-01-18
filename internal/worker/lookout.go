// Package worker implements worker management for Cosa.
package worker

import (
	"context"
	"sync"
	"time"

	"cosa/internal/ledger"
)

// LookoutConfig configures the Lookout health monitor.
type LookoutConfig struct {
	// CheckInterval is how often to check worker health (default: 30s).
	CheckInterval time.Duration

	// WarningThreshold is the inactivity time before emitting a warning (default: 5m).
	WarningThreshold time.Duration

	// ErrorThreshold is the inactivity time before marking as error (default: 15m).
	ErrorThreshold time.Duration

	// CriticalThreshold is the inactivity time before stopping worker (default: 30m).
	CriticalThreshold time.Duration

	// Pool is the worker pool to monitor.
	Pool *Pool

	// Ledger for recording events.
	Ledger *ledger.Ledger

	// OnStuck is called when a worker is detected as stuck.
	OnStuck func(w *Worker, severity StuckSeverity)
}

// StuckSeverity indicates the severity level of a stuck worker.
type StuckSeverity string

const (
	SeverityWarning  StuckSeverity = "warning"
	SeverityError    StuckSeverity = "error"
	SeverityCritical StuckSeverity = "critical"
)

// Lookout monitors worker health and detects stuck workers.
type Lookout struct {
	cfg    LookoutConfig
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Track warning states to avoid duplicate alerts
	mu             sync.Mutex
	warnedWorkers  map[string]StuckSeverity
}

// NewLookout creates a new health monitor.
func NewLookout(cfg LookoutConfig) *Lookout {
	// Set defaults
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.WarningThreshold == 0 {
		cfg.WarningThreshold = 5 * time.Minute
	}
	if cfg.ErrorThreshold == 0 {
		cfg.ErrorThreshold = 15 * time.Minute
	}
	if cfg.CriticalThreshold == 0 {
		cfg.CriticalThreshold = 30 * time.Minute
	}

	return &Lookout{
		cfg:           cfg,
		warnedWorkers: make(map[string]StuckSeverity),
	}
}

// Start begins the health monitoring loop.
func (l *Lookout) Start(ctx context.Context) {
	l.ctx, l.cancel = context.WithCancel(ctx)
	l.wg.Add(1)
	go l.monitorLoop()
}

// Stop stops the health monitor.
func (l *Lookout) Stop() {
	if l.cancel != nil {
		l.cancel()
	}
	l.wg.Wait()
}

func (l *Lookout) monitorLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.checkWorkers()
		}
	}
}

func (l *Lookout) checkWorkers() {
	if l.cfg.Pool == nil {
		return
	}

	workers := l.cfg.Pool.List()
	for _, w := range workers {
		l.checkWorker(w)
	}

	// Clean up warnings for workers that are no longer stuck
	l.cleanupWarnings(workers)
}

func (l *Lookout) checkWorker(w *Worker) {
	// Only check workers that are actively working
	if w.GetStatus() != StatusWorking {
		l.clearWarning(w.ID)
		return
	}

	// Determine severity based on inactivity duration
	severity := l.determineSeverity(w)
	if severity == "" {
		l.clearWarning(w.ID)
		return
	}

	// Check if we already warned at this severity or higher
	l.mu.Lock()
	currentSeverity, hasWarning := l.warnedWorkers[w.ID]
	l.mu.Unlock()

	if hasWarning && !l.isHigherSeverity(severity, currentSeverity) {
		return // Already warned at this level
	}

	// Record the warning
	l.mu.Lock()
	l.warnedWorkers[w.ID] = severity
	l.mu.Unlock()

	// Emit event to ledger
	if l.cfg.Ledger != nil {
		l.cfg.Ledger.Append(ledger.EventWorkerStuck, WorkerStuckEventData{
			WorkerID:     w.ID,
			WorkerName:   w.Name,
			Severity:     string(severity),
			InactiveSecs: int64(time.Since(w.GetLastActivity()).Seconds()),
			JobID:        l.getCurrentJobID(w),
		})
	}

	// Execute callback
	if l.cfg.OnStuck != nil {
		l.cfg.OnStuck(w, severity)
	}

	// Take action based on severity
	switch severity {
	case SeverityError:
		// Pause the worker (mark as error status)
		w.mu.Lock()
		w.Status = StatusError
		w.mu.Unlock()

	case SeverityCritical:
		// Stop the worker
		w.Stop()
	}
}

func (l *Lookout) determineSeverity(w *Worker) StuckSeverity {
	if w.IsStuck(l.cfg.CriticalThreshold) {
		return SeverityCritical
	}
	if w.IsStuck(l.cfg.ErrorThreshold) {
		return SeverityError
	}
	if w.IsStuck(l.cfg.WarningThreshold) {
		return SeverityWarning
	}
	return ""
}

func (l *Lookout) isHigherSeverity(a, b StuckSeverity) bool {
	severityOrder := map[StuckSeverity]int{
		SeverityWarning:  1,
		SeverityError:    2,
		SeverityCritical: 3,
	}
	return severityOrder[a] > severityOrder[b]
}

func (l *Lookout) clearWarning(workerID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.warnedWorkers, workerID)
}

func (l *Lookout) cleanupWarnings(workers []*Worker) {
	activeIDs := make(map[string]bool)
	for _, w := range workers {
		activeIDs[w.ID] = true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for id := range l.warnedWorkers {
		if !activeIDs[id] {
			delete(l.warnedWorkers, id)
		}
	}
}

func (l *Lookout) getCurrentJobID(w *Worker) string {
	job := w.GetCurrentJob()
	if job != nil {
		return job.ID
	}
	return ""
}

// WorkerStuckEventData contains data for worker.stuck events.
type WorkerStuckEventData struct {
	WorkerID     string `json:"worker_id"`
	WorkerName   string `json:"worker_name"`
	Severity     string `json:"severity"`
	InactiveSecs int64  `json:"inactive_secs"`
	JobID        string `json:"job_id,omitempty"`
}
