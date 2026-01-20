package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"cosa/internal/config"
)

// Server is an MCP server that provides Cosa tools to Claude.
type Server struct {
	daemon   DaemonInterface
	registry *ToolRegistry
}

// NewServer creates a new MCP server.
func NewServer(daemon DaemonInterface) *Server {
	return &Server{
		daemon:   daemon,
		registry: NewToolRegistry(),
	}
}

// Serve runs the MCP server, reading from stdin and writing to stdout.
func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)

	// Increase buffer size for large requests
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp, _ := NewErrorResponse(nil, ParseError, "Parse error", nil)
			s.sendResponse(stdout, resp)
			continue
		}

		resp := s.handleRequest(&req)
		s.sendResponse(stdout, resp)
	}

	return scanner.Err()
}

func (s *Server) handleRequest(req *Request) *Response {
	switch req.Method {
	case MethodInitialize:
		return s.handleInitialize(req)
	case MethodToolsList:
		return s.handleToolsList(req)
	case MethodToolsCall:
		return s.handleToolsCall(req)
	default:
		resp, _ := NewErrorResponse(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
		return resp
	}
}

func (s *Server) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapability{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "cosa-mcp",
			Version: config.Version,
		},
	}

	resp, _ := NewResponse(req.ID, result)
	return resp
}

func (s *Server) handleToolsList(req *Request) *Response {
	result := ToolsListResult{
		Tools: s.registry.Tools(),
	}

	resp, _ := NewResponse(req.ID, result)
	return resp
}

func (s *Server) handleToolsCall(req *Request) *Response {
	var params CallToolParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp, _ := NewErrorResponse(req.ID, InvalidParams, "Invalid params", nil)
			return resp
		}
	}

	result, err := s.registry.Call(params.Name, params.Arguments, s.daemon)
	if err != nil {
		// Return the error result (tool errors are returned as successful responses with isError=true)
		resp, _ := NewResponse(req.ID, result)
		return resp
	}

	resp, _ := NewResponse(req.ID, result)
	return resp
}

func (s *Server) sendResponse(w io.Writer, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	w.Write(append(data, '\n'))
}
