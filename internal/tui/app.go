// Package tui provides the Bubble Tea TUI for Cosa.
package tui

import (
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cosa/internal/daemon"
	"cosa/internal/ledger"
	"cosa/internal/protocol"
	"cosa/internal/tui/component"
	"cosa/internal/tui/page"
	"cosa/internal/tui/styles"
)

// App is the root Bubble Tea model.
type App struct {
	client    *daemon.Client
	dashboard *page.Dashboard
	chat      *page.Chat
	styles    styles.Styles
	width     int
	height    int
	err       error
	quitting  bool

	// Page routing
	activePage string // "dashboard" or "chat"

	// Chat state
	chatStarted bool
	workers     []protocol.WorkerInfo
	jobs        []protocol.JobInfo
}

// Messages

type tickMsg time.Time

type statusMsg *protocol.StatusResult
type workersMsg []protocol.WorkerInfo
type jobsMsg []protocol.JobInfo
type templatesMsg []component.TemplateItem
type eventMsg ledger.Event
type errMsg error

// Chat messages
type chatStartedMsg struct {
	sessionID string
	greeting  string
	err       error
}
type chatResponseMsg struct {
	response string
	err      error
}
type chatLoadingTickMsg struct{}

// NewApp creates a new TUI application.
func NewApp(client *daemon.Client) *App {
	app := &App{
		client:     client,
		dashboard:  page.NewDashboard(),
		chat:       page.NewChat(),
		styles:     styles.New(),
		activePage: "dashboard",
	}

	// Set up dashboard callbacks
	app.dashboard.SetOnCreateJob(func(description string) {
		app.createJob(description)
	})
	app.dashboard.SetOnReassignJob(func(jobID string) {
		app.reassignJob(jobID)
	})
	app.dashboard.SetOnUseTemplate(func(templateID string, variables map[string]string) {
		app.useTemplate(templateID, variables)
	})

	return app
}

// Init initializes the app.
func (a *App) Init() tea.Cmd {
	// Subscribe to events
	if a.client != nil {
		a.client.Subscribe([]string{"*"})
		a.client.OnNotification(func(r *protocol.Request) {
			// Handle notifications in the background
		})
	}

	return tea.Batch(
		a.fetchStatus,
		a.fetchWorkers,
		a.fetchJobs,
		a.fetchTemplates,
		a.tickEvery(time.Second),
	)
}

// Update handles messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKey(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.dashboard.SetSize(msg.Width, msg.Height)
		a.chat.SetSize(msg.Width, msg.Height)
		return a, nil

	case tickMsg:
		cmds := []tea.Cmd{
			a.fetchStatus,
			a.fetchWorkers,
			a.fetchJobs,
			a.tickEvery(time.Second),
		}
		// Add loading tick for chat if loading
		if a.activePage == "chat" && a.chat.IsLoading() {
			cmds = append(cmds, a.chatLoadingTick())
		}
		return a, tea.Batch(cmds...)

	case statusMsg:
		a.dashboard.SetStatus(msg)
		return a, nil

	case workersMsg:
		a.workers = msg
		a.dashboard.SetWorkers(msg)
		// Update chat sidebar
		a.chat.SetWorkers(msg)
		return a, nil

	case jobsMsg:
		a.jobs = msg
		a.dashboard.SetJobs(msg)
		// Update chat job counts
		a.updateChatJobCounts()
		return a, nil

	case templatesMsg:
		a.dashboard.SetTemplates(msg)
		return a, nil

	case eventMsg:
		a.handleEvent(ledger.Event(msg))
		return a, nil

	case errMsg:
		a.err = msg
		return a, nil

	// Chat messages
	case chatStartedMsg:
		if msg.err != nil {
			a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Chat error: %v", msg.err))
			a.activePage = "dashboard"
			return a, nil
		}
		a.chatStarted = true
		a.chat.SetSessionID(msg.sessionID, false)
		if msg.greeting != "" {
			a.chat.AddMessage("assistant", msg.greeting)
		}
		a.chat.SetLoading(false)
		return a, nil

	case chatResponseMsg:
		a.chat.SetLoading(false)
		if msg.err != nil {
			a.chat.AddMessage("assistant", fmt.Sprintf("Error: %v", msg.err))
		} else {
			a.chat.AddMessage("assistant", msg.response)
		}
		return a, nil

	case chatLoadingTickMsg:
		a.chat.TickLoading()
		if a.chat.IsLoading() {
			return a, a.chatLoadingTick()
		}
		return a, nil
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle chat page
	if a.activePage == "chat" {
		return a.handleChatKey(msg)
	}

	// Handle template selector mode
	if a.dashboard.IsTemplateMode() {
		a.dashboard.HandleTemplateSelectorKey(msg.String())
		return a, nil
	}

	// Handle input mode first (dialogs)
	if a.dashboard.IsInputMode() {
		a.dashboard.HandleDialogKey(msg.String())
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		return a, tea.Quit

	case "c":
		// Open chat with The Underboss
		return a.openChat()

	case "tab":
		a.dashboard.NextFocus()
		return a, nil

	case "shift+tab":
		a.dashboard.PrevFocus()
		return a, nil

	case "j", "down", "k", "up":
		a.dashboard.HandleKey(msg.String())
		return a, nil

	case "h", "left":
		if a.dashboard.Focus() != page.FocusWorkers {
			a.dashboard.PrevFocus()
		}
		return a, nil

	case "l", "right":
		if a.dashboard.Focus() == page.FocusWorkers {
			a.dashboard.NextFocus()
		}
		return a, nil

	case "1":
		a.dashboard.SetFocus(page.FocusWorkers)
		return a, nil

	case "2":
		a.dashboard.SetFocus(page.FocusJobs)
		return a, nil

	case "3":
		a.dashboard.SetFocus(page.FocusActivity)
		return a, nil

	case "n":
		// New job dialog
		a.dashboard.ShowNewJobDialog()
		return a, nil

	case "t":
		// Template selector
		a.dashboard.ShowTemplateSelector()
		return a, nil

	case "o":
		// New operation dialog
		a.dashboard.ShowNewOperationDialog()
		return a, nil

	case "/":
		// Search mode
		a.dashboard.ShowSearch()
		return a, nil

	case ":":
		// Command palette
		a.dashboard.ShowCommandPalette()
		return a, nil

	case "?":
		// Help overlay
		a.dashboard.ToggleHelp()
		return a, nil

	case "enter":
		// Select current item (open worker detail, etc.)
		a.dashboard.SelectCurrent()
		return a, nil

	case "esc":
		// Close dialogs/overlays
		a.dashboard.CloseOverlay()
		return a, nil

	case "r":
		// Refresh data
		return a, tea.Batch(
			a.fetchStatus,
			a.fetchWorkers,
			a.fetchJobs,
		)

	case "R":
		// Reassign failed job (capital R to avoid conflict with refresh)
		if a.dashboard.CanReassignSelectedJob() {
			a.dashboard.ReassignSelectedJob()
		}
		return a, nil
	}

	return a, nil
}

func (a *App) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	result := a.chat.HandleKey(msg.String())

	switch result {
	case "exit":
		a.activePage = "dashboard"
		return a, nil
	case "send":
		input := a.chat.GetInput()
		if input != "" {
			a.chat.AddMessage("user", input)
			a.chat.SetLoading(true)
			return a, tea.Batch(
				a.sendChatMessage(input),
				a.chatLoadingTick(),
			)
		}
	case "cancel":
		a.chat.SetLoading(false)
		// TODO: Actually cancel the request if possible
	}

	return a, nil
}

func (a *App) handleEvent(event ledger.Event) {
	timeStr := event.Timestamp.Format("15:04:05")
	var worker, message string

	switch event.Type {
	case ledger.EventWorkerAdded:
		var data ledger.WorkerEventData
		json.Unmarshal(event.Data, &data)
		worker = data.Name
		message = fmt.Sprintf("Worker added (%s)", data.Role)

	case ledger.EventWorkerStarted:
		var data ledger.WorkerEventData
		json.Unmarshal(event.Data, &data)
		worker = data.Name
		message = "Worker started"

	case ledger.EventJobCreated:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		message = fmt.Sprintf("Job created: %s", truncate(data.Description, 30))

	case ledger.EventJobQueued:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		worker = data.WorkerName
		message = fmt.Sprintf("Picked up job: %s", truncate(data.Description, 30))

	case ledger.EventJobStarted:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		worker = data.WorkerName
		message = fmt.Sprintf("Started job: %s", truncate(data.Description, 30))

	case ledger.EventJobCompleted:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		worker = data.WorkerName
		message = fmt.Sprintf("Completed job: %s", truncate(data.Description, 30))

	case ledger.EventJobFailed:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		worker = data.WorkerName
		message = fmt.Sprintf("Job failed: %s", data.Error)

	case ledger.EventJobCancelled:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		worker = data.WorkerName
		message = fmt.Sprintf("Job cancelled: %s", truncate(data.Description, 30))

	default:
		message = string(event.Type)
	}

	a.dashboard.AddActivity(timeStr, worker, message)
}

// View renders the app.
func (a *App) View() string {
	if a.quitting {
		return "Goodbye.\n"
	}

	if a.err != nil {
		return fmt.Sprintf("Error: %v\n", a.err)
	}

	if a.activePage == "chat" {
		return a.chat.View()
	}

	return a.dashboard.View()
}

// Commands

func (a *App) fetchStatus() tea.Msg {
	if a.client == nil {
		return nil
	}

	status, err := a.client.Status()
	if err != nil {
		return errMsg(err)
	}
	return statusMsg(status)
}

func (a *App) fetchWorkers() tea.Msg {
	if a.client == nil {
		return nil
	}

	resp, err := a.client.Call(protocol.MethodWorkerList, nil)
	if err != nil {
		return errMsg(err)
	}

	if resp.Error != nil {
		return nil
	}

	var workers []protocol.WorkerInfo
	json.Unmarshal(resp.Result, &workers)
	return workersMsg(workers)
}

func (a *App) fetchJobs() tea.Msg {
	if a.client == nil {
		return nil
	}

	resp, err := a.client.Call(protocol.MethodJobList, nil)
	if err != nil {
		return errMsg(err)
	}

	if resp.Error != nil {
		return nil
	}

	var jobs []protocol.JobInfo
	json.Unmarshal(resp.Result, &jobs)
	return jobsMsg(jobs)
}

func (a *App) tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) fetchTemplates() tea.Msg {
	if a.client == nil {
		return nil
	}

	resp, err := a.client.Call(protocol.MethodTemplateList, nil)
	if err != nil {
		return nil // Don't report error for templates
	}

	if resp.Error != nil {
		return nil
	}

	var result protocol.TemplateListResult
	json.Unmarshal(resp.Result, &result)

	// Convert to component.TemplateItem
	items := make([]component.TemplateItem, len(result.Templates))
	for i, t := range result.Templates {
		vars := make([]component.TemplateVariable, len(t.Variables))
		for j, v := range t.Variables {
			vars[j] = component.TemplateVariable{
				Name:        v.Name,
				Description: v.Description,
				Required:    v.Required,
				Default:     v.Default,
			}
		}
		items[i] = component.TemplateItem{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Type:        t.Type,
			Variables:   vars,
		}
	}
	return templatesMsg(items)
}

func (a *App) createJob(description string) {
	if a.client == nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", "Error: No connection to daemon")
		return
	}

	params := protocol.JobAddParams{
		Description: description,
		Priority:    3, // Default priority
	}

	resp, err := a.client.Call(protocol.MethodJobAdd, params)
	if err != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error creating job: %v", err))
		return
	}

	if resp.Error != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error: %s", resp.Error.Message))
		return
	}

	a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Job created: %s", truncate(description, 30)))
}

func (a *App) reassignJob(jobID string) {
	if a.client == nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", "Error: No connection to daemon")
		return
	}

	params := protocol.JobReassignParams{
		JobID: jobID,
	}

	resp, err := a.client.Call(protocol.MethodJobReassign, params)
	if err != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error reassigning job: %v", err))
		return
	}

	if resp.Error != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error: %s", resp.Error.Message))
		return
	}

	a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Job reassigned: %s", jobID[:8]))
}

func (a *App) useTemplate(templateID string, variables map[string]string) {
	if a.client == nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", "Error: No connection to daemon")
		return
	}

	params := protocol.TemplateUseParams{
		TemplateID: templateID,
		Variables:  variables,
	}

	resp, err := a.client.Call(protocol.MethodTemplateUse, params)
	if err != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error using template: %v", err))
		return
	}

	if resp.Error != nil {
		a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Error: %s", resp.Error.Message))
		return
	}

	var result protocol.TemplateUseResult
	json.Unmarshal(resp.Result, &result)

	a.dashboard.AddActivity(time.Now().Format("15:04:05"), "", fmt.Sprintf("Job created from template: %s", result.Job.ID[:8]))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// Chat commands

func (a *App) openChat() (tea.Model, tea.Cmd) {
	a.activePage = "chat"
	a.chat.SetSize(a.width, a.height)
	a.chat.SetWorkers(a.workers)
	a.updateChatJobCounts()

	// If chat not started, start it
	if !a.chatStarted {
		a.chat.SetLoading(true)
		return a, tea.Batch(
			a.startChat(),
			a.chatLoadingTick(),
		)
	}

	return a, nil
}

func (a *App) startChat() tea.Cmd {
	return func() tea.Msg {
		if a.client == nil {
			return chatStartedMsg{err: fmt.Errorf("no connection to daemon")}
		}

		resp, err := a.client.Call(protocol.MethodChatStart, nil)
		if err != nil {
			return chatStartedMsg{err: err}
		}

		if resp.Error != nil {
			return chatStartedMsg{err: fmt.Errorf("%s", resp.Error.Message)}
		}

		var result protocol.ChatStartResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return chatStartedMsg{err: err}
		}

		return chatStartedMsg{
			sessionID: result.SessionID,
			greeting:  result.Greeting,
		}
	}
}

func (a *App) sendChatMessage(message string) tea.Cmd {
	return func() tea.Msg {
		if a.client == nil {
			return chatResponseMsg{err: fmt.Errorf("no connection to daemon")}
		}

		resp, err := a.client.Call(protocol.MethodChatSend, protocol.ChatSendParams{
			Message: message,
		})
		if err != nil {
			return chatResponseMsg{err: err}
		}

		if resp.Error != nil {
			return chatResponseMsg{err: fmt.Errorf("%s", resp.Error.Message)}
		}

		var result protocol.ChatSendResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return chatResponseMsg{err: err}
		}

		return chatResponseMsg{response: result.Response}
	}
}

func (a *App) chatLoadingTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return chatLoadingTickMsg{}
	})
}

func (a *App) updateChatJobCounts() {
	var pending, inProgress int
	for _, j := range a.jobs {
		switch j.Status {
		case "pending", "queued":
			pending++
		case "running":
			inProgress++
		}
	}
	a.chat.SetJobCounts(page.JobCounts{
		Pending:    pending,
		InProgress: inProgress,
	})
}

// Run starts the TUI.
func Run(client *daemon.Client) error {
	app := NewApp(client)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
