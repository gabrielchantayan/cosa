package worker

import (
	"sync"
	"testing"

	"cosa/internal/job"
)

func TestNewPool(t *testing.T) {
	pool := NewPool()
	if pool == nil {
		t.Fatal("NewPool returned nil")
	}
	if pool.Count() != 0 {
		t.Errorf("expected empty pool, got %d workers", pool.Count())
	}
}

func TestPoolAdd(t *testing.T) {
	pool := NewPool()

	w := &Worker{
		ID:   "worker-1",
		Name: "paulie",
		Role: RoleSoldato,
	}

	err := pool.Add(w)
	if err != nil {
		t.Fatalf("failed to add worker: %v", err)
	}

	if pool.Count() != 1 {
		t.Errorf("expected 1 worker, got %d", pool.Count())
	}

	// Adding duplicate should fail
	err = pool.Add(w)
	if err == nil {
		t.Error("expected error when adding duplicate worker")
	}
}

func TestPoolRemove(t *testing.T) {
	pool := NewPool()

	w := &Worker{
		ID:   "worker-1",
		Name: "paulie",
		Role: RoleSoldato,
	}
	pool.Add(w)

	// Remove existing worker
	removed, err := pool.Remove("paulie")
	if err != nil {
		t.Fatalf("failed to remove worker: %v", err)
	}
	if removed.ID != w.ID {
		t.Error("removed wrong worker")
	}
	if pool.Count() != 0 {
		t.Errorf("expected 0 workers, got %d", pool.Count())
	}

	// Remove non-existent worker
	_, err = pool.Remove("nonexistent")
	if err == nil {
		t.Error("expected error when removing non-existent worker")
	}
}

func TestPoolGet(t *testing.T) {
	pool := NewPool()

	w := &Worker{
		ID:   "worker-1",
		Name: "paulie",
		Role: RoleSoldato,
	}
	pool.Add(w)

	// Get existing worker
	found, ok := pool.Get("paulie")
	if !ok {
		t.Fatal("worker not found")
	}
	if found.ID != w.ID {
		t.Error("got wrong worker")
	}

	// Get non-existent worker
	_, ok = pool.Get("nonexistent")
	if ok {
		t.Error("expected not found for non-existent worker")
	}
}

func TestPoolGetByID(t *testing.T) {
	pool := NewPool()

	w := &Worker{
		ID:   "worker-1",
		Name: "paulie",
		Role: RoleSoldato,
	}
	pool.Add(w)

	// Get existing worker by ID
	found, ok := pool.GetByID("worker-1")
	if !ok {
		t.Fatal("worker not found by ID")
	}
	if found.Name != w.Name {
		t.Error("got wrong worker")
	}

	// Get non-existent worker by ID
	_, ok = pool.GetByID("nonexistent-id")
	if ok {
		t.Error("expected not found for non-existent worker ID")
	}
}

func TestPoolList(t *testing.T) {
	pool := NewPool()

	workers := []*Worker{
		{ID: "1", Name: "paulie", Role: RoleSoldato},
		{ID: "2", Name: "silvio", Role: RoleSoldato},
		{ID: "3", Name: "tony", Role: RoleCapo},
	}

	for _, w := range workers {
		pool.Add(w)
	}

	list := pool.List()
	if len(list) != 3 {
		t.Errorf("expected 3 workers, got %d", len(list))
	}
}

func TestPoolListByRole(t *testing.T) {
	pool := NewPool()

	workers := []*Worker{
		{ID: "1", Name: "paulie", Role: RoleSoldato},
		{ID: "2", Name: "silvio", Role: RoleSoldato},
		{ID: "3", Name: "tony", Role: RoleCapo},
		{ID: "4", Name: "johnny", Role: RoleConsigliere},
	}

	for _, w := range workers {
		pool.Add(w)
	}

	soldatos := pool.ListByRole(RoleSoldato)
	if len(soldatos) != 2 {
		t.Errorf("expected 2 soldatos, got %d", len(soldatos))
	}

	capos := pool.ListByRole(RoleCapo)
	if len(capos) != 1 {
		t.Errorf("expected 1 capo, got %d", len(capos))
	}

	consiglieres := pool.ListByRole(RoleConsigliere)
	if len(consiglieres) != 1 {
		t.Errorf("expected 1 consigliere, got %d", len(consiglieres))
	}

	// Non-existent role should return empty slice
	cleaners := pool.ListByRole(RoleCleaner)
	if len(cleaners) != 0 {
		t.Errorf("expected 0 cleaners, got %d", len(cleaners))
	}
}

func TestPoolExists(t *testing.T) {
	pool := NewPool()

	w := &Worker{
		ID:   "worker-1",
		Name: "paulie",
		Role: RoleSoldato,
	}
	pool.Add(w)

	if !pool.Exists("paulie") {
		t.Error("expected worker to exist")
	}

	if pool.Exists("nonexistent") {
		t.Error("expected worker to not exist")
	}
}

func TestPoolGetAvailable(t *testing.T) {
	pool := NewPool()

	// Create workers with different statuses
	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato, Status: StatusIdle}
	w2 := &Worker{ID: "2", Name: "silvio", Role: RoleSoldato, Status: StatusWorking}
	w3 := &Worker{ID: "3", Name: "tony", Role: RoleCapo, Status: StatusIdle}

	pool.Add(w1)
	pool.Add(w2)
	pool.Add(w3)

	available := pool.GetAvailable()
	if len(available) != 2 {
		t.Errorf("expected 2 available workers, got %d", len(available))
	}
}

func TestPoolGetAvailableByRole(t *testing.T) {
	pool := NewPool()

	// Create workers with different statuses
	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato, Status: StatusIdle}
	w2 := &Worker{ID: "2", Name: "silvio", Role: RoleSoldato, Status: StatusWorking}
	w3 := &Worker{ID: "3", Name: "tony", Role: RoleCapo, Status: StatusIdle}

	pool.Add(w1)
	pool.Add(w2)
	pool.Add(w3)

	soldatos := pool.GetAvailableByRole(RoleSoldato)
	if len(soldatos) != 1 {
		t.Errorf("expected 1 available soldato, got %d", len(soldatos))
	}

	capos := pool.GetAvailableByRole(RoleCapo)
	if len(capos) != 1 {
		t.Errorf("expected 1 available capo, got %d", len(capos))
	}
}

func TestPoolFindBestWorker(t *testing.T) {
	pool := NewPool()

	// Create workers with different roles and job counts
	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato, Status: StatusIdle, JobsCompleted: 5}
	w2 := &Worker{ID: "2", Name: "silvio", Role: RoleSoldato, Status: StatusIdle, JobsCompleted: 2}
	w3 := &Worker{ID: "3", Name: "tony", Role: RoleCapo, Status: StatusIdle, JobsCompleted: 0}
	w4 := &Worker{ID: "4", Name: "johnny", Role: RoleConsigliere, Status: StatusIdle, JobsCompleted: 0}

	pool.Add(w1)
	pool.Add(w2)
	pool.Add(w3)
	pool.Add(w4)

	j := &job.Job{ID: "job-1", Description: "test"}

	best := pool.FindBestWorker(j)
	if best == nil {
		t.Fatal("expected to find a worker")
	}

	// Should prefer soldato with fewer jobs completed
	if best.Name != "silvio" {
		t.Errorf("expected silvio (soldato with fewer jobs), got %s", best.Name)
	}
}

func TestPoolFindBestWorkerNoAvailable(t *testing.T) {
	pool := NewPool()

	// All workers busy
	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato, Status: StatusWorking}
	w2 := &Worker{ID: "2", Name: "silvio", Role: RoleSoldato, Status: StatusWorking}

	pool.Add(w1)
	pool.Add(w2)

	j := &job.Job{ID: "job-1", Description: "test"}

	best := pool.FindBestWorker(j)
	if best != nil {
		t.Error("expected nil when no workers available")
	}
}

func TestPoolFindBestWorkerPrefersSoldato(t *testing.T) {
	pool := NewPool()

	// Capo with fewer jobs but soldato should still be preferred
	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato, Status: StatusIdle, JobsCompleted: 10}
	w2 := &Worker{ID: "2", Name: "tony", Role: RoleCapo, Status: StatusIdle, JobsCompleted: 0}

	pool.Add(w1)
	pool.Add(w2)

	j := &job.Job{ID: "job-1", Description: "test"}

	best := pool.FindBestWorker(j)
	if best == nil {
		t.Fatal("expected to find a worker")
	}

	// Should prefer soldato even with more jobs (due to +100 bonus)
	if best.Name != "paulie" {
		t.Errorf("expected paulie (soldato), got %s", best.Name)
	}
}

func TestPoolOnIdleCallback(t *testing.T) {
	pool := NewPool()

	var calledWith *Worker
	pool.SetOnIdle(func(w *Worker) {
		calledWith = w
	})

	w := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato}
	pool.Add(w)

	pool.NotifyIdle(w)

	if calledWith == nil {
		t.Error("callback was not called")
	}
	if calledWith.Name != "paulie" {
		t.Errorf("callback called with wrong worker: %s", calledWith.Name)
	}
}

func TestPoolConcurrentAccess(t *testing.T) {
	pool := NewPool()

	var wg sync.WaitGroup
	numWorkers := 100

	// Concurrent adds
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := &Worker{
				ID:   string(rune('0' + i)),
				Name: string(rune('a' + (i % 26))),
				Role: RoleSoldato,
			}
			pool.Add(w) // Some will fail due to name collision, that's OK
		}(i)
	}
	wg.Wait()

	// Concurrent reads shouldn't panic
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			pool.List()
		}()
		go func() {
			defer wg.Done()
			pool.GetAvailable()
		}()
		go func() {
			defer wg.Done()
			pool.Count()
		}()
	}
	wg.Wait()
}

func TestPoolRemoveUpdatesRoleIndex(t *testing.T) {
	pool := NewPool()

	w1 := &Worker{ID: "1", Name: "paulie", Role: RoleSoldato}
	w2 := &Worker{ID: "2", Name: "silvio", Role: RoleSoldato}
	w3 := &Worker{ID: "3", Name: "tony", Role: RoleCapo}

	pool.Add(w1)
	pool.Add(w2)
	pool.Add(w3)

	// Remove from middle of role slice
	pool.Remove("paulie")

	soldatos := pool.ListByRole(RoleSoldato)
	if len(soldatos) != 1 {
		t.Errorf("expected 1 soldato after removal, got %d", len(soldatos))
	}
	if soldatos[0].Name != "silvio" {
		t.Errorf("expected silvio to remain, got %s", soldatos[0].Name)
	}

	// Verify capos unaffected
	capos := pool.ListByRole(RoleCapo)
	if len(capos) != 1 {
		t.Errorf("expected 1 capo, got %d", len(capos))
	}
}
