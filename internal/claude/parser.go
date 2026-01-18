package claude

import (
	"encoding/json"
)

// EventType identifies the type of Claude event.
type EventType string

const (
	EventInit          EventType = "init"
	EventSystemPrompt  EventType = "system_prompt"
	EventUserMessage   EventType = "user_message"
	EventAssistantText EventType = "assistant_text"
	EventToolUse       EventType = "tool_use"
	EventToolResult    EventType = "tool_result"
	EventResult        EventType = "result"
	EventError         EventType = "error"
)

// Event represents a parsed event from Claude's stream-json output.
type Event struct {
	Type      EventType `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Tool      *ToolCall `json:"tool,omitempty"`
	Result    *Result   `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ToolCall represents a tool invocation by Claude.
type ToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Status string          `json:"status"` // pending, running, completed, error
	Output string          `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Result represents the final result of a Claude session.
type Result struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	TotalCost   string `json:"total_cost,omitempty"`
	TotalTokens int    `json:"total_tokens,omitempty"`
	Duration    string `json:"duration,omitempty"`
}

// Parser parses Claude's stream-json output.
type Parser struct {
	// Track current state for multi-line parsing if needed
	currentTool *ToolCall
}

// NewParser creates a new stream-json parser.
func NewParser() *Parser {
	return &Parser{}
}

// streamMessage is the raw JSON structure from Claude's stream output.
type streamMessage struct {
	Type       string          `json:"type"`
	SessionID  string          `json:"session_id,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	ToolResult json.RawMessage `json:"tool_result,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ParseLine parses a single line of stream-json output.
func (p *Parser) ParseLine(line string) (*Event, error) {
	var msg streamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, err
	}

	switch msg.Type {
	case "init", "system":
		return &Event{
			Type:      EventInit,
			SessionID: msg.SessionID,
		}, nil

	case "user", "human":
		var text string
		json.Unmarshal(msg.Content, &text)
		if text == "" {
			json.Unmarshal(msg.Message, &text)
		}
		return &Event{
			Type:    EventUserMessage,
			Message: text,
		}, nil

	case "assistant", "text":
		var text string
		json.Unmarshal(msg.Content, &text)
		if text == "" {
			json.Unmarshal(msg.Message, &text)
		}
		return &Event{
			Type:    EventAssistantText,
			Message: text,
		}, nil

	case "tool_use", "tool_use_begin":
		tool := &ToolCall{
			ID:     msg.ToolUseID,
			Name:   msg.ToolName,
			Input:  msg.ToolInput,
			Status: "running",
		}
		p.currentTool = tool
		return &Event{
			Type: EventToolUse,
			Tool: tool,
		}, nil

	case "tool_result", "tool_use_end":
		tool := p.currentTool
		if tool == nil {
			tool = &ToolCall{
				ID: msg.ToolUseID,
			}
		}
		tool.Status = "completed"

		var output string
		json.Unmarshal(msg.ToolResult, &output)
		if output == "" {
			json.Unmarshal(msg.Content, &output)
		}
		tool.Output = output

		p.currentTool = nil
		return &Event{
			Type: EventToolResult,
			Tool: tool,
		}, nil

	case "result", "end":
		result := &Result{Success: true}
		if msg.Result != nil {
			json.Unmarshal(msg.Result, result)
		}
		return &Event{
			Type:   EventResult,
			Result: result,
		}, nil

	case "error":
		return &Event{
			Type:  EventError,
			Error: msg.Error,
		}, nil
	}

	// Unknown message type, return nil to skip
	return nil, nil
}
