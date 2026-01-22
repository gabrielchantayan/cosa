// Package page provides page views for the TUI.
package page

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

	// Dialogs
	newJobDialog     *component.Dialog
	showDialog       bool
	templateSelector *component.TemplateSelector
	showTemplates    bool

	// Callbacks
	onCreateJob      func(description string)
	onReassignJob    func(jobID string)
	onUseTemplate    func(templateID string, variables map[string]string)
}

// NewDashboard creates a new dashboard page.
func NewDashboard() *Dashboard {
	d := &Dashboard{
		styles:           styles.New(),
		workerList:       component.NewWorkerList(),
		jobList:          component.NewJobList(),
		activity:         component.NewActivity(),
		newJobDialog:     component.NewJobDialog(),
		templateSelector: component.NewTemplateSelector(),
	}

	// Set up template selector callbacks
	d.templateSelector.SetOnSelect(func(templateID string, variables map[string]string) {
		d.showTemplates = false
		if d.onUseTemplate != nil {
			d.onUseTemplate(templateID, variables)
		}
	})
	d.templateSelector.SetOnCancel(func() {
		d.showTemplates = false
	})

	return d
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

	// Join columns horizontally with a gap
	gap := lipgloss.NewStyle().Width(1).Height(rightHeight).Render(" ")
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, gap, activityPanel)

	// Footer
	footer := d.renderFooter()

	// Combine all sections vertically
	result := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)

	base := lipgloss.NewStyle().
		Background(t.Background).
		Width(d.width).
		Height(d.height).
		Render(result)

	// Overlay dialog if visible
	if d.showDialog && d.newJobDialog != nil && d.newJobDialog.Visible() {
		return d.renderWithDialogOverlay(base, t)
	}

	// Overlay template selector if visible
	if d.showTemplates && d.templateSelector != nil && d.templateSelector.Visible() {
		return d.renderWithTemplateSelectorOverlay(base, t)
	}

	return base
}

func (d *Dashboard) renderWithDialogOverlay(baseView string, t theme.Theme) string {
	// Get the raw dialog view (not centered yet)
	dialogView := d.newJobDialog.View()
	if dialogView == "" {
		return baseView
	}
	return d.overlayOnBase(baseView, dialogView, t)
}

func (d *Dashboard) renderWithTemplateSelectorOverlay(baseView string, t theme.Theme) string {
	// Get the template selector view
	selectorView := d.templateSelector.View()
	if selectorView == "" {
		return baseView
	}
	return d.overlayOnBase(baseView, selectorView, t)
}

func (d *Dashboard) overlayOnBase(baseView, overlayView string, t theme.Theme) string {
	overlayWidth := lipgloss.Width(overlayView)
	overlayHeight := lipgloss.Height(overlayView)

	// Calculate centering position
	padLeft := (d.width - overlayWidth) / 2
	padTop := (d.height - overlayHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	// Split views into lines
	baseLines := strings.Split(baseView, "\n")
	overlayLines := strings.Split(overlayView, "\n")

	// Ensure we have enough base lines
	for len(baseLines) < d.height {
		baseLines = append(baseLines, strings.Repeat(" ", d.width))
	}

	// Overlay onto the base view at the calculated position
	// Use ANSI-aware string manipulation to avoid corrupting escape sequences
	result := make([]string, len(baseLines))
	for i := 0; i < len(baseLines); i++ {
		overlayLineIdx := i - padTop
		if overlayLineIdx >= 0 && overlayLineIdx < len(overlayLines) {
			// This line has overlay content - merge it using ANSI-aware functions
			baseLine := baseLines[i]
			overlayLine := overlayLines[overlayLineIdx]
			overlayLineWidth := lipgloss.Width(overlayLine)

			// Build merged line: base prefix + overlay + base suffix
			// ansi.Truncate safely cuts ANSI strings without breaking escape sequences
			prefix := ansi.Truncate(baseLine, padLeft, "")
			// Pad prefix if base line was shorter than padLeft
			if lipgloss.Width(prefix) < padLeft {
				prefix += strings.Repeat(" ", padLeft-lipgloss.Width(prefix))
			}

			// Get suffix from base line after the overlay position
			// ansi.Cut(s, left, right) extracts characters from position left to right
			suffixStart := padLeft + overlayLineWidth
			suffix := ""
			baseLineWidth := lipgloss.Width(baseLine)
			if suffixStart < baseLineWidth {
				suffix = ansi.Cut(baseLine, suffixStart, baseLineWidth)
			}

			result[i] = prefix + overlayLine + suffix
		} else {
			result[i] = baseLines[i]
		}
	}

	return lipgloss.NewStyle().
		Background(t.Background).
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
		{"t", "templates"},
		{"q", "quit"},
	}

	// Add reassign option when a failed/cancelled job is selected
	if d.CanReassignSelectedJob() {
		keys = append([]struct {
			key  string
			desc string
		}{{"R", "reassign job"}}, keys...)
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
	var borderColor lipgloss.Color

	if focused {
		borderColor = t.BorderActive
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)
		titleStyle = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
	} else {
		borderColor = t.Border
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)
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

	// Insert title into top border using shared helper
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
	if d.showDialog && d.newJobDialog != nil && d.newJobDialog.Visible() {
		return true
	}
	if d.showTemplates && d.templateSelector != nil && d.templateSelector.Visible() {
		return true
	}
	return false
}

// SetOnCreateJob sets the callback for when a job is created.
func (d *Dashboard) SetOnCreateJob(fn func(description string)) {
	d.onCreateJob = fn
}

// SetOnReassignJob sets the callback for when a job is reassigned.
func (d *Dashboard) SetOnReassignJob(fn func(jobID string)) {
	d.onReassignJob = fn
}

// CanReassignSelectedJob returns true if the selected job can be reassigned.
func (d *Dashboard) CanReassignSelectedJob() bool {
	if d.focus != FocusJobs {
		return false
	}
	return d.jobList.CanReassignSelected()
}

// ReassignSelectedJob triggers reassignment of the selected job.
func (d *Dashboard) ReassignSelectedJob() {
	if d.onReassignJob == nil {
		return
	}
	selected := d.jobList.Selected()
	if selected == nil {
		return
	}
	if selected.Status != "failed" && selected.Status != "cancelled" {
		return
	}
	d.onReassignJob(selected.ID)
}

// ShowNewJobDialog shows the new job dialog.
func (d *Dashboard) ShowNewJobDialog() {
	d.showDialog = true
	d.newJobDialog.Show()
}

// HandleDialogKey handles key input for dialogs. Returns the action if dialog submits.
func (d *Dashboard) HandleDialogKey(key string) string {
	if d.showDialog && d.newJobDialog != nil && d.newJobDialog.Visible() {
		action := d.newJobDialog.HandleKey(key)
		if action == "cancel" {
			d.newJobDialog.Hide()
			d.showDialog = false
			return ""
		}
		if action == "create" && d.onCreateJob != nil {
			description := d.newJobDialog.GetInputValue()
			if description != "" {
				d.onCreateJob(description)
			}
			d.newJobDialog.Hide()
			d.showDialog = false
			return ""
		}
		return action
	}
	return ""
}

// GetNewJobDescription returns the job description from the new job dialog.
func (d *Dashboard) GetNewJobDescription() string {
	if d.newJobDialog != nil {
		return d.newJobDialog.GetInputValue()
	}
	return ""
}

// HideNewJobDialog hides the new job dialog and resets it.
func (d *Dashboard) HideNewJobDialog() {
	if d.newJobDialog != nil {
		d.newJobDialog.Hide()
	}
	d.showDialog = false
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
	if d.showDialog && d.newJobDialog != nil {
		d.newJobDialog.Hide()
		d.showDialog = false
	}
	if d.showTemplates && d.templateSelector != nil {
		d.templateSelector.Hide()
		d.showTemplates = false
	}
}

// SetOnUseTemplate sets the callback for when a template is used to create a job.
func (d *Dashboard) SetOnUseTemplate(fn func(templateID string, variables map[string]string)) {
	d.onUseTemplate = fn
}

// SetTemplates sets the available templates for the template selector.
func (d *Dashboard) SetTemplates(templates []component.TemplateItem) {
	d.templateSelector.SetTemplates(templates)
}

// ShowTemplateSelector shows the template selection dialog.
func (d *Dashboard) ShowTemplateSelector() {
	d.showTemplates = true
	d.templateSelector.SetSize(80, 25)
	d.templateSelector.Show()
}

// HandleTemplateSelectorKey handles key input for the template selector.
func (d *Dashboard) HandleTemplateSelectorKey(key string) string {
	if d.showTemplates && d.templateSelector != nil && d.templateSelector.Visible() {
		action := d.templateSelector.HandleKey(key)
		if action == "cancel" || action == "select" {
			d.showTemplates = false
		}
		return action
	}
	return ""
}

// IsTemplateMode returns true if the template selector is visible.
func (d *Dashboard) IsTemplateMode() bool {
	return d.showTemplates && d.templateSelector != nil && d.templateSelector.Visible()
}
