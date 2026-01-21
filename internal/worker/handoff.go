// Package worker implements worker management for Cosa.
package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sanitizeWorkerDirPath validates the worker name and returns a safe directory path.
func sanitizeWorkerDirPath(basePath, workerName string) (string, error) {
	if err := ValidateWorkerName(workerName); err != nil {
		return "", err
	}

	// Construct the path and verify it stays within basePath
	fullPath := filepath.Join(basePath, workerName)
	cleanPath := filepath.Clean(fullPath)

	// Verify the path is still under basePath
	cleanBase := filepath.Clean(basePath)
	if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) && cleanPath != cleanBase {
		return "", fmt.Errorf("%w: path escapes base directory", ErrInvalidWorkerName)
	}

	return cleanPath, nil
}

// HandoffSummary contains a summary of a worker's current state for handoff.
type HandoffSummary struct {
	WorkerID      string    `json:"worker_id"`
	WorkerName    string    `json:"worker_name"`
	JobID         string    `json:"job_id,omitempty"`
	Status        string    `json:"status"`
	Decisions     []string  `json:"decisions,omitempty"`
	FilesTouched  []string  `json:"files_touched,omitempty"`
	OpenQuestions []string  `json:"open_questions,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// GenerateHandoffSummary creates a handoff summary for this worker.
func (w *Worker) GenerateHandoffSummary() *HandoffSummary {
	w.mu.RLock()
	defer w.mu.RUnlock()

	summary := &HandoffSummary{
		WorkerID:   w.ID,
		WorkerName: w.Name,
		Status:     string(w.Status),
		CreatedAt:  time.Now(),
	}

	if w.CurrentJob != nil {
		summary.JobID = w.CurrentJob.ID
	}

	// Note: In a full implementation, we would analyze the session output
	// to extract decisions, files touched, and open questions.
	// This would involve parsing Claude's output from the session history.

	return summary
}

// InjectHandoffContext injects a handoff summary into the worker's context.
func (w *Worker) InjectHandoffContext(summary *HandoffSummary) {
	if summary == nil {
		return
	}

	// Build context string from summary
	var context []string

	if summary.Summary != "" {
		context = append(context, "Previous work summary: "+summary.Summary)
	}

	if len(summary.Decisions) > 0 {
		context = append(context, "Key decisions made:")
		for _, d := range summary.Decisions {
			context = append(context, "  - "+d)
		}
	}

	if len(summary.FilesTouched) > 0 {
		context = append(context, "Files modified:")
		for _, f := range summary.FilesTouched {
			context = append(context, "  - "+f)
		}
	}

	if len(summary.OpenQuestions) > 0 {
		context = append(context, "Open questions:")
		for _, q := range summary.OpenQuestions {
			context = append(context, "  - "+q)
		}
	}

	// Add context as standing orders
	w.mu.Lock()
	w.StandingOrders = append([]string{"[HANDOFF CONTEXT]"}, context...)
	w.mu.Unlock()
}

// SaveHandoffSummary persists a handoff summary to disk.
func SaveHandoffSummary(sessionsDir, workerName string, summary *HandoffSummary) error {
	dir, err := sanitizeWorkerDirPath(sessionsDir, workerName)
	if err != nil {
		return fmt.Errorf("invalid worker name: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "handoff.json")
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadHandoffSummary loads a handoff summary from disk.
func LoadHandoffSummary(sessionsDir, workerName string) (*HandoffSummary, error) {
	dir, err := sanitizeWorkerDirPath(sessionsDir, workerName)
	if err != nil {
		return nil, fmt.Errorf("invalid worker name: %w", err)
	}

	path := filepath.Join(dir, "handoff.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var summary HandoffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

// HandoffStore manages handoff summaries.
type HandoffStore struct {
	sessionsDir string
}

// NewHandoffStore creates a new handoff store.
func NewHandoffStore(sessionsDir string) *HandoffStore {
	return &HandoffStore{
		sessionsDir: sessionsDir,
	}
}

// Save saves a handoff summary for a worker.
func (s *HandoffStore) Save(summary *HandoffSummary) error {
	return SaveHandoffSummary(s.sessionsDir, summary.WorkerName, summary)
}

// Load loads a handoff summary for a worker.
func (s *HandoffStore) Load(workerName string) (*HandoffSummary, error) {
	return LoadHandoffSummary(s.sessionsDir, workerName)
}

// Exists checks if a handoff summary exists for a worker.
func (s *HandoffStore) Exists(workerName string) bool {
	dir, err := sanitizeWorkerDirPath(s.sessionsDir, workerName)
	if err != nil {
		return false
	}
	path := filepath.Join(dir, "handoff.json")
	_, err = os.Stat(path)
	return err == nil
}

// Delete removes a handoff summary for a worker.
func (s *HandoffStore) Delete(workerName string) error {
	dir, err := sanitizeWorkerDirPath(s.sessionsDir, workerName)
	if err != nil {
		return fmt.Errorf("invalid worker name: %w", err)
	}
	path := filepath.Join(dir, "handoff.json")
	return os.Remove(path)
}
