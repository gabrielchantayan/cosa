package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cosa/internal/claude"
	"cosa/internal/ledger"
	"cosa/internal/protocol"
)

// ChatSession manages an interactive chat with the underboss (Claude).
// Each message spawns a new Claude process, but uses --resume to maintain context.
type ChatSession struct {
	ID            string
	sessionID     string // Claude session ID for resuming
	cfg           claude.ClientConfig
	workdir       string
	mcpConfigPath string // Path to MCP config file
	cosaBinary    string // Path to cosa binary for MCP server
	messages      []protocol.ChatMessage
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// newChatSession creates a new chat session with the underboss.
func newChatSession(cfg claude.ClientConfig, workdir string, cosaBinary string) *ChatSession {
	ctx, cancel := context.WithCancel(context.Background())

	return &ChatSession{
		ID:         fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		cfg:        cfg,
		workdir:    workdir,
		cosaBinary: cosaBinary,
		messages:   make([]protocol.ChatMessage, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// MCPConfig represents the structure of an MCP configuration file.
type MCPConfig struct {
	McpServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig represents a single MCP server configuration.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// createMCPConfig creates a temporary MCP config file and returns its path.
func (cs *ChatSession) createMCPConfig() (string, error) {
	// Create temp directory if it doesn't exist
	tmpDir := filepath.Join(os.TempDir(), "cosa-mcp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Determine cosa binary path
	cosaBinary := cs.cosaBinary
	if cosaBinary == "" {
		// Try to find cosa in PATH
		var err error
		cosaBinary, err = os.Executable()
		if err != nil {
			cosaBinary = "cosa"
		}
	}

	// Create MCP config
	config := MCPConfig{
		McpServers: map[string]MCPServerConfig{
			"cosa": {
				Command: cosaBinary,
				Args:    []string{"mcp-serve"},
			},
		},
	}

	// Write config to temp file
	configPath := filepath.Join(tmpDir, fmt.Sprintf("mcp-config-%s.json", cs.ID))
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write MCP config: %w", err)
	}

	return configPath, nil
}

// cleanupMCPConfig removes the temporary MCP config file.
func (cs *ChatSession) cleanupMCPConfig() {
	if cs.mcpConfigPath != "" {
		os.Remove(cs.mcpConfigPath)
	}
}

// Start initiates the chat session with the first system prompt.
func (cs *ChatSession) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Create MCP config for Claude to connect to Cosa tools
	mcpConfigPath, err := cs.createMCPConfig()
	if err != nil {
		// Log warning but continue without MCP - chat will still work, just without tools
		fmt.Printf("Warning: Could not create MCP config: %v\n", err)
	} else {
		cs.mcpConfigPath = mcpConfigPath
		cs.cfg.MCPConfig = mcpConfigPath
	}

	// Send initial prompt to establish the session with mafia character
	prompt := `You are The Underboss of the Cosa development organization. You oversee all the soldati (workers) and manage the family's development operations.

Your character:
- You speak with a classic mafia underboss persona, using expressions like "capisce?", "fuggedaboutit", "the family", "our thing", "make 'em an offer they can't refuse"
- You're respectful but firm, always looking out for the family's interests
- You call workers "soldati" or by their names, jobs are "contracts" or "hits"
- Keep responses conversational and in character, but still helpful and informative
- Don't overdo it - a light touch of flavor, not a parody

Your responsibilities:
- You oversee the soldati (workers) and their assignments
- You manage the job queue and priorities
- You can check on worker status, job progress, costs, and operations
- You can create new jobs, cancel jobs, and adjust priorities
- You keep the boss (user) informed about what's happening

You have MCP tools available to interact with the Cosa system:
- cosa_list_workers: See all our soldati
- cosa_get_worker: Get details on a specific soldato
- cosa_list_jobs: Check on all the contracts
- cosa_get_job: Get details on a specific contract
- cosa_create_job: Put out a new contract
- cosa_cancel_job: Call off a contract
- cosa_set_job_priority: Change contract priority
- cosa_queue_status: Check the queue
- cosa_list_activity: See recent activity
- cosa_list_territories: Check our territories
- cosa_list_operations: Check ongoing operations
- cosa_get_costs: See what we're spending

Use these tools proactively when the user asks about workers, jobs, status, or operations.

Say hello to the boss and let them know you're ready to discuss family business.`

	response, sessionID, err := cs.sendMessage(prompt, "")
	if err != nil {
		return err
	}

	cs.sessionID = sessionID
	cs.recordMessageLocked("assistant", response)

	return nil
}

// Send sends a message and returns the response.
func (cs *ChatSession) Send(message string) (string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.sessionID == "" {
		return "", fmt.Errorf("chat session not started")
	}

	// Record user message
	cs.recordMessageLocked("user", message)

	// Send to Claude with resume
	response, newSessionID, err := cs.sendMessage(message, cs.sessionID)
	if err != nil {
		return "", err
	}

	// Update session ID if it changed
	if newSessionID != "" {
		cs.sessionID = newSessionID
	}

	cs.recordMessageLocked("assistant", response)
	return response, nil
}

// sendMessage sends a message and waits for the complete response.
func (cs *ChatSession) sendMessage(prompt, resumeSessionID string) (string, string, error) {
	clientCfg := cs.cfg
	clientCfg.Workdir = cs.workdir
	clientCfg.MaxTurns = 50

	client := claude.NewClient(clientCfg)

	var err error
	if resumeSessionID != "" {
		err = client.Resume(cs.ctx, resumeSessionID, prompt)
	} else {
		err = client.Start(cs.ctx, prompt)
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Collect response
	var responseBuilder strings.Builder
	var sessionID string

	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-cs.ctx.Done():
			client.Stop()
			return "", "", fmt.Errorf("chat session cancelled")

		case <-timeout:
			client.Stop()
			return "", "", fmt.Errorf("response timeout")

		case <-client.Done():
			// Process completed
			return responseBuilder.String(), sessionID, nil

		case event, ok := <-client.Events():
			if !ok {
				return responseBuilder.String(), sessionID, nil
			}

			switch event.Type {
			case claude.EventInit:
				if event.SessionID != "" {
					sessionID = event.SessionID
				}

			case claude.EventAssistantText:
				responseBuilder.WriteString(event.Message)

			case claude.EventToolUse:
				if event.Tool != nil {
					responseBuilder.WriteString(fmt.Sprintf("\n[Using tool: %s]\n", event.Tool.Name))
				}

			case claude.EventError:
				if event.Error != "" {
					return "", "", fmt.Errorf("claude error: %s", event.Error)
				}
			}
		}
	}
}

// Stop ends the chat session.
func (cs *ChatSession) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.cancel()
	cs.cleanupMCPConfig()
}

// History returns the chat history.
func (cs *ChatSession) History() []protocol.ChatMessage {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	result := make([]protocol.ChatMessage, len(cs.messages))
	copy(result, cs.messages)
	return result
}

// GetGreeting returns the initial greeting if available.
func (cs *ChatSession) GetGreeting() string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if len(cs.messages) > 0 && cs.messages[0].Role == "assistant" {
		return cs.messages[0].Content
	}
	return ""
}

func (cs *ChatSession) recordMessageLocked(role, content string) {
	cs.messages = append(cs.messages, protocol.ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Unix(),
	})
}

// Chat handlers for the server

func (s *Server) handleChatStart(req *protocol.Request) *protocol.Response {
	var params protocol.ChatStartParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// End existing session if any
	if s.chatSession != nil {
		s.chatSession.Stop()
	}

	// Get working directory
	workdir := ""
	if s.territory != nil {
		workdir = s.territory.RepoRoot
	}

	// Get cosa binary path for MCP server
	cosaBinary, _ := os.Executable()

	// Create new session
	s.chatSession = newChatSession(claude.ClientConfig{
		Binary:   s.cfg.Claude.Binary,
		Model:    s.cfg.Claude.Model,
		MaxTurns: 1000,
	}, workdir, cosaBinary)

	if err := s.chatSession.Start(); err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	s.ledger.Append(ledger.EventType("chat.started"), map[string]string{
		"session_id": s.chatSession.ID,
	})

	resp, _ := protocol.NewResponse(req.ID, protocol.ChatStartResult{
		SessionID: s.chatSession.ID,
		Status:    "started",
		Greeting:  s.chatSession.GetGreeting(),
	})
	return resp
}

func (s *Server) handleChatSend(req *protocol.Request) *protocol.Response {
	var params protocol.ChatSendParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Message == "" {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InvalidParams, "message is required", nil)
		return resp
	}

	s.mu.RLock()
	session := s.chatSession
	s.mu.RUnlock()

	if session == nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.ErrInvalidState, "no active chat session", nil)
		return resp
	}

	response, err := session.Send(params.Message)
	if err != nil {
		resp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
		return resp
	}

	s.ledger.Append(ledger.EventType("chat.message"), map[string]interface{}{
		"session_id": session.ID,
		"user":       params.Message,
		"assistant":  response,
	})

	resp, _ := protocol.NewResponse(req.ID, protocol.ChatSendResult{
		Response: response,
	})
	return resp
}

func (s *Server) handleChatEnd(req *protocol.Request) *protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.chatSession == nil {
		resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "no_session"})
		return resp
	}

	sessionID := s.chatSession.ID
	s.chatSession.Stop()
	s.chatSession = nil

	s.ledger.Append(ledger.EventType("chat.ended"), map[string]string{
		"session_id": sessionID,
	})

	resp, _ := protocol.NewResponse(req.ID, map[string]string{"status": "ended"})
	return resp
}

func (s *Server) handleChatHistory(req *protocol.Request) *protocol.Response {
	s.mu.RLock()
	session := s.chatSession
	s.mu.RUnlock()

	if session == nil {
		resp, _ := protocol.NewResponse(req.ID, protocol.ChatHistoryResult{
			Messages: []protocol.ChatMessage{},
		})
		return resp
	}

	resp, _ := protocol.NewResponse(req.ID, protocol.ChatHistoryResult{
		Messages: session.History(),
	})
	return resp
}
