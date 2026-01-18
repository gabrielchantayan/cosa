// Package page provides page views for the TUI.
package page

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/protocol"
	"cosa/internal/tui/component"
	"cosa/internal/tui/styles"
	"cosa/internal/tui/theme"
)

// FocusArea represents which panel is focused.
type FocusArea int

const (
	FocusWorkers FocusArea = iota
	FocusJobs
	FocusActivity
)

// Dashboard is the main dashboard page.
type Dashboard struct {
	styles  styles.Styles
	width   int
	height  int
	status  *protocol.StatusResult

	focus      FocusArea
	workerList *component.WorkerList
	jobList    *component.JobList
	activity   *component.Activity
}

// NewDashboard creates a new dashboard page.
func NewDashboard() *Dashboard {
	return &Dashboard{
		styles:     styles.New(),
		workerList: component.NewWorkerList(),
		jobList:    component.NewJobList(),
		activity:   component.NewActivity(),
	}
}

// SetSize sets the page dimensions.
func (d *Dashboard) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.updateComponentSizes()
}

// SetStatus updates the daemon status.
func (d *Dashboard) SetStatus(status *protocol.StatusResult) {
	d.status = status
}

// SetWorkers updates the worker list.
func (d *Dashboard) SetWorkers(workers []protocol.WorkerInfo) {
	d.workerList.SetWorkers(workers)
}

// SetJobs updates the job list.
func (d *Dashboard) SetJobs(jobs []protocol.JobInfo) {
	d.jobList.SetJobs(jobs)
}

// AddActivity adds an activity item.
func (d *Dashboard) AddActivity(time, worker, message string) {
	d.activity.AddItem(component.ActivityItem{
		Time:    time,
		Worker:  worker,
		Message: message,
	})
}

// Focus returns the current focus area.
func (d *Dashboard) Focus() FocusArea {
	return d.focus
}

// SetFocus sets the focus area.
func (d *Dashboard) SetFocus(focus FocusArea) {
	d.focus = focus
	d.updateFocus()
}

// NextFocus moves focus to the next panel.
func (d *Dashboard) NextFocus() {
	d.focus = (d.focus + 1) % 3
	d.updateFocus()
}

// PrevFocus moves focus to the previous panel.
func (d *Dashboard) PrevFocus() {
	d.focus = (d.focus + 2) % 3
	d.updateFocus()
}

// HandleKey handles key presses on the focused component.
func (d *Dashboard) HandleKey(key string) {
	switch d.focus {
	case FocusWorkers:
		switch key {
		case "j", "down":
			d.workerList.MoveDown()
		case "k", "up":
			d.workerList.MoveUp()
		}
	case FocusJobs:
		switch key {
		case "j", "down":
			d.jobList.MoveDown()
		case "k", "up":
			d.jobList.MoveUp()
		}
	}
}

func (d *Dashboard) updateFocus() {
	d.workerList.SetFocused(d.focus == FocusWorkers)
	d.jobList.SetFocused(d.focus == FocusJobs)
}

func (d *Dashboard) updateComponentSizes() {
	// Left column: 30% width, workers and jobs stacked
	leftWidth := d.width * 30 / 100
	leftHeight := (d.height - 4) / 2 // Account for header/footer, split in half

	d.workerList.SetSize(leftWidth, leftHeight)
	d.jobList.SetSize(leftWidth, leftHeight)

	// Right column: 70% width, activity
	rightWidth := d.width - leftWidth - 3
	rightHeight := d.height - 4

	d.activity.SetSize(rightWidth, rightHeight)
}

// View renders the dashboard.
func (d *Dashboard) View() string {
	t := theme.Current

	// Header
	header := d.renderHeader()

	// Left column: Workers + Jobs
	leftWidth := d.width * 30 / 100
	leftHeight := (d.height - 4) / 2

	workersPanel := d.renderPanel("WORKERS", d.workerList.View(), leftWidth, leftHeight, d.focus == FocusWorkers)
	jobsPanel := d.renderPanel("JOBS", d.jobList.View(), leftWidth, leftHeight, d.focus == FocusJobs)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, workersPanel, jobsPanel)

	// Right column: Activity
	rightWidth := d.width - leftWidth - 3
	rightHeight := d.height - 4
	activityPanel := d.renderPanel("ACTIVITY", d.activity.View(), rightWidth, rightHeight, d.focus == FocusActivity)

	// Join columns
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, activityPanel)

	// Footer
	footer := d.renderFooter()

	// Combine with background
	result := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)

	return lipgloss.NewStyle().
		Background(t.Background).
		Width(d.width).
		Height(d.height).
		Render(result)
}

func (d *Dashboard) renderHeader() string {
	t := theme.Current

	// Logo/title
	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("◆ COSA NOSTRA")

	// Status info
	var statusInfo string
	if d.status != nil {
		uptime := formatUptime(d.status.Uptime)
		statusInfo = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("v%s │ %s │ %d workers │ %d jobs",
				d.status.Version, uptime, d.status.Workers, d.status.ActiveJobs))
	}

	// Spacer
	spacerWidth := d.width - lipgloss.Width(title) - lipgloss.Width(statusInfo) - 4
	spacer := strings.Repeat(" ", max(spacerWidth, 1))

	header := fmt.Sprintf(" %s%s%s ", title, spacer, statusInfo)

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(d.width).
		Render(header)
}

func (d *Dashboard) renderFooter() string {
	t := theme.Current

	keys := []struct {
		key  string
		desc string
	}{
		{"Tab", "switch panel"},
		{"j/k", "navigate"},
		{"a", "add worker"},
		{"n", "new job"},
		{"q", "quit"},
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
		Width(d.width).
		Render(footer)
}

func (d *Dashboard) renderPanel(title, content string, width, height int, focused bool) string {
	t := theme.Current

	// Ensure minimum dimensions to avoid slice bounds errors
	if width < 6 {
		width = 6
	}
	if height < 4 {
		height = 4
	}

	var borderStyle lipgloss.Style
	var titleStyle lipgloss.Style

	if focused {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderActive)
		titleStyle = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
	} else {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border)
		titleStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
	}

	// Title with padding
	titleStr := titleStyle.Render(fmt.Sprintf(" %s ", title))

	// Content area
	contentWidth := width - 4  // Account for borders and padding
	contentHeight := height - 3 // Account for title and borders

	// Pad or truncate content
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < contentHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > contentHeight {
		contentLines = contentLines[:contentHeight]
	}

	// Ensure each line is the right width
	for i, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			contentLines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	paddedContent := strings.Join(contentLines, "\n")

	panel := borderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(paddedContent)

	// Add title to top border
	lines := strings.Split(panel, "\n")
	if len(lines) > 0 {
		firstLine := lines[0]
		// Replace part of the top border with the title
		titleWidth := lipgloss.Width(titleStr)
		if len(firstLine) > titleWidth+4 {
			lines[0] = firstLine[:2] + titleStr + firstLine[2+titleWidth:]
		}
	}

	return strings.Join(lines, "\n")
}

func formatUptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// IsInputMode returns true if the dashboard is in input mode.
func (d *Dashboard) IsInputMode() bool {
	// For now, no input mode - will be used when dialogs are added
	return false
}

// ShowNewJobDialog shows the new job dialog.
func (d *Dashboard) ShowNewJobDialog() {
	// Stub for new job dialog - will be implemented with full input component
	d.AddActivity(time.Now().Format("15:04:05"), "", "New job dialog (press ESC to close)")
}

// ShowNewOperationDialog shows the new operation dialog.
func (d *Dashboard) ShowNewOperationDialog() {
	// Stub for new operation dialog
	d.AddActivity(time.Now().Format("15:04:05"), "", "New operation dialog (press ESC to close)")
}

// ShowSearch shows the search interface.
func (d *Dashboard) ShowSearch() {
	// Stub for search
	d.AddActivity(time.Now().Format("15:04:05"), "", "Search mode (press ESC to close)")
}

// ShowCommandPalette shows the command palette.
func (d *Dashboard) ShowCommandPalette() {
	// Stub for command palette
	d.AddActivity(time.Now().Format("15:04:05"), "", "Command palette (press ESC to close)")
}

// ToggleHelp toggles the help overlay.
func (d *Dashboard) ToggleHelp() {
	// Stub for help overlay
	d.AddActivity(time.Now().Format("15:04:05"), "", "Help: Tab=switch, j/k=nav, n=new job, o=new op, /=search, :=cmd, ?=help, q=quit")
}

// SelectCurrent selects the currently focused item.
func (d *Dashboard) SelectCurrent() {
	switch d.focus {
	case FocusWorkers:
		if selected := d.workerList.Selected(); selected != nil {
			d.AddActivity(time.Now().Format("15:04:05"), selected.Name, "Selected worker")
		}
	case FocusJobs:
		if selected := d.jobList.Selected(); selected != nil {
			d.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Selected job: %s", selected.ID[:8]))
		}
	}
}

// CloseOverlay closes any open overlay/dialog.
func (d *Dashboard) CloseOverlay() {
	// Stub for closing overlays
}
