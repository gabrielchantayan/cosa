// Package notify provides notification functionality for Cosa.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"cosa/internal/config"
)

// EventType represents the type of notification event.
type EventType string

const (
	EventJobCompleted  EventType = "job_completed"
	EventJobFailed     EventType = "job_failed"
	EventWorkerStuck   EventType = "worker_stuck"
	EventBudgetWarning EventType = "budget_warning"
	EventBudgetExceeded EventType = "budget_exceeded"
)

// Notification represents a notification to be sent.
type Notification struct {
	Event       EventType
	Title       string
	Message     string
	JobID       string
	WorkerName  string
	Severity    string // info, warning, error
	Timestamp   time.Time
	ExtraFields map[string]string
}

// Notifier handles sending notifications through multiple channels.
type Notifier struct {
	config     *config.NotificationConfig
	httpClient *http.Client
	mu         sync.Mutex
}

// New creates a new notifier with the given configuration.
func New(cfg *config.NotificationConfig) *Notifier {
	return &Notifier{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotifyJobComplete sends a notification for a completed job.
func (n *Notifier) NotifyJobComplete(jobID, description, workerName string) {
	if !n.config.OnJobComplete {
		return
	}

	notif := Notification{
		Event:      EventJobCompleted,
		Title:      "Job Completed",
		Message:    truncate(description, 100),
		JobID:      jobID,
		WorkerName: workerName,
		Severity:   "info",
		Timestamp:  time.Now(),
	}

	if notif.Message == "" {
		notif.Message = fmt.Sprintf("Job %s completed", truncateID(jobID))
	}

	n.send(notif)
}

// NotifyJobFailed sends a notification for a failed job.
func (n *Notifier) NotifyJobFailed(jobID, description, workerName, err string) {
	if !n.config.OnJobFailed {
		return
	}

	message := truncate(description, 60)
	if err != "" {
		message += ": " + truncate(err, 40)
	}
	if message == "" {
		message = fmt.Sprintf("Job %s failed", truncateID(jobID))
	}

	notif := Notification{
		Event:      EventJobFailed,
		Title:      "Job Failed",
		Message:    message,
		JobID:      jobID,
		WorkerName: workerName,
		Severity:   "error",
		Timestamp:  time.Now(),
		ExtraFields: map[string]string{
			"error": err,
		},
	}

	n.send(notif)
}

// NotifyWorkerStuck sends a notification for a stuck worker.
func (n *Notifier) NotifyWorkerStuck(workerName, severity string) {
	if !n.config.OnWorkerStuck {
		return
	}

	notif := Notification{
		Event:      EventWorkerStuck,
		Title:      "Worker Stuck",
		Message:    fmt.Sprintf("Worker %s appears stuck (%s)", workerName, severity),
		WorkerName: workerName,
		Severity:   mapSeverity(severity),
		Timestamp:  time.Now(),
	}

	n.send(notif)
}

// NotifyBudgetWarning sends a notification when cost approaches budget threshold.
func (n *Notifier) NotifyBudgetWarning(currentCost, budgetLimit float64, percentage int) {
	if !n.config.OnBudgetAlert {
		return
	}

	notif := Notification{
		Event:    EventBudgetWarning,
		Title:    "Budget Warning",
		Message:  fmt.Sprintf("Cost has reached %d%% of budget ($%.2f / $%.2f)", percentage, currentCost, budgetLimit),
		Severity: "warning",
		Timestamp: time.Now(),
		ExtraFields: map[string]string{
			"current_cost":  fmt.Sprintf("$%.2f", currentCost),
			"budget_limit":  fmt.Sprintf("$%.2f", budgetLimit),
			"percentage":    fmt.Sprintf("%d%%", percentage),
		},
	}

	n.send(notif)
}

// NotifyBudgetExceeded sends a notification when cost exceeds budget.
func (n *Notifier) NotifyBudgetExceeded(currentCost, budgetLimit float64) {
	if !n.config.OnBudgetAlert {
		return
	}

	notif := Notification{
		Event:    EventBudgetExceeded,
		Title:    "Budget Exceeded",
		Message:  fmt.Sprintf("Cost ($%.2f) has exceeded budget ($%.2f)", currentCost, budgetLimit),
		Severity: "error",
		Timestamp: time.Now(),
		ExtraFields: map[string]string{
			"current_cost": fmt.Sprintf("$%.2f", currentCost),
			"budget_limit": fmt.Sprintf("$%.2f", budgetLimit),
		},
	}

	n.send(notif)
}

// Notify sends a generic notification.
func (n *Notifier) Notify(title, message string) {
	notif := Notification{
		Title:     title,
		Message:   message,
		Severity:  "info",
		Timestamp: time.Now(),
	}
	n.send(notif)
}

func (n *Notifier) send(notif Notification) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// System notifications (macOS)
	if n.config.SystemNotifications {
		n.sendSystemNotification(notif.Title, notif.Message)
	}

	// Terminal bell
	if n.config.TerminalBell {
		n.sendTerminalBell()
	}

	// Slack
	if n.config.Slack.Enabled && n.config.Slack.WebhookURL != "" {
		go n.sendSlackNotification(notif)
	}

	// Discord
	if n.config.Discord.Enabled && n.config.Discord.WebhookURL != "" {
		go n.sendDiscordNotification(notif)
	}

	// Generic webhook
	if n.config.Webhook.Enabled && n.config.Webhook.URL != "" {
		go n.sendWebhookNotification(notif)
	}
}

func (n *Notifier) sendSystemNotification(title, message string) {
	// Use osascript for macOS notifications
	script := fmt.Sprintf(`display notification %q with title %q sound name "Pop"`,
		message, "Cosa: "+title)

	cmd := exec.Command("osascript", "-e", script)
	cmd.Run() // Ignore errors - notifications are best-effort
}

func (n *Notifier) sendTerminalBell() {
	fmt.Fprint(os.Stderr, "\a")
}

// sendSlackNotification sends a notification to Slack via webhook.
func (n *Notifier) sendSlackNotification(notif Notification) {
	color := getSlackColor(notif.Severity)

	// Build fields for the attachment
	var fields []map[string]interface{}

	if notif.JobID != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Job ID",
			"value": truncateID(notif.JobID),
			"short": true,
		})
	}

	if notif.WorkerName != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Worker",
			"value": notif.WorkerName,
			"short": true,
		})
	}

	for key, value := range notif.ExtraFields {
		fields = append(fields, map[string]interface{}{
			"title": key,
			"value": value,
			"short": true,
		})
	}

	payload := map[string]interface{}{
		"username":   "Cosa",
		"icon_emoji": ":robot_face:",
		"attachments": []map[string]interface{}{
			{
				"fallback":  fmt.Sprintf("%s: %s", notif.Title, notif.Message),
				"color":     color,
				"title":     notif.Title,
				"text":      notif.Message,
				"fields":    fields,
				"footer":    "Cosa Agent Framework",
				"ts":        notif.Timestamp.Unix(),
			},
		},
	}

	// Add channel override if configured
	if n.config.Slack.Channel != "" {
		payload["channel"] = n.config.Slack.Channel
	}

	n.postJSON(n.config.Slack.WebhookURL, payload)
}

// sendDiscordNotification sends a notification to Discord via webhook.
func (n *Notifier) sendDiscordNotification(notif Notification) {
	color := getDiscordColor(notif.Severity)

	// Build fields for the embed
	var fields []map[string]interface{}

	if notif.JobID != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Job ID",
			"value":  truncateID(notif.JobID),
			"inline": true,
		})
	}

	if notif.WorkerName != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Worker",
			"value":  notif.WorkerName,
			"inline": true,
		})
	}

	for key, value := range notif.ExtraFields {
		fields = append(fields, map[string]interface{}{
			"name":   key,
			"value":  value,
			"inline": true,
		})
	}

	payload := map[string]interface{}{
		"username": "Cosa",
		"embeds": []map[string]interface{}{
			{
				"title":       notif.Title,
				"description": notif.Message,
				"color":       color,
				"fields":      fields,
				"footer": map[string]interface{}{
					"text": "Cosa Agent Framework",
				},
				"timestamp": notif.Timestamp.Format(time.RFC3339),
			},
		},
	}

	n.postJSON(n.config.Discord.WebhookURL, payload)
}

// sendWebhookNotification sends a notification to a generic webhook endpoint.
func (n *Notifier) sendWebhookNotification(notif Notification) {
	payload := map[string]interface{}{
		"event":       string(notif.Event),
		"title":       notif.Title,
		"message":     notif.Message,
		"severity":    notif.Severity,
		"timestamp":   notif.Timestamp.Format(time.RFC3339),
		"job_id":      notif.JobID,
		"worker_name": notif.WorkerName,
		"extra":       notif.ExtraFields,
	}

	// Add custom headers
	req, err := http.NewRequest("POST", n.config.Webhook.URL, nil)
	if err != nil {
		return
	}

	// Add configured headers
	for key, value := range n.config.Webhook.Headers {
		req.Header.Set(key, value)
	}

	// Add secret as header if configured
	if n.config.Webhook.Secret != "" {
		req.Header.Set("X-Cosa-Secret", n.config.Webhook.Secret)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req.Body = bytesReadCloser(body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (n *Notifier) postJSON(url string, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	resp, err := n.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func mapSeverity(severity string) string {
	switch severity {
	case "critical", "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func getSlackColor(severity string) string {
	switch severity {
	case "error":
		return "danger"
	case "warning":
		return "warning"
	default:
		return "good"
	}
}

func getDiscordColor(severity string) int {
	switch severity {
	case "error":
		return 15158332 // Red
	case "warning":
		return 15105570 // Orange
	default:
		return 3066993 // Green
	}
}

// bytesReadCloser wraps a byte slice in an io.ReadCloser.
func bytesReadCloser(b []byte) *bytesRC {
	return &bytesRC{Reader: bytes.NewReader(b)}
}

type bytesRC struct {
	*bytes.Reader
}

func (b *bytesRC) Close() error {
	return nil
}
