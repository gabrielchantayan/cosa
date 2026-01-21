package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"cosa/internal/config"
)

func TestNotifier_NotifyJobComplete(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnJobComplete: true,
	}
	n := New(cfg)

	// Should not panic when called
	n.NotifyJobComplete("job-123", "Test job description", "worker-1")
}

func TestNotifier_NotifyJobComplete_Disabled(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnJobComplete: false,
	}
	n := New(cfg)

	// Should do nothing when disabled
	n.NotifyJobComplete("job-123", "Test job description", "worker-1")
}

func TestNotifier_NotifyJobFailed(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnJobFailed: true,
	}
	n := New(cfg)

	// Should not panic when called
	n.NotifyJobFailed("job-123", "Test job description", "worker-1", "some error")
}

func TestNotifier_NotifyWorkerStuck(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnWorkerStuck: true,
	}
	n := New(cfg)

	// Should not panic when called
	n.NotifyWorkerStuck("worker-1", "warning")
}

func TestNotifier_NotifyBudgetWarning(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnBudgetAlert: true,
	}
	n := New(cfg)

	// Should not panic when called
	n.NotifyBudgetWarning(8.0, 10.0, 80)
}

func TestNotifier_NotifyBudgetExceeded(t *testing.T) {
	cfg := &config.NotificationConfig{
		OnBudgetAlert: true,
	}
	n := New(cfg)

	// Should not panic when called
	n.NotifyBudgetExceeded(12.0, 10.0)
}

func TestNotifier_SlackWebhook(t *testing.T) {
	var received struct {
		Username    string                   `json:"username"`
		Attachments []map[string]interface{} `json:"attachments"`
	}
	var mu sync.Mutex
	var requestReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestReceived = true

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		OnJobComplete: true,
		Slack: config.SlackConfig{
			Enabled:    true,
			WebhookURL: server.URL,
		},
	}
	n := New(cfg)

	n.NotifyJobComplete("job-123", "Test completed", "test-worker")

	// Wait for async request
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !requestReceived {
		t.Error("expected Slack webhook request")
	}
	if received.Username != "Cosa" {
		t.Errorf("expected username 'Cosa', got '%s'", received.Username)
	}
	if len(received.Attachments) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(received.Attachments))
	}
}

func TestNotifier_DiscordWebhook(t *testing.T) {
	var received struct {
		Username string                   `json:"username"`
		Embeds   []map[string]interface{} `json:"embeds"`
	}
	var mu sync.Mutex
	var requestReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestReceived = true

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		OnJobFailed: true,
		Discord: config.DiscordConfig{
			Enabled:    true,
			WebhookURL: server.URL,
		},
	}
	n := New(cfg)

	n.NotifyJobFailed("job-456", "Test failed", "test-worker", "error message")

	// Wait for async request
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !requestReceived {
		t.Error("expected Discord webhook request")
	}
	if received.Username != "Cosa" {
		t.Errorf("expected username 'Cosa', got '%s'", received.Username)
	}
	if len(received.Embeds) != 1 {
		t.Errorf("expected 1 embed, got %d", len(received.Embeds))
	}
}

func TestNotifier_GenericWebhook(t *testing.T) {
	var received map[string]interface{}
	var mu sync.Mutex
	var requestReceived bool
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestReceived = true
		receivedHeaders = r.Header.Clone()

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		OnWorkerStuck: true,
		Webhook: config.WebhookConfig{
			Enabled: true,
			URL:     server.URL,
			Secret:  "test-secret",
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
	}
	n := New(cfg)

	n.NotifyWorkerStuck("paulie", "critical")

	// Wait for async request
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !requestReceived {
		t.Error("expected webhook request")
	}

	if received["event"] != "worker_stuck" {
		t.Errorf("expected event 'worker_stuck', got '%v'", received["event"])
	}
	if received["severity"] != "error" {
		t.Errorf("expected severity 'error', got '%v'", received["severity"])
	}

	if receivedHeaders.Get("X-Cosa-Secret") != "test-secret" {
		t.Errorf("expected secret header 'test-secret', got '%s'", receivedHeaders.Get("X-Cosa-Secret"))
	}
	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected custom header 'custom-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is .."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc12345", "abc12345"},
		{"abc123456789", "abc12345"},
		{"short", "short"},
	}

	for _, tt := range tests {
		got := truncateID(tt.input)
		if got != tt.want {
			t.Errorf("truncateID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"critical", "error"},
		{"error", "error"},
		{"warning", "warning"},
		{"info", "info"},
		{"unknown", "info"},
	}

	for _, tt := range tests {
		got := mapSeverity(tt.input)
		if got != tt.want {
			t.Errorf("mapSeverity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetSlackColor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"error", "danger"},
		{"warning", "warning"},
		{"info", "good"},
	}

	for _, tt := range tests {
		got := getSlackColor(tt.input)
		if got != tt.want {
			t.Errorf("getSlackColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetDiscordColor(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"error", 15158332},
		{"warning", 15105570},
		{"info", 3066993},
	}

	for _, tt := range tests {
		got := getDiscordColor(tt.input)
		if got != tt.want {
			t.Errorf("getDiscordColor(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNotifier_MultipleChannels(t *testing.T) {
	var slackReceived, discordReceived, webhookReceived bool
	var mu sync.Mutex

	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		slackReceived = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer slackServer.Close()

	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		discordReceived = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer discordServer.Close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		webhookReceived = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.NotificationConfig{
		OnJobComplete: true,
		Slack: config.SlackConfig{
			Enabled:    true,
			WebhookURL: slackServer.URL,
		},
		Discord: config.DiscordConfig{
			Enabled:    true,
			WebhookURL: discordServer.URL,
		},
		Webhook: config.WebhookConfig{
			Enabled: true,
			URL:     webhookServer.URL,
		},
	}
	n := New(cfg)

	n.NotifyJobComplete("job-123", "Test completed", "test-worker")

	// Wait for async requests
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !slackReceived {
		t.Error("expected Slack webhook request")
	}
	if !discordReceived {
		t.Error("expected Discord webhook request")
	}
	if !webhookReceived {
		t.Error("expected generic webhook request")
	}
}
