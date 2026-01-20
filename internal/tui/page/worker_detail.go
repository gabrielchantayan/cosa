// Package page provides page views for the TUI.
package page

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/protocol"
	"cosa/internal/tui/styles"
	"cosa/internal/tui/theme"
)

// WorkerDetail shows detailed information about a worker.
type WorkerDetail struct {
	styles styles.Styles
	width  int
	height int

	worker       *protocol.WorkerDetailInfo
	activities   []ActivityEntry
	sessionLines []string

	// Scroll positions
	activityScroll int
	sessionScroll  int

	// Focus: 0=activity, 1=session
	focusSection int

	// Message input
	inputMode    bool
	inputBuffer  string
	inputCursor  int
}

// ActivityEntry represents an activity log entry.
type ActivityEntry struct {
	Time    string
	Message string
}

// NewWorkerDetail creates a new worker detail view.
func NewWorkerDetail() *WorkerDetail {
	return &WorkerDetail{
		styles:       styles.New(),
		activities:   make([]ActivityEntry, 0),
		sessionLines: make([]string, 0),
	}
}

// SetSize sets the view dimensions.
func (w *WorkerDetail) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// SetWorker sets the worker to display.
func (w *WorkerDetail) SetWorker(worker *protocol.WorkerDetailInfo) {
	w.worker = worker
}

// AddActivity adds an activity entry.
func (w *WorkerDetail) AddActivity(time, message string) {
	w.activities = append(w.activities, ActivityEntry{
		Time:    time,
		Message: message,
	})
	// Keep only last 100 entries
	if len(w.activities) > 100 {
		w.activities = w.activities[len(w.activities)-100:]
	}
}

// SetSessionOutput sets the session output lines.
func (w *WorkerDetail) SetSessionOutput(lines []string) {
	w.sessionLines = lines
}

// AppendSessionOutput appends a line to the session output.
func (w *WorkerDetail) AppendSessionOutput(line string) {
	w.sessionLines = append(w.sessionLines, line)
	// Keep last 1000 lines
	if len(w.sessionLines) > 1000 {
		w.sessionLines = w.sessionLines[len(w.sessionLines)-1000:]
	}
}

// IsInputMode returns true if in message input mode.
func (w *WorkerDetail) IsInputMode() bool {
	return w.inputMode
}

// StartInput enters message input mode.
func (w *WorkerDetail) StartInput() {
	w.inputMode = true
	w.inputBuffer = ""
	w.inputCursor = 0
}

// CancelInput exits message input mode.
func (w *WorkerDetail) CancelInput() {
	w.inputMode = false
	w.inputBuffer = ""
	w.inputCursor = 0
}

// GetInput returns the current input buffer and clears it.
func (w *WorkerDetail) GetInput() string {
	msg := w.inputBuffer
	w.inputBuffer = ""
	w.inputCursor = 0
	w.inputMode = false
	return msg
}

// HandleKey handles key presses.
func (w *WorkerDetail) HandleKey(key string) {
	if w.inputMode {
		w.handleInputKey(key)
		return
	}

	switch key {
	case "tab":
		w.focusSection = (w.focusSection + 1) % 2
	case "j", "down":
		w.scrollDown()
	case "k", "up":
		w.scrollUp()
	case "g":
		w.scrollToTop()
	case "G":
		w.scrollToBottom()
	case "m":
		w.StartInput()
	}
}

func (w *WorkerDetail) handleInputKey(key string) {
	switch key {
	case "enter":
		// Input will be retrieved by caller
	case "esc":
		w.CancelInput()
	case "backspace":
		if w.inputCursor > 0 && len(w.inputBuffer) > 0 {
			w.inputBuffer = w.inputBuffer[:w.inputCursor-1] + w.inputBuffer[w.inputCursor:]
			w.inputCursor--
		}
	case "left":
		if w.inputCursor > 0 {
			w.inputCursor--
		}
	case "right":
		if w.inputCursor < len(w.inputBuffer) {
			w.inputCursor++
		}
	default:
		if len(key) == 1 {
			w.inputBuffer = w.inputBuffer[:w.inputCursor] + key + w.inputBuffer[w.inputCursor:]
			w.inputCursor++
		}
	}
}

func (w *WorkerDetail) scrollDown() {
	if w.focusSection == 0 {
		maxScroll := max(0, len(w.activities)-5)
		if w.activityScroll < maxScroll {
			w.activityScroll++
		}
	} else {
		maxScroll := max(0, len(w.sessionLines)-10)
		if w.sessionScroll < maxScroll {
			w.sessionScroll++
		}
	}
}

func (w *WorkerDetail) scrollUp() {
	if w.focusSection == 0 {
		if w.activityScroll > 0 {
			w.activityScroll--
		}
	} else {
		if w.sessionScroll > 0 {
			w.sessionScroll--
		}
	}
}

func (w *WorkerDetail) scrollToTop() {
	if w.focusSection == 0 {
		w.activityScroll = 0
	} else {
		w.sessionScroll = 0
	}
}

func (w *WorkerDetail) scrollToBottom() {
	if w.focusSection == 0 {
		w.activityScroll = max(0, len(w.activities)-5)
	} else {
		w.sessionScroll = max(0, len(w.sessionLines)-10)
	}
}

// View renders the worker detail view.
func (w *WorkerDetail) View() string {
	t := theme.Current

	if w.worker == nil {
		return lipgloss.NewStyle().
			Width(w.width).
			Height(w.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.TextMuted).
			Render("No worker selected")
	}

	// Header: name, role, status, current job
	header := w.renderHeader()

	// Calculate section heights
	contentHeight := w.height - 6 // Account for header and footer
	activityHeight := contentHeight / 3
	sessionHeight := contentHeight - activityHeight

	// Activity section
	activitySection := w.renderActivitySection(activityHeight)

	// Session output section
	sessionSection := w.renderSessionSection(sessionHeight)

	// Message input or footer
	var footer string
	if w.inputMode {
		footer = w.renderInput()
	} else {
		footer = w.renderFooter()
	}

	// Combine all sections
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		activitySection,
		sessionSection,
		footer,
	)

	return lipgloss.NewStyle().
		Background(t.Background).
		Width(w.width).
		Height(w.height).
		Render(content)
}

func (w *WorkerDetail) renderHeader() string {
	t := theme.Current

	if w.worker == nil {
		return ""
	}

	// Worker name and role
	nameStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	roleStyle := lipgloss.NewStyle().
		Foreground(t.Secondary)

	// Status badge
	var statusColor lipgloss.Color
	switch w.worker.Status {
	case "running":
		statusColor = t.Success
	case "idle":
		statusColor = t.TextMuted
	case "error":
		statusColor = t.Error
	default:
		statusColor = t.Text
	}
	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	// Current job
	jobStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	var currentJob string
	if w.worker.CurrentJob != "" {
		jobDisplay := w.worker.CurrentJob
		if len(jobDisplay) > 8 {
			jobDisplay = jobDisplay[:8]
		}
		currentJob = fmt.Sprintf("Job: %s", jobDisplay)
	} else {
		currentJob = "No active job"
	}

	// Cost info
	costStyle := lipgloss.NewStyle().
		Foreground(t.Warning)
	costInfo := ""
	if w.worker.TotalCost != "" && w.worker.TotalCost != "$0.00" {
		costInfo = fmt.Sprintf(" │ %s (%d tokens)", w.worker.TotalCost, w.worker.TotalTokens)
	}

	line1 := fmt.Sprintf(" %s %s  %s",
		nameStyle.Render(w.worker.Name),
		roleStyle.Render(fmt.Sprintf("[%s]", w.worker.Role)),
		statusStyle.Render(strings.ToUpper(w.worker.Status)),
	)

	line2 := fmt.Sprintf(" %s%s",
		jobStyle.Render(currentJob),
		costStyle.Render(costInfo),
	)

	header := lipgloss.JoinVertical(lipgloss.Left, line1, line2)

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(w.width).
		Padding(0, 0, 0, 0).
		Render(header)
}

func (w *WorkerDetail) renderActivitySection(height int) string {
	t := theme.Current

	focused := w.focusSection == 0
	borderColor := t.Border
	titleColor := t.TextMuted
	if focused {
		borderColor = t.BorderActive
		titleColor = t.Primary
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(focused)
	title := titleStyle.Render(" RECENT ACTIVITY ")

	// Content
	contentHeight := height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	var lines []string
	start := w.activityScroll
	end := min(start+contentHeight, len(w.activities))

	timeStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	msgStyle := lipgloss.NewStyle().Foreground(t.Text)

	for i := start; i < end; i++ {
		entry := w.activities[i]
		line := fmt.Sprintf(" %s %s",
			timeStyle.Render(entry.Time),
			msgStyle.Render(entry.Message),
		)
		lines = append(lines, truncateLine(line, w.width-4))
	}

	// Pad to fill height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w.width - 4).
		Height(height - 2).
		Render(content)

	// Insert title into border using shared helper
	return styles.InsertPanelTitle(panel, title, borderColor)
}

func (w *WorkerDetail) renderSessionSection(height int) string {
	t := theme.Current

	focused := w.focusSection == 1
	borderColor := t.Border
	titleColor := t.TextMuted
	if focused {
		borderColor = t.BorderActive
		titleColor = t.Primary
	}

	// Title with scroll indicator
	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(focused)

	scrollInfo := ""
	if len(w.sessionLines) > height-3 {
		scrollInfo = fmt.Sprintf(" [%d/%d]", w.sessionScroll+1, len(w.sessionLines))
	}
	title := titleStyle.Render(fmt.Sprintf(" SESSION OUTPUT%s ", scrollInfo))

	// Content
	contentHeight := height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	var lines []string
	start := w.sessionScroll
	end := min(start+contentHeight, len(w.sessionLines))

	lineStyle := lipgloss.NewStyle().Foreground(t.Text)

	for i := start; i < end; i++ {
		line := lineStyle.Render(" " + w.sessionLines[i])
		lines = append(lines, truncateLine(line, w.width-4))
	}

	// Pad to fill height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w.width - 4).
		Height(height - 2).
		Render(content)

	// Insert title into border using shared helper
	return styles.InsertPanelTitle(panel, title, borderColor)
}

func (w *WorkerDetail) renderInput() string {
	t := theme.Current

	promptStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	// Show cursor
	beforeCursor := w.inputBuffer[:w.inputCursor]
	afterCursor := w.inputBuffer[w.inputCursor:]
	cursorStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background)

	cursor := cursorStyle.Render(" ")
	if w.inputCursor < len(w.inputBuffer) {
		cursor = cursorStyle.Render(string(w.inputBuffer[w.inputCursor]))
		afterCursor = w.inputBuffer[w.inputCursor+1:]
	}

	input := promptStyle.Render(" > ") + inputStyle.Render(beforeCursor) + cursor + inputStyle.Render(afterCursor)

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(w.width).
		Render(input)
}

func (w *WorkerDetail) renderFooter() string {
	t := theme.Current

	keys := []struct {
		key  string
		desc string
	}{
		{"Tab", "switch section"},
		{"j/k", "scroll"},
		{"g/G", "top/bottom"},
		{"m", "message"},
		{"Esc", "back"},
	}

	var parts []string
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	for _, k := range keys {
		parts = append(parts, keyStyle.Render(k.key)+" "+descStyle.Render(k.desc))
	}

	footer := " " + strings.Join(parts, "  │  ")

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(w.width).
		Render(footer)
}

func truncateLine(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Simple truncation - could be improved to handle ANSI codes
	runes := []rune(s)
	if len(runes) > maxWidth-2 {
		return string(runes[:maxWidth-2]) + ".."
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
