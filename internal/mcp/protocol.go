// Package mcp implements the Model Context Protocol (MCP) server for Cosa.
// This allows Claude to interact with Cosa's daemon through tool calls.
package mcp

import (
	"encoding/json"
)

// JSON-RPC 2.0 constants
const JSONRPCVersion = "2.0"

// Request represents an MCP JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// RequestID can be a string or integer.
type RequestID struct {
	Str *string
	Num *int64
}

func (id *RequestID) MarshalJSON() ([]byte, error) {
	if id == nil {
		return []byte("null"), nil
	}
	if id.Str != nil {
		return json.Marshal(*id.Str)
	}
	if id.Num != nil {
		return json.Marshal(*id.Num)
	}
	return []byte("null"), nil
}

func (id *RequestID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.Str = &s
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		id.Num = &n
		return nil
	}
	return nil
}

// NewIntID creates a RequestID from an integer.
func NewIntID(n int64) *RequestID {
	return &RequestID{Num: &n}
}

// Error represents an MCP JSON-RPC error.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP method constants
const (
	MethodInitialize = "initialize"
	MethodToolsList  = "tools/list"
	MethodToolsCall  = "tools/call"
)

// InitializeParams represents the parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ClientCapability `json:"capabilities"`
	ClientInfo      ClientInfo       `json:"clientInfo"`
}

// ClientCapability represents client capabilities.
type ClientCapability struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// RootsCapability represents root URI capabilities.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling capabilities.
type SamplingCapability struct{}

// ClientInfo represents information about the client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the result of an initialize request.
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ServerCapability `json:"capabilities"`
	ServerInfo      ServerInfo       `json:"serverInfo"`
}

// ServerCapability represents server capabilities.
type ServerCapability struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability represents tools capability.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo represents information about the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents the JSON schema for tool input.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a JSON schema property.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// ToolsListResult represents the result of a tools/list request.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents the parameters for a tools/call request.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult represents the result of a tools/call request.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewResponse creates a successful JSON-RPC response.
func NewResponse(id *RequestID, result interface{}) (*Response, error) {
	var rawResult json.RawMessage
	if result != nil {
		r, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		rawResult = r
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  rawResult,
	}, nil
}

// NewErrorResponse creates an error JSON-RPC response.
func NewErrorResponse(id *RequestID, code int, message string, data interface{}) (*Response, error) {
	var rawData json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = d
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    rawData,
		},
	}, nil
}

// TextContent creates a text content block.
func TextContent(text string) ContentBlock {
	return ContentBlock{
		Type: "text",
		Text: text,
	}
}

// ToolSuccess creates a successful tool result.
func ToolSuccess(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentBlock{TextContent(text)},
	}
}

// ToolError creates an error tool result.
func ToolError(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentBlock{TextContent(text)},
		IsError: true,
	}
}
