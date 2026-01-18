// Package component provides UI components for the TUI.
package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// Command represents a command in the palette.
type Command struct {
	Name        string
	Description string
	Shortcut    string
	Action      string
}

// CommandPalette is a command palette component.
type CommandPalette struct {
	commands  []Command
	filtered  []Command
	input     *Input
	selected  int
	visible   bool
	width     int
	maxHeight int
}

// NewCommandPalette creates a new command palette.
func NewCommandPalette() *CommandPalette {
	cp := &CommandPalette{
		commands:  defaultCommands(),
		input:     NewInput(),
		width:     60,
		maxHeight: 15,
	}
	cp.input.SetPlaceholder("Type a command...")
	cp.filtered = cp.commands
	return cp
}

func defaultCommands() []Command {
	return []Command{
		{Name: "New Job", Description: "Create a new job", Shortcut: "n", Action: "new_job"},
		{Name: "New Operation", Description: "Create a new operation", Shortcut: "o", Action: "new_operation"},
		{Name: "Add Worker", Description: "Add a new worker", Shortcut: "a", Action: "add_worker"},
		{Name: "Refresh", Description: "Refresh data", Shortcut: "r", Action: "refresh"},
		{Name: "Search", Description: "Search workers/jobs", Shortcut: "/", Action: "search"},
		{Name: "Help", Description: "Show help", Shortcut: "?", Action: "help"},
		{Name: "Quit", Description: "Exit the application", Shortcut: "q", Action: "quit"},
		{Name: "Worker List", Description: "Show all workers", Shortcut: "1", Action: "focus_workers"},
		{Name: "Job List", Description: "Show all jobs", Shortcut: "2", Action: "focus_jobs"},
		{Name: "Activity", Description: "Show activity log", Shortcut: "3", Action: "focus_activity"},
	}
}

// SetSize sets the palette dimensions.
func (cp *CommandPalette) SetSize(width, maxHeight int) {
	cp.width = width
	cp.maxHeight = maxHeight
	cp.input.SetWidth(width - 6)
}

// Show makes the palette visible.
func (cp *CommandPalette) Show() {
	cp.visible = true
	cp.input.Reset()
	cp.input.Focus()
	cp.selected = 0
	cp.filtered = cp.commands
}

// Hide hides the palette.
func (cp *CommandPalette) Hide() {
	cp.visible = false
	cp.input.Reset()
}

// Visible returns true if the palette is visible.
func (cp *CommandPalette) Visible() bool {
	return cp.visible
}

// HandleKey handles key presses.
func (cp *CommandPalette) HandleKey(key string) string {
	switch key {
	case "esc":
		cp.Hide()
		return "cancel"
	case "enter":
		if cp.selected >= 0 && cp.selected < len(cp.filtered) {
			action := cp.filtered[cp.selected].Action
			cp.Hide()
			return action
		}
		return ""
	case "up", "ctrl+p":
		if cp.selected > 0 {
			cp.selected--
		}
		return ""
	case "down", "ctrl+n":
		if cp.selected < len(cp.filtered)-1 {
			cp.selected++
		}
		return ""
	default:
		cp.input.HandleKey(key)
		cp.filterCommands()
		return ""
	}
}

func (cp *CommandPalette) filterCommands() {
	query := strings.ToLower(cp.input.Value())
	if query == "" {
		cp.filtered = cp.commands
		cp.selected = 0
		return
	}

	cp.filtered = make([]Command, 0)
	for _, cmd := range cp.commands {
		name := strings.ToLower(cmd.Name)
		desc := strings.ToLower(cmd.Description)
		if strings.Contains(name, query) || strings.Contains(desc, query) {
			cp.filtered = append(cp.filtered, cmd)
		}
	}

	if cp.selected >= len(cp.filtered) {
		cp.selected = len(cp.filtered) - 1
	}
	if cp.selected < 0 {
		cp.selected = 0
	}
}

// View renders the command palette.
func (cp *CommandPalette) View() string {
	if !cp.visible {
		return ""
	}

	t := theme.Current

	// Input field
	inputView := cp.input.View()

	// Command list
	listHeight := cp.maxHeight - 4
	if listHeight < 3 {
		listHeight = 3
	}
	if listHeight > len(cp.filtered) {
		listHeight = len(cp.filtered)
	}

	var cmdLines []string
	for i, cmd := range cp.filtered {
		if len(cmdLines) >= listHeight {
			break
		}

		line := cp.renderCommandLine(cmd, i == cp.selected)
		cmdLines = append(cmdLines, line)
	}

	if len(cp.filtered) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 2).
			Render("No commands found")
		cmdLines = append(cmdLines, noResults)
	}

	commandList := strings.Join(cmdLines, "\n")

	// Container
	content := lipgloss.JoinVertical(lipgloss.Left,
		inputView,
		commandList,
	)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Background(t.Background).
		Width(cp.width).
		Padding(1)

	return containerStyle.Render(content)
}

func (cp *CommandPalette) renderCommandLine(cmd Command, selected bool) string {
	t := theme.Current

	var nameStyle, descStyle, shortcutStyle lipgloss.Style

	if selected {
		nameStyle = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		descStyle = lipgloss.NewStyle().
			Foreground(t.Text)
		shortcutStyle = lipgloss.NewStyle().
			Foreground(t.Secondary).
			Bold(true)
	} else {
		nameStyle = lipgloss.NewStyle().
			Foreground(t.Text)
		descStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
		shortcutStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
	}

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().
			Foreground(t.Primary).
			Render("▸ ")
	}

	// Build line
	name := nameStyle.Render(cmd.Name)
	desc := descStyle.Render(" - " + cmd.Description)
	shortcut := shortcutStyle.Render(" [" + cmd.Shortcut + "]")

	// Truncate if needed
	available := cp.width - 10
	combined := name + desc
	if lipgloss.Width(combined) > available {
		// Truncate description
		descLen := available - lipgloss.Width(name) - 4
		if descLen > 0 {
			truncDesc := cmd.Description
			if len(truncDesc) > descLen {
				truncDesc = truncDesc[:descLen-2] + ".."
			}
			desc = descStyle.Render(" - " + truncDesc)
		} else {
			desc = ""
		}
	}

	return indicator + name + desc + shortcut
}

// CenterIn returns the palette centered in the given dimensions.
func (cp *CommandPalette) CenterIn(screenWidth, screenHeight int) string {
	palette := cp.View()
	if palette == "" {
		return ""
	}

	paletteWidth := lipgloss.Width(palette)

	// Center horizontally, position near top
	padLeft := (screenWidth - paletteWidth) / 2
	padTop := screenHeight / 5 // Position in upper portion of screen

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		PaddingTop(padTop).
		Render(palette)
}
