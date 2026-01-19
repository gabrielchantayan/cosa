// Package component provides reusable TUI components.
package component

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/protocol"
	"cosa/internal/tui/styles"
)

// WorkerList displays a list of workers.
type WorkerList struct {
	workers  []protocol.WorkerInfo
	selected int
	focused  bool
	width    int
	height   int
	styles   styles.Styles
}

// NewWorkerList creates a new worker list component.
func NewWorkerList() *WorkerList {
	return &WorkerList{
		styles: styles.New(),
	}
}

// SetWorkers updates the worker list, sorted alphabetically by name.
func (w *WorkerList) SetWorkers(workers []protocol.WorkerInfo) {
	// Sort workers alphabetically by name
	sort.Slice(workers, func(i, j int) bool {
		return strings.ToLower(workers[i].Name) < strings.ToLower(workers[j].Name)
	})
	w.workers = workers
	if w.selected >= len(workers) {
		w.selected = max(0, len(workers)-1)
	}
}

// SetSize sets the component dimensions.
func (w *WorkerList) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// SetFocused sets the focus state.
func (w *WorkerList) SetFocused(focused bool) {
	w.focused = focused
}

// MoveUp moves selection up.
func (w *WorkerList) MoveUp() {
	if w.selected > 0 {
		w.selected--
	}
}

// MoveDown moves selection down.
func (w *WorkerList) MoveDown() {
	if w.selected < len(w.workers)-1 {
		w.selected++
	}
}

// Selected returns the currently selected worker.
func (w *WorkerList) Selected() *protocol.WorkerInfo {
	if w.selected >= 0 && w.selected < len(w.workers) {
		return &w.workers[w.selected]
	}
	return nil
}

// View renders the worker list.
func (w *WorkerList) View() string {
	if len(w.workers) == 0 {
		return w.styles.TextMuted.Render("No workers")
	}

	var lines []string
	contentHeight := w.height
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Calculate visible range based on selection
	start := 0
	if w.selected >= contentHeight {
		start = w.selected - contentHeight + 1
	}
	end := min(start+contentHeight, len(w.workers))

	for i := start; i < end; i++ {
		worker := w.workers[i]
		line := w.renderWorkerLine(worker, i == w.selected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (w *WorkerList) renderWorkerLine(worker protocol.WorkerInfo, selected bool) string {
	// Status indicator
	var statusIcon string
	switch worker.Status {
	case "idle":
		statusIcon = "○"
	case "working":
		statusIcon = "●"
	case "reviewing":
		statusIcon = "◐"
	case "error":
		statusIcon = "✗"
	default:
		statusIcon = "○"
	}

	// Role badge
	roleStyle := w.styles.RoleStyle(worker.Role)
	roleAbbrev := worker.Role
	if len(roleAbbrev) > 3 {
		roleAbbrev = roleAbbrev[:3]
	}
	role := roleStyle.Render(fmt.Sprintf("[%s]", roleAbbrev))

	// Status style
	statusStyle := w.styles.StatusStyle(worker.Status)
	status := statusStyle.Render(statusIcon)

	// Name
	name := worker.Name
	if len(name) > 12 {
		name = name[:12]
	}

	// Build line
	content := fmt.Sprintf("%s %s %-12s", status, role, name)

	// Apply selection style
	lineWidth := w.width - 2
	if selected && w.focused {
		return w.styles.ListItemActive.Width(lineWidth).Render(content)
	} else if selected {
		return w.styles.ListItemSelected.Width(lineWidth).Render(content)
	}
	return w.styles.ListItem.Width(lineWidth).Render(content)
}

// JobList displays a list of jobs.
type JobList struct {
	jobs     []protocol.JobInfo
	selected int
	focused  bool
	width    int
	height   int
	styles   styles.Styles
}

// NewJobList creates a new job list component.
func NewJobList() *JobList {
	return &JobList{
		styles: styles.New(),
	}
}

// jobStatusOrder defines the sort order for job statuses.
// Active/in-progress statuses come first, then pending, then terminal states.
var jobStatusOrder = map[string]int{
	"running":   0,
	"queued":    1,
	"pending":   2,
	"completed": 3,
	"failed":    4,
	"cancelled": 5,
}

// SetJobs updates the job list, sorted by status, then priority, then description.
func (j *JobList) SetJobs(jobs []protocol.JobInfo) {
	// Sort jobs: 1) by status order, 2) by priority (lower = higher), 3) alphabetically by description
	sort.Slice(jobs, func(i, k int) bool {
		// Compare by status first
		statusI := jobStatusOrder[jobs[i].Status]
		statusK := jobStatusOrder[jobs[k].Status]
		if statusI != statusK {
			return statusI < statusK
		}
		// Then by priority (lower number = higher priority)
		if jobs[i].Priority != jobs[k].Priority {
			return jobs[i].Priority < jobs[k].Priority
		}
		// Finally alphabetically by description
		return strings.ToLower(jobs[i].Description) < strings.ToLower(jobs[k].Description)
	})
	j.jobs = jobs
	if j.selected >= len(jobs) {
		j.selected = max(0, len(jobs)-1)
	}
}

// SetSize sets the component dimensions.
func (j *JobList) SetSize(width, height int) {
	j.width = width
	j.height = height
}

// SetFocused sets the focus state.
func (j *JobList) SetFocused(focused bool) {
	j.focused = focused
}

// MoveUp moves selection up.
func (j *JobList) MoveUp() {
	if j.selected > 0 {
		j.selected--
	}
}

// MoveDown moves selection down.
func (j *JobList) MoveDown() {
	if j.selected < len(j.jobs)-1 {
		j.selected++
	}
}

// Selected returns the currently selected job.
func (j *JobList) Selected() *protocol.JobInfo {
	if j.selected >= 0 && j.selected < len(j.jobs) {
		return &j.jobs[j.selected]
	}
	return nil
}

// View renders the job list.
func (j *JobList) View() string {
	if len(j.jobs) == 0 {
		return j.styles.TextMuted.Render("No jobs")
	}

	var lines []string
	contentHeight := j.height
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := 0
	if j.selected >= contentHeight {
		start = j.selected - contentHeight + 1
	}
	end := min(start+contentHeight, len(j.jobs))

	for i := start; i < end; i++ {
		job := j.jobs[i]
		line := j.renderJobLine(job, i == j.selected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (j *JobList) renderJobLine(job protocol.JobInfo, selected bool) string {
	// Status indicator
	var statusIcon string
	switch job.Status {
	case "pending":
		statusIcon = "○"
	case "queued":
		statusIcon = "◔"
	case "running":
		statusIcon = "●"
	case "completed":
		statusIcon = "✓"
	case "failed":
		statusIcon = "✗"
	case "cancelled":
		statusIcon = "⊘"
	default:
		statusIcon = "○"
	}

	statusStyle := j.styles.StatusStyle(job.Status)
	status := statusStyle.Render(statusIcon)

	// Priority
	priority := fmt.Sprintf("P%d", job.Priority)

	// Description
	desc := job.Description
	maxLen := j.width - 12
	if len(desc) > maxLen {
		desc = desc[:maxLen-2] + ".."
	}

	content := fmt.Sprintf("%s %s %s", status, priority, desc)

	lineWidth := j.width - 2
	if selected && j.focused {
		return j.styles.ListItemActive.Width(lineWidth).Render(content)
	} else if selected {
		return j.styles.ListItemSelected.Width(lineWidth).Render(content)
	}
	return j.styles.ListItem.Width(lineWidth).Render(content)
}

// Activity displays activity feed.
type Activity struct {
	items  []ActivityItem
	width  int
	height int
	styles styles.Styles
}

// ActivityItem represents a single activity entry.
type ActivityItem struct {
	Time    string
	Worker  string
	Message string
}

// NewActivity creates a new activity component.
func NewActivity() *Activity {
	return &Activity{
		styles: styles.New(),
	}
}

// AddItem adds an activity item.
func (a *Activity) AddItem(item ActivityItem) {
	a.items = append(a.items, item)
	// Keep last 100 items
	if len(a.items) > 100 {
		a.items = a.items[1:]
	}
}

// SetSize sets the component dimensions.
func (a *Activity) SetSize(width, height int) {
	a.width = width
	a.height = height
}

// View renders the activity feed.
func (a *Activity) View() string {
	if len(a.items) == 0 {
		return a.styles.TextMuted.Render("No activity")
	}

	var lines []string
	contentHeight := a.height
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Show most recent items (scroll to bottom)
	start := max(0, len(a.items)-contentHeight)
	for i := start; i < len(a.items); i++ {
		item := a.items[i]
		line := a.renderActivityLine(item)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (a *Activity) renderActivityLine(item ActivityItem) string {
	time := a.styles.ActivityTime.Render(item.Time)
	worker := ""
	if item.Worker != "" {
		worker = a.styles.ActivityWorker.Render(fmt.Sprintf("[%s] ", item.Worker))
	}
	message := a.styles.ActivityMessage.Render(item.Message)

	line := fmt.Sprintf("%s %s%s", time, worker, message)

	// Truncate if too long
	if lipgloss.Width(line) > a.width-2 {
		// Simple truncation
		maxLen := a.width - 5
		if len(line) > maxLen {
			line = line[:maxLen] + "..."
		}
	}

	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
