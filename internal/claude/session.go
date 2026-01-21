package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionInfo represents stored session information.
type SessionInfo struct {
	SessionID  string    `json:"session_id"`
	WorkerID   string    `json:"worker_id"`
	WorkerName string    `json:"worker_name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsed   time.Time `json:"last_used"`
}

// SessionStore manages session persistence.
type SessionStore struct {
	path     string                   // Directory for session files
	sessions map[string]*SessionInfo  // Keyed by session ID
	mu       sync.RWMutex
}

// NewSessionStore creates a new session store.
func NewSessionStore(path string) (*SessionStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	store := &SessionStore{
		path:     path,
		sessions: make(map[string]*SessionInfo),
	}

	// Load existing sessions
	if err := store.loadAll(); err != nil {
		return nil, fmt.Errorf("failed to load sessions: %w", err)
	}

	return store, nil
}

// Save persists a session to disk.
func (s *SessionStore) Save(info *SessionInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info.LastUsed = time.Now()
	s.sessions[info.SessionID] = info

	// Write to file
	filePath := s.sessionFilePath(info.SessionID)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Load retrieves a session by ID.
func (s *SessionStore) Load(sessionID string) (*SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	return info, nil
}

// LoadByWorker retrieves a session by worker ID.
func (s *SessionStore) LoadByWorker(workerID string) (*SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, info := range s.sessions {
		if info.WorkerID == workerID {
			return info, nil
		}
	}

	return nil, fmt.Errorf("no session found for worker %q", workerID)
}

// LoadByWorkerName retrieves a session by worker name.
func (s *SessionStore) LoadByWorkerName(workerName string) (*SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, info := range s.sessions {
		if info.WorkerName == workerName {
			return info, nil
		}
	}

	return nil, fmt.Errorf("no session found for worker %q", workerName)
}

// Delete removes a session.
func (s *SessionStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)

	filePath := s.sessionFilePath(sessionID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// DeleteByWorker removes sessions for a worker.
func (s *SessionStore) DeleteByWorker(workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var toDelete []string
	for sessionID, info := range s.sessions {
		if info.WorkerID == workerID {
			toDelete = append(toDelete, sessionID)
		}
	}

	for _, sessionID := range toDelete {
		delete(s.sessions, sessionID)
		filePath := s.sessionFilePath(sessionID)
		os.Remove(filePath)
	}

	return nil
}

// List returns all stored sessions.
func (s *SessionStore) List() []*SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*SessionInfo, 0, len(s.sessions))
	for _, info := range s.sessions {
		sessions = append(sessions, info)
	}
	return sessions
}

// Cleanup removes sessions older than maxAge.
func (s *SessionStore) Cleanup(maxAge time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var removed int

	for sessionID, info := range s.sessions {
		if info.LastUsed.Before(cutoff) {
			delete(s.sessions, sessionID)
			filePath := s.sessionFilePath(sessionID)
			os.Remove(filePath)
			removed++
		}
	}

	return removed, nil
}

// Update updates the LastUsed timestamp for a session.
func (s *SessionStore) Update(sessionID string) error {
	s.mu.Lock()
	info, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session %q not found", sessionID)
	}
	info.LastUsed = time.Now()
	s.mu.Unlock()

	// Persist to disk
	return s.Save(info)
}

func (s *SessionStore) sessionFilePath(sessionID string) string {
	// Use a safe filename
	safeName := sessionID
	if len(safeName) > 64 {
		safeName = safeName[:64]
	}
	return filepath.Join(s.path, safeName+".json")
}

func (s *SessionStore) loadAll() error {
	entries, err := os.ReadDir(s.path)
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

		filePath := filepath.Join(s.path, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		var info SessionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue // Skip unparseable files
		}

		s.sessions[info.SessionID] = &info
	}

	return nil
}

// Count returns the number of stored sessions.
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
