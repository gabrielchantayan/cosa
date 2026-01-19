package daemon

import (
	"context"
	"encoding/json"
	"fmt"
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
	ID        string
	sessionID string // Claude session ID for resuming
	cfg       claude.ClientConfig
	workdir   string
	messages  []protocol.ChatMessage
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// newChatSession creates a new chat session with the underboss.
func newChatSession(cfg claude.ClientConfig, workdir string) *ChatSession {
	ctx, cancel := context.WithCancel(context.Background())

	return &ChatSession{
		ID:       fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		cfg:      cfg,
		workdir:  workdir,
		messages: make([]protocol.ChatMessage, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start initiates the chat session with the first system prompt.
func (cs *ChatSession) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Send initial prompt to establish the session
	prompt := `You are the Underboss of the Cosa development organization. You manage the development workflow and help coordinate work.

You are now in an interactive chat session with a user. Respond conversationally and helpfully. Keep your responses concise but informative.

Say hello and let them know you're ready to chat.`

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

	// Create new session
	s.chatSession = newChatSession(claude.ClientConfig{
		Binary:   s.cfg.Claude.Binary,
		Model:    s.cfg.Claude.Model,
		MaxTurns: 1000,
	}, workdir)

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
