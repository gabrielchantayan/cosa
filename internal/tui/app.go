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
	"cosa/internal/tui/page"
	"cosa/internal/tui/styles"
)

// App is the root Bubble Tea model.
type App struct {
	client    *daemon.Client
	dashboard *page.Dashboard
	styles    styles.Styles
	width     int
	height    int
	err       error
	quitting  bool
}

// Messages

type tickMsg time.Time

type statusMsg *protocol.StatusResult
type workersMsg []protocol.WorkerInfo
type jobsMsg []protocol.JobInfo
type eventMsg ledger.Event
type errMsg error

// NewApp creates a new TUI application.
func NewApp(client *daemon.Client) *App {
	return &App{
		client:    client,
		dashboard: page.NewDashboard(),
		styles:    styles.New(),
	}
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
		return a, nil

	case tickMsg:
		return a, tea.Batch(
			a.fetchStatus,
			a.fetchWorkers,
			a.fetchJobs,
			a.tickEvery(time.Second),
		)

	case statusMsg:
		a.dashboard.SetStatus(msg)
		return a, nil

	case workersMsg:
		a.dashboard.SetWorkers(msg)
		return a, nil

	case jobsMsg:
		a.dashboard.SetJobs(msg)
		return a, nil

	case eventMsg:
		a.handleEvent(ledger.Event(msg))
		return a, nil

	case errMsg:
		a.err = msg
		return a, nil
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle input mode first
	if a.dashboard.IsInputMode() {
		a.dashboard.HandleKey(msg.String())
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		return a, tea.Quit

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

	case ledger.EventJobStarted:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		message = fmt.Sprintf("Job started: %s", truncate(data.Description, 30))

	case ledger.EventJobCompleted:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		message = fmt.Sprintf("Job completed: %s", truncate(data.Description, 30))

	case ledger.EventJobFailed:
		var data ledger.JobEventData
		json.Unmarshal(event.Data, &data)
		message = fmt.Sprintf("Job failed: %s", data.Error)

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// Run starts the TUI.
func Run(client *daemon.Client) error {
	app := NewApp(client)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
