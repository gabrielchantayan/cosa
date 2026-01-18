// Package notify provides notification functionality for Cosa.
package notify

import (
	"fmt"
	"os"
	"os/exec"

	"cosa/internal/config"
)

// Notifier handles sending notifications.
type Notifier struct {
	config *config.NotificationConfig
}

// New creates a new notifier with the given configuration.
func New(cfg *config.NotificationConfig) *Notifier {
	return &Notifier{config: cfg}
}

// NotifyJobComplete sends a notification for a completed job.
func (n *Notifier) NotifyJobComplete(jobID, description string) {
	if !n.config.OnJobComplete {
		return
	}

	title := "Job Completed"
	message := truncate(description, 100)
	if message == "" {
		message = fmt.Sprintf("Job %s completed", truncateID(jobID))
	}

	n.send(title, message)
}

// NotifyJobFailed sends a notification for a failed job.
func (n *Notifier) NotifyJobFailed(jobID, description, err string) {
	if !n.config.OnJobFailed {
		return
	}

	title := "Job Failed"
	message := truncate(description, 60)
	if err != "" {
		message += ": " + truncate(err, 40)
	}
	if message == "" {
		message = fmt.Sprintf("Job %s failed", truncateID(jobID))
	}

	n.send(title, message)
}

// NotifyWorkerStuck sends a notification for a stuck worker.
func (n *Notifier) NotifyWorkerStuck(workerName, severity string) {
	if !n.config.OnWorkerStuck {
		return
	}

	title := "Worker Stuck"
	message := fmt.Sprintf("Worker %s appears stuck (%s)", workerName, severity)

	n.send(title, message)
}

// Notify sends a generic notification.
func (n *Notifier) Notify(title, message string) {
	n.send(title, message)
}

func (n *Notifier) send(title, message string) {
	if n.config.SystemNotifications {
		n.sendSystemNotification(title, message)
	}

	if n.config.TerminalBell {
		n.sendTerminalBell()
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
