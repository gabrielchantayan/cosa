// Package daemon implements the Cosa daemon with Unix socket IPC.
package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cosa/internal/claude"
	"cosa/internal/config"
	"cosa/internal/git"
	"cosa/internal/job"
	"cosa/internal/ledger"
	"cosa/internal/protocol"
	"cosa/internal/review"
	"cosa/internal/territory"
	"cosa/internal/worker"
)

// Server is the Cosa daemon server.
type Server struct {
	cfg       *config.Config
	ledger    *ledger.Ledger
	listener  net.Listener
	startedAt time.Time

	// Territory and workers
	territory         *territory.Territory
	pool              *worker.Pool
	jobs              *job.Store
	queue             *job.Queue
	operations        *job.OperationStore
	sessions          *claude.SessionStore
	scheduler         *scheduler
	reviewCoordinator *review.Coordinator

	// Background services
	lookout *worker.Lookout
	cleaner *worker.Cleaner

	// Chat session for interactive communication with underboss
	chatSession *ChatSession

	// Client subscriptions for real-time events
	clients   map[net.Conn]*clientState
	clientsMu sync.RWMutex

	// Shutdown handling
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// scheduler manages job-to-worker assignment.
type scheduler struct {
	ctx      context.Context
	cancel   context.CancelFunc
	pool     *worker.Pool
	queue    *job.Queue
	jobs     *job.Store
	ledger   *ledger.Ledger
	tickRate time.Duration
	server   *Server // Reference to server for territory access
}

type clientState struct {
	subscribed bool
	events     []string // event types subscribed to, empty = all
}

// New creates a new daemon server.
func New(cfg *config.Config) (*Server, error) {
	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open ledger
	l, err := ledger.Open(cfg.LedgerPath())
	if err != nil {
		return nil, fmt.Errorf("failed to open ledger: %w", err)
	}

	// Create session store
	sessionsPath := filepath.Join(cfg.DataDir, "sessions")
	sessions, err := claude.NewSessionStore(sessionsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create persistent job store
	jobsPath := filepath.Join(cfg.DataDir, "jobs")
	jobs, err := job.NewPersistentStore(jobsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create job store: %w", err)
	}

	// Create persistent worker pool
	workersPath := filepath.Join(cfg.DataDir, "workers")
	pool, err := worker.NewPersistentPool(workersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}

	queue := job.NewQueue(jobs)
	operations := job.NewOperationStore()

	return &Server{
		cfg:        cfg,
		ledger:     l,
		clients:    make(map[net.Conn]*clientState),
		pool:       pool,
		jobs:       jobs,
		queue:      queue,
		operations: operations,
		sessions:   sessions,
		ctx:        ctx,
		cancel:     cancel,
		startedAt:  time.Now(),
	}, nil
}

// Start begins listening on the Unix socket.
func (s *Server) Start() error {
	// Remove stale socket
	os.Remove(s.cfg.SocketPath)

	listener, err := net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Write PID file
	if err := os.WriteFile(s.cfg.PIDPath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Log daemon start
	s.ledger.Append(ledger.EventDaemonStarted, ledger.DaemonEventData{
		Version: config.Version,
		PID:     os.Getpid(),
	})

	// Try to auto-load territory from current directory
	if wd, err := os.Getwd(); err == nil {
		if territory.Exists(wd) {
			s.loadExistingTerritory(wd)
		}
	}

	// Restore workers from persistence
	s.restoreWorkers()

	// Re-queue pending/queued jobs
	s.requeueJobs()

	// Start the scheduler
	s.startScheduler()

	// Start background services
	s.startLookout()
	s.startCleaner()

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	// Forward ledger events to subscribed clients
	eventCh := make(chan ledger.Event, 100)
	s.ledger.Subscribe(eventCh)
	s.wg.Add(1)
	go s.eventForwarder(eventCh)

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Stop the scheduler
	s.stopScheduler()

	// Stop background services
	s.stopLookout()
	s.stopCleaner()

	// Stop chat session if active
	s.mu.Lock()
	if s.chatSession != nil {
		s.chatSession.Stop()
		s.chatSession = nil
	}
	s.mu.Unlock()

	// Save active sessions and stop all workers
	for _, w := range s.pool.List() {
		if w.SessionID != "" {
			s.sessions.Save(&claude.SessionInfo{
				SessionID:  w.SessionID,
				WorkerID:   w.ID,
				WorkerName: w.Name,
				CreatedAt:  w.CreatedAt,
				LastUsed:   time.Now(),
			})
		}
		w.Stop()
	}

	// Close all client connections
	s.clientsMu.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clientsMu.Unlock()

	s.wg.Wait()

	// Log daemon stop
	s.ledger.Append(ledger.EventDaemonStopped, ledger.DaemonEventData{
		Version: config.Version,
		PID:     os.Getpid(),
	})

	s.ledger.Close()

	// Clean up socket and PID file
	os.Remove(s.cfg.SocketPath)
	os.Remove(s.cfg.PIDPath())

	return nil
}

// Wait blocks until the server is stopped.
func (s *Server) Wait() {
	<-s.ctx.Done()
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				continue
			}
		}

		s.clientsMu.Lock()
		s.clients[conn] = &clientState{}
		s.clientsMu.Unlock()

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		var req protocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			s.sendError(conn, nil, protocol.ParseError, "Parse error")
			continue
		}

		resp := s.handleRequest(&req, conn)
		if resp != nil {
			s.sendResponse(conn, resp)
		}
	}
}

func (s *Server) handleRequest(req *protocol.Request, conn net.Conn) *protocol.Response {
	switch req.Method {
	case protocol.MethodStatus:
		return s.handleStatus(req)
	case protocol.MethodShutdown:
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.cancel()
		}()
		resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "shutting_down"})
		return resp
	case protocol.MethodSubscribe:
		return s.handleSubscribe(req, conn)
	case protocol.MethodUnsubscribe:
		return s.handleUnsubscribe(req, conn)
	case protocol.MethodTerritoryInit:
		return s.handleTerritoryInit(req)
	case protocol.MethodTerritoryStatus:
		return s.handleTerritoryStatus(req)
	case protocol.MethodTerritoryList:
		return s.handleTerritoryList(req)
	case protocol.MethodTerritoryAdd:
		return s.handleTerritoryAdd(req)
	case protocol.MethodTerritorySetDevBranch:
		return s.handleTerritorySetDevBranch(req)
	case protocol.MethodWorkerAdd:
		return s.handleWorkerAdd(req)
	case protocol.MethodWorkerList:
		return s.handleWorkerList(req)
	case protocol.MethodWorkerStatus:
		return s.handleWorkerStatus(req)
	case protocol.MethodWorkerRemove:
		return s.handleWorkerRemove(req)
	case protocol.MethodWorkerDetail:
		return s.handleWorkerDetail(req)
	case protocol.MethodWorkerMessage:
		return s.handleWorkerMessage(req)
	case protocol.MethodJobAdd:
		return s.handleJobAdd(req)
	case protocol.MethodJobList:
		return s.handleJobList(req)
	case protocol.MethodJobCancel:
		return s.handleJobCancel(req)
	case protocol.MethodJobStatus:
		return s.handleJobStatus(req)
	case protocol.MethodJobAssign:
		return s.handleJobAssign(req)
	case protocol.MethodJobSetPriority:
		return s.handleJobSetPriority(req)
	case protocol.MethodQueueStatus:
		return s.handleQueueStatus(req)
	case protocol.MethodReviewStart:
		return s.handleReviewStart(req)
	case protocol.MethodReviewStatus:
		return s.handleReviewStatus(req)
	case protocol.MethodReviewList:
		return s.handleReviewList(req)
	case protocol.MethodOperationCreate:
		return s.handleOperationCreate(req)
	case protocol.MethodOperationStatus:
		return s.handleOperationStatus(req)
	case protocol.MethodOperationList:
		return s.handleOperationList(req)
	case protocol.MethodOperationCancel:
		return s.handleOperationCancel(req)
	case protocol.MethodOrderSet:
		return s.handleOrderSet(req)
	case protocol.MethodOrderList:
		return s.handleOrderList(req)
	case protocol.MethodOrderClear:
		return s.handleOrderClear(req)
	case protocol.MethodHandoffGenerate:
		return s.handleHandoffGenerate(req)
	case protocol.MethodChatStart:
		return s.handleChatStart(req)
	case protocol.MethodChatSend:
		return s.handleChatSend(req)
	case protocol.MethodChatEnd:
		return s.handleChatEnd(req)
	case protocol.MethodChatHistory:
		return s.handleChatHistory(req)
	default:
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.MethodNotFound, "Method not found", nil)
		return resp
	}
}

func (s *Server) handleStatus(req *protocol.Request) *protocol.Response {
	uptime := int64(time.Since(s.startedAt).Seconds())

	workerCount := s.pool.Count()
	activeJobs := s.jobs.CountByStatus(job.StatusRunning)

	// Calculate total cost across all workers
	var totalTokens int
	totalCost := "$0.00"
	for _, w := range s.pool.List() {
		totalTokens += w.TotalTokens
		if w.TotalCost != "" {
			// Note: For proper cost aggregation, we'd need to parse and sum costs
			// For now, just show the last worker's cost as the total
			totalCost = w.TotalCost
		}
	}

	result := protocol.StatusResult{
		Running:     true,
		Version:     config.Version,
		Uptime:      uptime,
		Workers:     workerCount,
		ActiveJobs:  activeJobs,
		TotalCost:   totalCost,
		TotalTokens: totalTokens,
	}

	s.mu.RLock()
	if s.territory != nil {
		result.Territory = s.territory.Path
	}
	s.mu.RUnlock()

	resp, _ := protocol.NewResponse(req.ID, result)
	return resp
}

func (s *Server) handleSubscribe(req *protocol.Request, conn net.Conn) *protocol.Response {
	var params protocol.SubscribeParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	s.clientsMu.Lock()
	if state, ok := s.clients[conn]; ok {
		state.subscribed = true
		state.events = params.Events
	}
	s.clientsMu.Unlock()

	resp, _ := protocol.NewResponse(req.ID, map[string]bool{"subscribed": true})
	return resp
}

func (s *Server) handleUnsubscribe(req *protocol.Request, conn net.Conn) *protocol.Response {
	s.clientsMu.Lock()
	if state, ok := s.clients[conn]; ok {
		state.subscribed = false
		state.events = nil
	}
	s.clientsMu.Unlock()

	resp, _ := protocol.NewResponse(req.ID, map[string]bool{"subscribed": false})
	return resp
}

func (s *Server) eventForwarder(eventCh <-chan ledger.Event) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-eventCh:
			s.broadcastEvent(event)
		}
	}
}

func (s *Server) broadcastEvent(event ledger.Event) {
	notification, err := protocol.NewNotification(protocol.NotifyLogEntry, event)
	if err != nil {
		return
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for conn, state := range s.clients {
		if !state.subscribed {
			continue
		}

		// Check if client is subscribed to this event type
		if len(state.events) > 0 && state.events[0] != "*" {
			found := false
			for _, e := range state.events {
				if e == string(event.Type) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		conn.Write(data)
	}
}

func (s *Server) sendResponse(conn net.Conn, resp *protocol.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	conn.Write(append(data, '\n'))
}

func (s *Server) sendError(conn net.Conn, id *protocol.RequestID, code int, message string) {
	resp, _ := protocol.NewErrorResponse(id, code, message, nil)
	s.sendResponse(conn, resp)
}

// startScheduler initializes and starts the job scheduler.
func (s *Server) startScheduler() {
	ctx, cancel := context.WithCancel(s.ctx)

	s.scheduler = &scheduler{
		ctx:      ctx,
		cancel:   cancel,
		pool:     s.pool,
		queue:    s.queue,
		jobs:     s.jobs,
		ledger:   s.ledger,
		tickRate: 100 * time.Millisecond,
		server:   s,
	}

	s.wg.Add(1)
	go s.scheduler.run(&s.wg)
}

// stopScheduler stops the job scheduler.
func (s *Server) stopScheduler() {
	if s.scheduler != nil {
		s.scheduler.cancel()
	}
}

// run is the main scheduler loop.
func (sched *scheduler) run(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(sched.tickRate)
	defer ticker.Stop()

	for {
		select {
		case <-sched.ctx.Done():
			return
		case <-ticker.C:
			sched.processQueue()
		}
	}
}

// processQueue assigns ready jobs to available workers.
func (sched *scheduler) processQueue() {
	ready := sched.queue.GetReady()
	for _, j := range ready {
		w := sched.pool.FindBestWorker(j)
		if w == nil {
			continue // No available worker
		}

		// Remove from queue and mark as queued
		sched.queue.Remove(j.ID)
		j.Queue()
		sched.jobs.Save(j) // Persist queued state

		sched.ledger.Append(ledger.EventJobQueued, ledger.JobEventData{
			ID:          j.ID,
			Description: j.Description,
			Worker:      w.ID,
			WorkerName:  w.Name,
		})

		// Execute in goroutine using the consolidated helper
		go sched.server.executeJobWithWorktree(w, j)
	}
}

// onJobComplete is called when a job completes successfully.
func (s *Server) onJobComplete(j *job.Job) {
	s.queue.NotifyCompletion(j.ID)
	s.jobs.Save(j) // Persist final state

	// Get worker name for logging
	var workerName string
	if w, exists := s.pool.GetByID(j.Worker); exists {
		workerName = w.Name
	}

	s.ledger.Append(ledger.EventJobCompleted, ledger.JobEventData{
		ID:          j.ID,
		Description: j.Description,
		Worker:      j.Worker,
		WorkerName:  workerName,
	})

	// Merge the job's worktree into the target branch and cleanup
	if err := s.mergeAndCleanupJobWorktree(j); err != nil {
		s.ledger.Append(ledger.EventType("job.post_complete_error"), ledger.JobEventData{
			ID:    j.ID,
			Error: fmt.Sprintf("post-completion merge failed: %v", err),
		})
	}

	// Trigger auto-review if enabled
	s.mu.RLock()
	t := s.territory
	coord := s.reviewCoordinator
	s.mu.RUnlock()

	if t != nil && t.Config.AutoReview && coord != nil {
		w, exists := s.pool.GetByID(j.Worker)
		if exists {
			go coord.StartReview(s.ctx, j, w)
		}
	}
}

// onJobFail is called when a job fails.
func (s *Server) onJobFail(j *job.Job, err error) {
	s.queue.NotifyFailure(j.ID)
	s.jobs.Save(j) // Persist final state

	// Get worker name for logging
	var workerName string
	if w, exists := s.pool.GetByID(j.Worker); exists {
		workerName = w.Name
	}

	s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
		ID:          j.ID,
		Description: j.Description,
		Worker:      j.Worker,
		WorkerName:  workerName,
		Error:       err.Error(),
	})
}

// initReviewCoordinator initializes the review coordinator for the current territory.
func (s *Server) initReviewCoordinator() {
	if s.territory == nil {
		return
	}

	s.reviewCoordinator = review.NewCoordinator(review.CoordinatorConfig{
		GitManager: s.territory.GitManager(),
		JobStore:   s.jobs,
		JobQueue:   s.queue,
		Ledger:     s.ledger,
		ClaudeConfig: review.ConsigliereConfig{
			Binary:   s.cfg.Claude.Binary,
			Model:    s.cfg.Claude.Model,
			MaxTurns: 10,
		},
		GateConfig: review.GateRunnerConfig{
			TestCommand:  s.territory.Config.TestCommand,
			BuildCommand: s.territory.Config.BuildCommand,
		},
		BaseBranch: s.territory.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch),
	})
}

// restoreWorkers reinitializes workers loaded from persistence.
func (s *Server) restoreWorkers() {
	pending := s.pool.PendingWorkers()
	if len(pending) == 0 {
		return
	}

	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	for _, info := range pending {
		// Verify worktree still exists
		if _, err := os.Stat(info.Worktree); os.IsNotExist(err) {
			continue // Skip workers with missing worktrees
		}

		// Create worktree reference
		var wt *git.Worktree
		if t != nil {
			wt = &git.Worktree{
				Path:   info.Worktree,
				Branch: info.Branch,
			}
		}

		// Create worker with completion callbacks
		w := worker.New(worker.Config{
			Name:     info.Name,
			Role:     info.Role,
			Worktree: wt,
			ClaudeConfig: claude.ClientConfig{
				Binary:   s.cfg.Claude.Binary,
				Model:    s.cfg.Claude.Model,
				MaxTurns: s.cfg.Claude.MaxTurns,
			},
			OnEvent: func(e worker.Event) {
				s.ledger.Append(ledger.EventType("worker."+e.Type), e)
			},
			OnJobComplete: s.onJobComplete,
			OnJobFail:     s.onJobFail,
		})

		// Restore persisted state
		w.ID = info.ID
		w.SessionID = info.SessionID
		w.StandingOrders = info.StandingOrders
		w.JobsCompleted = info.JobsCompleted
		w.JobsFailed = info.JobsFailed

		// Add to pool and start
		if err := s.pool.Add(w); err != nil {
			continue // Skip if already exists or other error
		}
		w.Start()
	}

	s.pool.ClearPending()
}

// requeueJobs re-queues jobs that were pending or queued when daemon stopped.
func (s *Server) requeueJobs() {
	for _, j := range s.jobs.List() {
		status := j.GetStatus()
		switch status {
		case job.StatusPending, job.StatusQueued:
			// Re-queue for execution
			s.queue.Enqueue(j)
		case job.StatusRunning:
			// Job was interrupted - mark as failed and re-queue
			j.Fail("daemon restarted during execution")
			s.jobs.Save(j)
		}
	}
}

// startLookout initializes and starts the health monitor.
func (s *Server) startLookout() {
	s.lookout = worker.NewLookout(worker.LookoutConfig{
		Pool:   s.pool,
		Ledger: s.ledger,
		OnStuck: func(w *worker.Worker, severity worker.StuckSeverity) {
			// Log the stuck worker event
			s.ledger.Append(ledger.EventWorkerError, ledger.WorkerEventData{
				ID:    w.ID,
				Name:  w.Name,
				Role:  string(w.Role),
				Error: fmt.Sprintf("worker stuck: severity=%s", severity),
			})
		},
	})
	s.lookout.Start(s.ctx)
}

// stopLookout stops the health monitor.
func (s *Server) stopLookout() {
	if s.lookout != nil {
		s.lookout.Stop()
	}
}

// startCleaner initializes and starts the resource cleanup service.
func (s *Server) startCleaner() {
	var gitManager *git.Manager
	s.mu.RLock()
	if s.territory != nil {
		gitManager = s.territory.GitManager()
	}
	s.mu.RUnlock()

	s.cleaner = worker.NewCleaner(worker.CleanerConfig{
		Pool:         s.pool,
		GitManager:   gitManager,
		SessionStore: s.sessions,
		Ledger:       s.ledger,
	})
	s.cleaner.Start(s.ctx)
}

// stopCleaner stops the resource cleanup service.
func (s *Server) stopCleaner() {
	if s.cleaner != nil {
		s.cleaner.Stop()
	}
}

// createJobWorktree creates a dedicated worktree for a job.
func (s *Server) createJobWorktree(j *job.Job) error {
	if j == nil {
		return fmt.Errorf("job is nil")
	}

	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	if t == nil {
		return fmt.Errorf("territory not initialized")
	}

	gitMgr := t.GitManager()
	baseBranch := t.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch)

	wt, err := gitMgr.CreateJobWorktree(j.ID, baseBranch)
	if err != nil {
		return err
	}

	j.SetWorktree(wt.Path, wt.Branch)
	s.jobs.Save(j)

	return nil
}

// executeJobWithWorktree handles the full job execution lifecycle:
// creates worktree, logs events, executes job, and handles failures.
// This consolidates the duplicated logic from handlers and scheduler.
func (s *Server) executeJobWithWorktree(w *worker.Worker, j *job.Job) {
	// Create job worktree before starting
	if err := s.createJobWorktree(j); err != nil {
		s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
			ID:          j.ID,
			Description: j.Description,
			Worker:      w.ID,
			WorkerName:  w.Name,
			Error:       fmt.Sprintf("failed to create job worktree: %v", err),
		})
		j.Fail(fmt.Sprintf("failed to create job worktree: %v", err))
		s.jobs.Save(j)
		return
	}

	s.ledger.Append(ledger.EventJobStarted, ledger.JobEventData{
		ID:          j.ID,
		Description: j.Description,
		Worker:      w.ID,
		WorkerName:  w.Name,
	})

	if err := w.ExecuteInWorktree(j, j.GetWorktree()); err != nil {
		s.ledger.Append(ledger.EventJobFailed, ledger.JobEventData{
			ID:          j.ID,
			Description: j.Description,
			Worker:      w.ID,
			WorkerName:  w.Name,
			Error:       err.Error(),
		})
	}
}

// mergeAndCleanupJobWorktree merges the job's branch into the target branch and cleans up.
func (s *Server) mergeAndCleanupJobWorktree(j *job.Job) error {
	s.mu.RLock()
	t := s.territory
	s.mu.RUnlock()

	if t == nil {
		return fmt.Errorf("territory not initialized")
	}

	jobBranch := j.GetBranch()
	if jobBranch == "" {
		// No worktree was created for this job (legacy job)
		return nil
	}

	gitMgr := t.GitManager()
	targetBranch := t.MergeTargetBranch(s.cfg.Git.DefaultMergeBranch)

	// First, remove the worktree (must be done before merging to release the branch)
	if err := gitMgr.RemoveJobWorktree(j.ID, true); err != nil {
		s.ledger.Append(ledger.EventType("job.worktree_cleanup_error"), ledger.JobEventData{
			ID:    j.ID,
			Error: fmt.Sprintf("failed to remove worktree: %v", err),
		})
		// Continue with merge attempt anyway
	} else {
		// Clear worktree path (but keep branch name for merge tracking)
		j.SetWorktree("", jobBranch)
		s.jobs.Save(j)
	}

	// Merge the job branch into the target branch
	result, err := gitMgr.Merge(jobBranch, targetBranch)
	if err != nil {
		s.ledger.Append(ledger.EventType("job.merge_error"), ledger.JobEventData{
			ID:    j.ID,
			Error: fmt.Sprintf("failed to merge: %v", err),
		})
		return err
	}

	if !result.Success {
		s.ledger.Append(ledger.EventType("job.merge_conflict"), ledger.JobEventData{
			ID:    j.ID,
			Error: result.Message,
		})
		return fmt.Errorf("merge conflict: %s", result.Message)
	}

	s.ledger.Append(ledger.EventType("job.merged"), ledger.JobEventData{
		ID:          j.ID,
		Description: fmt.Sprintf("Merged into %s (commit: %s)", targetBranch, result.MergeCommit),
	})

	// Delete the job branch after successful merge
	if err := gitMgr.DeleteBranch(jobBranch, true); err != nil {
		s.ledger.Append(ledger.EventType("job.branch_cleanup_error"), ledger.JobEventData{
			ID:    j.ID,
			Error: fmt.Sprintf("failed to delete branch: %v", err),
		})
		// Non-fatal, continue
	}

	// Clear worktree info from job
	j.ClearWorktree()
	s.jobs.Save(j)

	return nil
}
