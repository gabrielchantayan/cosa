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

// ForcedBackground is the hardcoded background color for the TUI.
const ForcedBackground = lipgloss.Color("#0a0a0a")

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

	// Dialogs
	newJobDialog *component.Dialog

	// Callbacks
	onCreateJob func(description string)
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
	// Handle dialog input if visible
	if d.newJobDialog != nil && d.newJobDialog.Visible() {
		action := d.newJobDialog.HandleKey(key)
		switch action {
		case "create":
			description := d.newJobDialog.GetInputValue()
			if description != "" && d.onCreateJob != nil {
				d.onCreateJob(description)
			}
			d.newJobDialog.Hide()
			d.newJobDialog = nil
		case "cancel":
			d.newJobDialog.Hide()
			d.newJobDialog = nil
		}
		return
	}

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
	// Content area height (total - header - footer)
	contentHeight := d.height - 2

	// Left column: 30% width
	leftWidth := d.width * 30 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}

	// Each left panel gets half the content height
	// Account for panel borders (2 lines each for top/bottom border)
	leftPanelHeight := contentHeight / 2
	innerLeftHeight := leftPanelHeight - 2 // Inner height after border
	if innerLeftHeight < 3 {
		innerLeftHeight = 3
	}

	d.workerList.SetSize(leftWidth-2, innerLeftHeight)
	d.jobList.SetSize(leftWidth-2, innerLeftHeight)

	// Right column: Activity
	rightWidth := d.width - leftWidth - 1
	if rightWidth < 20 {
		rightWidth = 20
	}
	innerRightHeight := contentHeight - 2
	if innerRightHeight < 5 {
		innerRightHeight = 5
	}

	d.activity.SetSize(rightWidth-2, innerRightHeight)
}

// View renders the dashboard.
func (d *Dashboard) View() string {
	t := theme.Current

	// Ensure minimum dimensions
	if d.width < 40 || d.height < 10 {
		return lipgloss.NewStyle().
			Width(d.width).
			Height(d.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.TextMuted).
			Render("Terminal too small")
	}

	// Header (1 line)
	header := d.renderHeader()

	// Footer (1 line)
	footer := d.renderFooter()

	// Content area height (total - header - footer)
	contentHeight := d.height - 2

	// Left column: 30% width, Workers and Jobs stacked
	leftWidth := d.width * 30 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}

	// Each left panel gets half the content height
	leftPanelHeight := contentHeight / 2

	workersPanel := d.renderPanel("WORKERS", d.workerList.View(), leftWidth, leftPanelHeight, d.focus == FocusWorkers)
	jobsPanel := d.renderPanel("JOBS", d.jobList.View(), leftWidth, contentHeight-leftPanelHeight, d.focus == FocusJobs)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, workersPanel, jobsPanel)

	// Right column: Activity takes remaining width and full content height
	rightWidth := d.width - leftWidth - 1 // 1 char gap between columns
	if rightWidth < 20 {
		rightWidth = 20
	}
	activityPanel := d.renderPanel("ACTIVITY", d.activity.View(), rightWidth, contentHeight, d.focus == FocusActivity)

	// Join columns horizontally with a gap
	gap := lipgloss.NewStyle().Width(1).Height(contentHeight).Render(" ")
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, gap, activityPanel)

	// Combine all sections vertically
	result := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)

	// Base view with forced background color
	baseView := lipgloss.NewStyle().
		Background(ForcedBackground).
		Width(d.width).
		Height(d.height).
		Render(result)

	// Render dialog overlay if visible
	if d.newJobDialog != nil && d.newJobDialog.Visible() {
		return d.renderWithDialogOverlay(baseView)
	}

	return baseView
}

func (d *Dashboard) renderWithDialogOverlay(baseView string) string {
	// Get dialog view centered in the screen
	dialogView := d.newJobDialog.CenterIn(d.width, d.height)

	// Split base view into lines
	baseLines := strings.Split(baseView, "\n")

	// Split dialog view into lines
	dialogLines := strings.Split(dialogView, "\n")

	// Overlay the dialog on top of the base view
	result := make([]string, d.height)
	for i := 0; i < d.height && i < len(baseLines); i++ {
		if i < len(dialogLines) && strings.TrimSpace(dialogLines[i]) != "" {
			// Use dialog line, but pad to full width
			dialogLine := dialogLines[i]
			dialogWidth := lipgloss.Width(dialogLine)
			if dialogWidth < d.width {
				dialogLine = dialogLine + strings.Repeat(" ", d.width-dialogWidth)
			}
			result[i] = dialogLine
		} else {
			result[i] = baseLines[i]
		}
	}

	return lipgloss.NewStyle().
		Background(ForcedBackground).
		Render(strings.Join(result, "\n"))
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
	if width < 8 {
		width = 8
	}
	if height < 5 {
		height = 5
	}

	var borderColor lipgloss.Color
	var titleStyle lipgloss.Style

	if focused {
		borderColor = t.BorderActive
		titleStyle = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
	} else {
		borderColor = t.Border
		titleStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
	}

	// Calculate inner content dimensions (accounting for border)
	innerWidth := width - 2   // 1 char border on each side
	innerHeight := height - 2 // 1 char border top and bottom

	if innerWidth < 4 {
		innerWidth = 4
	}
	if innerHeight < 2 {
		innerHeight = 2
	}

	// Pad or truncate content to fit
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < innerHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}

	// Ensure each line fits within the inner width
	for i, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			contentLines[i] = line + strings.Repeat(" ", innerWidth-lineWidth)
		} else if lineWidth > innerWidth {
			// Truncate line to fit
			runes := []rune(line)
			if len(runes) > innerWidth-2 {
				contentLines[i] = string(runes[:innerWidth-2]) + ".."
			}
		}
	}

	paddedContent := strings.Join(contentLines, "\n")

	// Build the panel with border
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerWidth).
		Height(innerHeight).
		Render(paddedContent)

	// Insert title into top border using shared helper
	titleStr := titleStyle.Render(fmt.Sprintf(" %s ", title))
	return styles.InsertPanelTitle(panel, titleStr, borderColor)
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
	return d.newJobDialog != nil && d.newJobDialog.Visible()
}

// SetOnCreateJob sets the callback for when a job is created.
func (d *Dashboard) SetOnCreateJob(fn func(description string)) {
	d.onCreateJob = fn
}

// ShowNewJobDialog shows the new job dialog.
func (d *Dashboard) ShowNewJobDialog() {
	d.newJobDialog = component.NewDialog("New Job")
	d.newJobDialog.SetInput("Job Description:")
	d.newJobDialog.AddButton("Create", "create", true)
	d.newJobDialog.AddButton("Cancel", "cancel", false)
	d.newJobDialog.SetSize(86, 12)
	d.newJobDialog.Show()
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
	if d.newJobDialog != nil {
		d.newJobDialog.Hide()
		d.newJobDialog = nil
	}
}
