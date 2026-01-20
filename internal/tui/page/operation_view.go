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

// OperationView shows operation progress with job status.
type OperationView struct {
	styles styles.Styles
	width  int
	height int

	operation *protocol.OperationInfo
	jobs      []OperationJobInfo

	// Scroll position
	scroll   int
	selected int
}

// OperationJobInfo contains job info for the operation view.
type OperationJobInfo struct {
	ID          string
	Description string
	Status      string // pending, running, completed, failed
	Worker      string
	Progress    float64 // 0.0 to 1.0
}

// NewOperationView creates a new operation view.
func NewOperationView() *OperationView {
	return &OperationView{
		styles: styles.New(),
		jobs:   make([]OperationJobInfo, 0),
	}
}

// SetSize sets the view dimensions.
func (o *OperationView) SetSize(width, height int) {
	o.width = width
	o.height = height
}

// SetOperation sets the operation to display.
func (o *OperationView) SetOperation(op *protocol.OperationInfo) {
	o.operation = op
}

// SetJobs sets the jobs to display.
func (o *OperationView) SetJobs(jobs []OperationJobInfo) {
	o.jobs = jobs
}

// HandleKey handles key presses.
func (o *OperationView) HandleKey(key string) {
	switch key {
	case "j", "down":
		if o.selected < len(o.jobs)-1 {
			o.selected++
			o.ensureVisible()
		}
	case "k", "up":
		if o.selected > 0 {
			o.selected--
			o.ensureVisible()
		}
	case "g":
		o.selected = 0
		o.scroll = 0
	case "G":
		o.selected = len(o.jobs) - 1
		o.ensureVisible()
	}
}

func (o *OperationView) ensureVisible() {
	visibleHeight := o.height - 10 // Account for header, footer, etc.
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if o.selected < o.scroll {
		o.scroll = o.selected
	}
	if o.selected >= o.scroll+visibleHeight {
		o.scroll = o.selected - visibleHeight + 1
	}
}

// Selected returns the selected job index.
func (o *OperationView) Selected() int {
	return o.selected
}

// View renders the operation view.
func (o *OperationView) View() string {
	t := theme.Current

	if o.operation == nil {
		return lipgloss.NewStyle().
			Width(o.width).
			Height(o.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.TextMuted).
			Render("No operation selected")
	}

	// Header with operation info
	header := o.renderHeader()

	// Overall progress bar
	progressBar := o.renderOverallProgress()

	// Jobs list with individual progress
	jobsList := o.renderJobsList()

	// Footer
	footer := o.renderFooter()

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		progressBar,
		jobsList,
		footer,
	)

	return lipgloss.NewStyle().
		Background(t.Background).
		Width(o.width).
		Height(o.height).
		Render(content)
}

func (o *OperationView) renderHeader() string {
	t := theme.Current

	if o.operation == nil {
		return ""
	}

	// Operation name
	nameStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// Status badge
	var statusColor lipgloss.Color
	switch o.operation.Status {
	case "running":
		statusColor = t.Success
	case "completed":
		statusColor = t.Primary
	case "failed":
		statusColor = t.Error
	case "cancelled":
		statusColor = t.Warning
	default:
		statusColor = t.TextMuted
	}
	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	// Description
	descStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	// Stats
	statsStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	line1 := fmt.Sprintf(" %s  %s",
		nameStyle.Render(o.operation.Name),
		statusStyle.Render(strings.ToUpper(o.operation.Status)),
	)

	line2 := fmt.Sprintf(" %s", descStyle.Render(o.operation.Description))

	stats := fmt.Sprintf(" Jobs: %d/%d completed, %d failed",
		o.operation.CompletedJobs, o.operation.TotalJobs, o.operation.FailedJobs)
	line3 := statsStyle.Render(stats)

	header := lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3)

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(o.width).
		Padding(0, 0, 0, 0).
		Render(header)
}

func (o *OperationView) renderOverallProgress() string {
	t := theme.Current

	if o.operation == nil || o.operation.TotalJobs == 0 {
		return ""
	}

	progress := float64(o.operation.CompletedJobs) / float64(o.operation.TotalJobs)
	barWidth := o.width - 20

	// Build progress bar
	filled := int(progress * float64(barWidth))
	empty := barWidth - filled

	filledStyle := lipgloss.NewStyle().
		Background(t.Success).
		Foreground(t.Success)

	emptyStyle := lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.Surface)

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	percentStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	percent := percentStyle.Render(fmt.Sprintf(" %3.0f%%", progress*100))

	return fmt.Sprintf(" %s%s\n", bar, percent)
}

func (o *OperationView) renderJobsList() string {
	t := theme.Current

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	title := titleStyle.Render(" JOBS ")

	// Calculate visible area
	contentHeight := o.height - 10
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Build job list
	var lines []string
	barWidth := o.width - 50 // Space for status, description, etc.
	if barWidth < 10 {
		barWidth = 10
	}

	end := min(o.scroll+contentHeight, len(o.jobs))
	for i := o.scroll; i < end; i++ {
		job := o.jobs[i]
		line := o.renderJobLine(job, i == o.selected, barWidth)
		lines = append(lines, line)
	}

	// Pad to fill height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Width(o.width - 4).
		Height(contentHeight + 2).
		Render(content)

	// Insert title into border using shared helper
	return styles.InsertPanelTitle(panel, title, t.BorderActive)
}

func (o *OperationView) renderJobLine(job OperationJobInfo, selected bool, barWidth int) string {
	t := theme.Current

	// Selection indicator
	selIndicator := "  "
	if selected {
		selIndicator = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render("▸ ")
	}

	// Status icon
	var statusIcon string
	var statusColor lipgloss.Color
	switch job.Status {
	case "completed":
		statusIcon = "✓"
		statusColor = t.Success
	case "running":
		statusIcon = "●"
		statusColor = t.Warning
	case "failed":
		statusIcon = "✗"
		statusColor = t.Error
	default:
		statusIcon = "○"
		statusColor = t.TextMuted
	}
	statusStyle := lipgloss.NewStyle().Foreground(statusColor)

	// Job ID (first 8 chars)
	idStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	jobID := job.ID
	if len(jobID) > 8 {
		jobID = jobID[:8]
	}

	// Description (truncated)
	descStyle := lipgloss.NewStyle().Foreground(t.Text)
	desc := job.Description
	maxDescLen := 30
	if len(desc) > maxDescLen {
		desc = desc[:maxDescLen-2] + ".."
	}

	// Progress bar for running jobs
	progressBar := ""
	if job.Status == "running" && barWidth > 0 {
		filled := int(job.Progress * float64(barWidth))
		empty := barWidth - filled

		filledStyle := lipgloss.NewStyle().Foreground(t.Success)
		emptyStyle := lipgloss.NewStyle().Foreground(t.Surface)

		progressBar = " " + filledStyle.Render(strings.Repeat("█", filled)) +
			emptyStyle.Render(strings.Repeat("░", empty)) +
			fmt.Sprintf(" %3.0f%%", job.Progress*100)
	} else if job.Status == "completed" {
		progressBar = lipgloss.NewStyle().Foreground(t.Success).Render(" ████████████ 100%")
	}

	// Worker name
	workerStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	worker := ""
	if job.Worker != "" {
		worker = workerStyle.Render(fmt.Sprintf(" [%s]", job.Worker))
	}

	return fmt.Sprintf("%s%s %s %s%s%s",
		selIndicator,
		statusStyle.Render(statusIcon),
		idStyle.Render(jobID),
		descStyle.Render(desc),
		progressBar,
		worker,
	)
}

func (o *OperationView) renderFooter() string {
	t := theme.Current

	keys := []struct {
		key  string
		desc string
	}{
		{"j/k", "navigate"},
		{"g/G", "top/bottom"},
		{"Enter", "view job"},
		{"c", "cancel"},
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
		Width(o.width).
		Render(footer)
}
