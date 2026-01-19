package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSessionStore(t *testing.T) {
	dir := t.TempDir()

	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	if store == nil {
		t.Fatal("NewSessionStore returned nil")
	}
	if store.Count() != 0 {
		t.Errorf("expected empty store, got %d sessions", store.Count())
	}
}

func TestNewSessionStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "sessions")

	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	if store == nil {
		t.Fatal("NewSessionStore returned nil")
	}

	// Verify directory was created
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, not file")
	}
}

func TestSessionStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}

	err := store.Save(info)
	if err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	loaded, err := store.Load("session-123")
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if loaded.SessionID != info.SessionID {
		t.Errorf("expected session ID %q, got %q", info.SessionID, loaded.SessionID)
	}
	if loaded.WorkerID != info.WorkerID {
		t.Errorf("expected worker ID %q, got %q", info.WorkerID, loaded.WorkerID)
	}
	if loaded.WorkerName != info.WorkerName {
		t.Errorf("expected worker name %q, got %q", info.WorkerName, loaded.WorkerName)
	}
}

func TestSessionStore_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("expected error when loading nonexistent session")
	}
}

func TestSessionStore_LoadByWorker(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	store.Save(info)

	loaded, err := store.LoadByWorker("worker-1")
	if err != nil {
		t.Fatalf("failed to load session by worker: %v", err)
	}
	if loaded.SessionID != "session-123" {
		t.Errorf("expected session ID %q, got %q", "session-123", loaded.SessionID)
	}

	// Not found case
	_, err = store.LoadByWorker("nonexistent")
	if err == nil {
		t.Error("expected error when loading by nonexistent worker")
	}
}

func TestSessionStore_LoadByWorkerName(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	store.Save(info)

	loaded, err := store.LoadByWorkerName("paulie")
	if err != nil {
		t.Fatalf("failed to load session by worker name: %v", err)
	}
	if loaded.SessionID != "session-123" {
		t.Errorf("expected session ID %q, got %q", "session-123", loaded.SessionID)
	}

	// Not found case
	_, err = store.LoadByWorkerName("nonexistent")
	if err == nil {
		t.Error("expected error when loading by nonexistent worker name")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	store.Save(info)

	err := store.Delete("session-123")
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	_, err = store.Load("session-123")
	if err == nil {
		t.Error("expected error when loading deleted session")
	}

	// Verify file is removed
	filePath := filepath.Join(dir, "session-123.json")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected session file to be deleted")
	}
}

func TestSessionStore_Delete_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	// Should not error when deleting non-existent session
	err := store.Delete("nonexistent")
	if err != nil {
		t.Errorf("unexpected error deleting nonexistent session: %v", err)
	}
}

func TestSessionStore_DeleteByWorker(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	// Add multiple sessions for same worker
	info1 := &SessionInfo{
		SessionID:  "session-1",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	info2 := &SessionInfo{
		SessionID:  "session-2",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	info3 := &SessionInfo{
		SessionID:  "session-3",
		WorkerID:   "worker-2",
		WorkerName: "silvio",
		CreatedAt:  time.Now(),
	}

	store.Save(info1)
	store.Save(info2)
	store.Save(info3)

	err := store.DeleteByWorker("worker-1")
	if err != nil {
		t.Fatalf("failed to delete by worker: %v", err)
	}

	// Sessions for worker-1 should be gone
	_, err = store.Load("session-1")
	if err == nil {
		t.Error("expected session-1 to be deleted")
	}
	_, err = store.Load("session-2")
	if err == nil {
		t.Error("expected session-2 to be deleted")
	}

	// Session for worker-2 should remain
	loaded, err := store.Load("session-3")
	if err != nil {
		t.Fatalf("session-3 should still exist: %v", err)
	}
	if loaded.WorkerName != "silvio" {
		t.Errorf("expected silvio, got %q", loaded.WorkerName)
	}
}

func TestSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	infos := []*SessionInfo{
		{SessionID: "s1", WorkerID: "w1", WorkerName: "paulie", CreatedAt: time.Now()},
		{SessionID: "s2", WorkerID: "w2", WorkerName: "silvio", CreatedAt: time.Now()},
		{SessionID: "s3", WorkerID: "w3", WorkerName: "tony", CreatedAt: time.Now()},
	}

	for _, info := range infos {
		store.Save(info)
	}

	list := store.List()
	if len(list) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(list))
	}
}

func TestSessionStore_Count(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}

	store.Save(&SessionInfo{SessionID: "s1", WorkerID: "w1", WorkerName: "paulie", CreatedAt: time.Now()})
	if store.Count() != 1 {
		t.Errorf("expected 1, got %d", store.Count())
	}

	store.Save(&SessionInfo{SessionID: "s2", WorkerID: "w2", WorkerName: "silvio", CreatedAt: time.Now()})
	if store.Count() != 2 {
		t.Errorf("expected 2, got %d", store.Count())
	}
}

func TestSessionStore_Cleanup(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	now := time.Now()

	// Add old and new sessions
	oldSession := &SessionInfo{
		SessionID:  "old-session",
		WorkerID:   "w1",
		WorkerName: "paulie",
		CreatedAt:  now.Add(-48 * time.Hour),
		LastUsed:   now.Add(-48 * time.Hour),
	}
	newSession := &SessionInfo{
		SessionID:  "new-session",
		WorkerID:   "w2",
		WorkerName: "silvio",
		CreatedAt:  now,
		LastUsed:   now,
	}

	// Save directly to map to preserve LastUsed times
	store.sessions[oldSession.SessionID] = oldSession
	store.sessions[newSession.SessionID] = newSession

	// Cleanup sessions older than 24 hours
	removed, err := store.Cleanup(24 * time.Hour)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Old session should be gone
	_, err = store.Load("old-session")
	if err == nil {
		t.Error("expected old session to be removed")
	}

	// New session should remain
	_, err = store.Load("new-session")
	if err != nil {
		t.Error("expected new session to still exist")
	}
}

func TestSessionStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now().Add(-1 * time.Hour),
		LastUsed:   time.Now().Add(-1 * time.Hour),
	}
	store.Save(info)

	oldLastUsed := info.LastUsed

	// Wait a moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	err := store.Update("session-123")
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	loaded, _ := store.Load("session-123")
	if !loaded.LastUsed.After(oldLastUsed) {
		t.Error("expected LastUsed to be updated to a later time")
	}
}

func TestSessionStore_Update_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	err := store.Update("nonexistent")
	if err == nil {
		t.Error("expected error when updating nonexistent session")
	}
}

func TestSessionStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	// Create store and add session
	store1, _ := NewSessionStore(dir)
	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}
	store1.Save(info)

	// Create new store that should load existing session
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}

	loaded, err := store2.Load("session-123")
	if err != nil {
		t.Fatalf("failed to load persisted session: %v", err)
	}
	if loaded.WorkerName != "paulie" {
		t.Errorf("expected paulie, got %q", loaded.WorkerName)
	}
}

func TestSessionStore_LongSessionID(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	// Create session with very long ID
	longID := ""
	for i := 0; i < 100; i++ {
		longID += "abcdefgh"
	}

	info := &SessionInfo{
		SessionID:  longID,
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  time.Now(),
	}

	err := store.Save(info)
	if err != nil {
		t.Fatalf("failed to save session with long ID: %v", err)
	}

	loaded, err := store.Load(longID)
	if err != nil {
		t.Fatalf("failed to load session with long ID: %v", err)
	}
	if loaded.SessionID != longID {
		t.Error("session ID was modified")
	}
}

func TestSessionStore_SaveUpdatesLastUsed(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSessionStore(dir)

	past := time.Now().Add(-24 * time.Hour)
	info := &SessionInfo{
		SessionID:  "session-123",
		WorkerID:   "worker-1",
		WorkerName: "paulie",
		CreatedAt:  past,
		LastUsed:   past,
	}

	err := store.Save(info)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Save should update LastUsed to now
	loaded, _ := store.Load("session-123")
	if !loaded.LastUsed.After(past) {
		t.Error("expected LastUsed to be updated on save")
	}
}
