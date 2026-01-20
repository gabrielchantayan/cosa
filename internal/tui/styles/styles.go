// Package styles provides the styling for the TUI.
package styles

import (
	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// Styles contains all the reusable styles for the TUI.
type Styles struct {
	// Layout
	App        lipgloss.Style
	Header     lipgloss.Style
	Footer     lipgloss.Style
	Content    lipgloss.Style

	// Panels
	Panel          lipgloss.Style
	PanelActive    lipgloss.Style
	PanelTitle     lipgloss.Style
	PanelTitleActive lipgloss.Style

	// Text
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Text       lipgloss.Style
	TextMuted  lipgloss.Style
	TextDim    lipgloss.Style

	// Status
	StatusIdle     lipgloss.Style
	StatusWorking  lipgloss.Style
	StatusReview   lipgloss.Style
	StatusError    lipgloss.Style
	StatusComplete lipgloss.Style

	// Roles
	RoleDon         lipgloss.Style
	RoleConsigliere lipgloss.Style
	RoleCapo        lipgloss.Style
	RoleSoldato     lipgloss.Style

	// List items
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemActive   lipgloss.Style

	// Activity
	ActivityTime    lipgloss.Style
	ActivityMessage lipgloss.Style
	ActivityWorker  lipgloss.Style

	// Keys help
	KeyHelp     lipgloss.Style
	KeyHelpKey  lipgloss.Style
	KeyHelpDesc lipgloss.Style
}

// New creates styles based on the current theme.
func New() Styles {
	t := theme.Current

	return Styles{
		// Layout
		App: lipgloss.NewStyle().
			Background(t.Background),

		Header: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 1),

		Content: lipgloss.NewStyle().
			Padding(1, 2),

		// Panels
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		PanelActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderActive).
			Padding(0, 1),

		PanelTitle: lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Bold(true).
			Padding(0, 1),

		PanelTitleActive: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		// Text
		Title: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.Secondary),

		Text: lipgloss.NewStyle().
			Foreground(t.Text),

		TextMuted: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		TextDim: lipgloss.NewStyle().
			Foreground(t.TextDim),

		// Status
		StatusIdle: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		StatusWorking: lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true),

		StatusReview: lipgloss.NewStyle().
			Foreground(t.Info),

		StatusError: lipgloss.NewStyle().
			Foreground(t.Error),

		StatusComplete: lipgloss.NewStyle().
			Foreground(t.Success),

		// Roles
		RoleDon: lipgloss.NewStyle().
			Foreground(t.RoleDon).
			Bold(true),

		RoleConsigliere: lipgloss.NewStyle().
			Foreground(t.RoleConsigliere),

		RoleCapo: lipgloss.NewStyle().
			Foreground(t.RoleCapo),

		RoleSoldato: lipgloss.NewStyle().
			Foreground(t.RoleSoldato),

		// List items
		ListItem: lipgloss.NewStyle().
			Foreground(t.Text).
			Padding(0, 1),

		ListItemSelected: lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.SurfaceLight).
			Padding(0, 1),

		ListItemActive: lipgloss.NewStyle().
			Foreground(t.Primary).
			Background(t.Surface).
			Padding(0, 1).
			Bold(true),

		// Activity
		ActivityTime: lipgloss.NewStyle().
			Foreground(t.TextDim).
			Width(8),

		ActivityMessage: lipgloss.NewStyle().
			Foreground(t.Text),

		ActivityWorker: lipgloss.NewStyle().
			Foreground(t.Accent),

		// Keys help
		KeyHelp: lipgloss.NewStyle().
			Foreground(t.TextDim),

		KeyHelpKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		KeyHelpDesc: lipgloss.NewStyle().
			Foreground(t.TextMuted),
	}
}

// RoleStyle returns the appropriate style for a role.
func (s Styles) RoleStyle(role string) lipgloss.Style {
	switch role {
	case "don":
		return s.RoleDon
	case "consigliere":
		return s.RoleConsigliere
	case "capo":
		return s.RoleCapo
	default:
		return s.RoleSoldato
	}
}

// StatusStyle returns the appropriate style for a status.
func (s Styles) StatusStyle(status string) lipgloss.Style {
	switch status {
	case "idle":
		return s.StatusIdle
	case "working":
		return s.StatusWorking
	case "reviewing":
		return s.StatusReview
	case "completed":
		return s.StatusComplete
	case "error", "failed":
		return s.StatusError
	default:
		return s.TextMuted
	}
}

// InsertPanelTitle inserts a styled title into the top border of a panel.
// It takes the panel content (already rendered with lipgloss border), the styled
// title string, and the border color to use for reconstructing the border line.
// Returns the modified panel with the title inserted.
func InsertPanelTitle(panel, styledTitle string, borderColor lipgloss.Color) string {
	lines := splitLines(panel)
	if len(lines) == 0 {
		return panel
	}

	titleVisualWidth := lipgloss.Width(styledTitle)
	firstLineWidth := lipgloss.Width(lines[0])

	// Need enough space for corner + title + at least one border char + corner
	if firstLineWidth <= titleVisualWidth+2 {
		return panel
	}

	// Build the border with title: corner + title + remaining border + corner
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	corner := borderStyle.Render("╭")
	endCorner := borderStyle.Render("╮")

	// Calculate how many border characters we need after the title
	// Total width = 1 (start corner) + titleWidth + borderChars + 1 (end corner)
	// So borderChars = firstLineWidth - 2 - titleWidth
	borderChars := firstLineWidth - 2 - titleVisualWidth
	if borderChars < 0 {
		borderChars = 0
	}

	borderLine := borderStyle.Render(repeat("─", borderChars))
	lines[0] = corner + styledTitle + borderLine + endCorner

	return joinLines(lines)
}

// repeat returns a string with s repeated n times.
// Returns empty string if n <= 0.
func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines joins lines with newline separators.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}
