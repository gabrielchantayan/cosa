package protocol

import (
	"encoding/json"
	"testing"
)

func TestRequestID_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		id       *RequestID
		expected string
	}{
		{
			name:     "nil ID",
			id:       nil,
			expected: "null",
		},
		{
			name:     "string ID",
			id:       NewStringID("test-123"),
			expected: `"test-123"`,
		},
		{
			name:     "integer ID",
			id:       NewIntID(42),
			expected: "42",
		},
		{
			name:     "empty struct",
			id:       &RequestID{},
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.id.MarshalJSON()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(result))
			}
		})
	}
}

func TestRequestID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectStr  *string
		expectNum  *int64
	}{
		{
			name:      "string ID",
			input:     `"request-456"`,
			expectStr: strPtr("request-456"),
			expectNum: nil,
		},
		{
			name:      "integer ID",
			input:     "123",
			expectStr: nil,
			expectNum: int64Ptr(123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id RequestID
			err := json.Unmarshal([]byte(tt.input), &id)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectStr != nil {
				if id.Str == nil || *id.Str != *tt.expectStr {
					t.Errorf("expected string %s, got %v", *tt.expectStr, id.Str)
				}
			}
			if tt.expectNum != nil {
				if id.Num == nil || *id.Num != *tt.expectNum {
					t.Errorf("expected num %d, got %v", *tt.expectNum, id.Num)
				}
			}
		})
	}
}

func TestNewStringID(t *testing.T) {
	id := NewStringID("my-id")
	if id.Str == nil || *id.Str != "my-id" {
		t.Errorf("expected string ID 'my-id', got %v", id.Str)
	}
	if id.Num != nil {
		t.Errorf("expected nil Num, got %v", id.Num)
	}
}

func TestNewIntID(t *testing.T) {
	id := NewIntID(999)
	if id.Num == nil || *id.Num != 999 {
		t.Errorf("expected int ID 999, got %v", id.Num)
	}
	if id.Str != nil {
		t.Errorf("expected nil Str, got %v", id.Str)
	}
}

func TestNewRequest(t *testing.T) {
	type testParams struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name        string
		id          *RequestID
		method      string
		params      interface{}
		expectError bool
	}{
		{
			name:        "simple request without params",
			id:          NewStringID("req-1"),
			method:      MethodStatus,
			params:      nil,
			expectError: false,
		},
		{
			name:   "request with params",
			id:     NewIntID(1),
			method: MethodWorkerAdd,
			params: testParams{Name: "test-worker"},
		},
		{
			name:   "notification (nil ID)",
			id:     nil,
			method: MethodJobList,
			params: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewRequest(tt.id, tt.method, tt.params)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if req.JSONRPC != JSONRPCVersion {
				t.Errorf("expected JSONRPC %s, got %s", JSONRPCVersion, req.JSONRPC)
			}
			if req.Method != tt.method {
				t.Errorf("expected method %s, got %s", tt.method, req.Method)
			}
		})
	}
}

func TestNewNotification(t *testing.T) {
	req, err := NewNotification(MethodSubscribe, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ID != nil {
		t.Errorf("notification should have nil ID, got %v", req.ID)
	}
	if req.Method != MethodSubscribe {
		t.Errorf("expected method %s, got %s", MethodSubscribe, req.Method)
	}
}

func TestNewResponse(t *testing.T) {
	type testResult struct {
		Status string `json:"status"`
	}

	id := NewStringID("resp-1")
	resp, err := NewResponse(id, testResult{Status: "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("expected JSONRPC %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.ID == nil {
		t.Error("expected non-nil ID")
	}
	if resp.Error != nil {
		t.Errorf("expected nil error, got %v", resp.Error)
	}

	// Check result can be unmarshaled
	var result testResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
}

func TestNewErrorResponse(t *testing.T) {
	id := NewIntID(5)
	resp, err := NewErrorResponse(id, ErrWorkerNotFound, "worker not found", map[string]string{"id": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("expected JSONRPC %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.Result != nil {
		t.Errorf("expected nil result, got %v", resp.Result)
	}
	if resp.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if resp.Error.Code != ErrWorkerNotFound {
		t.Errorf("expected error code %d, got %d", ErrWorkerNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "worker not found" {
		t.Errorf("expected error message 'worker not found', got '%s'", resp.Error.Message)
	}
}

func TestErrorCodes(t *testing.T) {
	// Verify standard JSON-RPC error codes
	if ParseError != -32700 {
		t.Errorf("ParseError should be -32700, got %d", ParseError)
	}
	if InvalidRequest != -32600 {
		t.Errorf("InvalidRequest should be -32600, got %d", InvalidRequest)
	}
	if MethodNotFound != -32601 {
		t.Errorf("MethodNotFound should be -32601, got %d", MethodNotFound)
	}
	if InvalidParams != -32602 {
		t.Errorf("InvalidParams should be -32602, got %d", InvalidParams)
	}
	if InternalError != -32603 {
		t.Errorf("InternalError should be -32603, got %d", InternalError)
	}

	// Verify application-specific codes are in reserved range
	appCodes := []int{
		ErrDaemonNotRunning,
		ErrWorkerNotFound,
		ErrJobNotFound,
		ErrInvalidState,
		ErrTerritoryExists,
		ErrReviewNotFound,
		ErrOperationNotFound,
		ErrGateFailed,
		ErrMergeConflict,
	}

	for _, code := range appCodes {
		if code > -32000 || code < -32099 {
			t.Errorf("application error code %d not in reserved range [-32099, -32000]", code)
		}
	}
}

func TestMethodConstants(t *testing.T) {
	// Verify method constants match expected strings
	methods := map[string]string{
		MethodStatus:          "status",
		MethodShutdown:        "shutdown",
		MethodTerritoryInit:   "territory.init",
		MethodWorkerAdd:       "worker.add",
		MethodJobAdd:          "job.add",
		MethodQueueStatus:     "queue.status",
		MethodReviewStart:     "review.start",
		MethodOperationCreate: "operation.create",
		MethodSubscribe:       "subscribe",
	}

	for constant, expected := range methods {
		if constant != expected {
			t.Errorf("method constant mismatch: expected %s, got %s", expected, constant)
		}
	}
}

func TestRequestJSONRoundTrip(t *testing.T) {
	params := WorkerAddParams{Name: "test-worker", Role: "soldato"}
	req, err := NewRequest(NewStringID("test"), MethodWorkerAdd, params)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Marshal
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Unmarshal
	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.JSONRPC != JSONRPCVersion {
		t.Errorf("JSONRPC mismatch after round-trip")
	}
	if decoded.Method != MethodWorkerAdd {
		t.Errorf("method mismatch after round-trip")
	}
}

func TestResponseJSONRoundTrip(t *testing.T) {
	result := StatusResult{
		Running:    true,
		Version:    "1.0.0",
		Uptime:     3600,
		Workers:    5,
		ActiveJobs: 3,
	}

	resp, err := NewResponse(NewIntID(1), result)
	if err != nil {
		t.Fatalf("failed to create response: %v", err)
	}

	// Marshal
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Unmarshal
	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Decode result
	var decodedResult StatusResult
	if err := json.Unmarshal(decoded.Result, &decodedResult); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decodedResult.Running != true {
		t.Error("Running mismatch")
	}
	if decodedResult.Workers != 5 {
		t.Error("Workers mismatch")
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func int64Ptr(n int64) *int64 {
	return &n
}
