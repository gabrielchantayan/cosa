// Package component provides UI components for the TUI.
package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// Dialog is a modal dialog component.
type Dialog struct {
	title   string
	content string
	width   int
	height  int
	visible bool

	// Input field (single line)
	input       *Input
	hasInput    bool
	inputLabel  string

	// Text area (multi-line with word wrap)
	textArea     *TextArea
	hasTextArea  bool
	textAreaLabel string

	// Buttons
	buttons        []DialogButton
	selectedButton int
}

// DialogButton represents a dialog button.
type DialogButton struct {
	Label   string
	Action  string // Identifier for the action
	Primary bool
}

// NewDialog creates a new dialog.
func NewDialog(title string) *Dialog {
	return &Dialog{
		title:   title,
		buttons: make([]DialogButton, 0),
	}
}

// SetSize sets the dialog dimensions.
func (d *Dialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	if d.input != nil {
		d.input.SetWidth(width - 8)
	}
	if d.textArea != nil {
		d.textArea.SetSize(width-8, height-10)
	}
}

// SetContent sets the dialog content.
func (d *Dialog) SetContent(content string) {
	d.content = content
}

// SetInput enables input mode with a label.
func (d *Dialog) SetInput(label string) {
	d.hasInput = true
	d.inputLabel = label
	d.input = NewInput()
	d.input.SetWidth(d.width - 8)
	d.input.SetMultiline(true)
	d.input.SetHeightLimits(1, 4)
	d.input.Focus()
}

// SetTextArea enables multi-line text area mode with a label and word wrapping.
func (d *Dialog) SetTextArea(label string) {
	d.hasTextArea = true
	d.textAreaLabel = label
	d.textArea = NewTextArea()
	d.textArea.SetSize(d.width-8, d.height-10)
	d.textArea.Focus()
}

// AddButton adds a button to the dialog.
func (d *Dialog) AddButton(label, action string, primary bool) {
	d.buttons = append(d.buttons, DialogButton{
		Label:   label,
		Action:  action,
		Primary: primary,
	})
}

// Show makes the dialog visible.
func (d *Dialog) Show() {
	d.visible = true
	if d.input != nil {
		d.input.Focus()
	}
	if d.textArea != nil {
		d.textArea.Focus()
	}
}

// Hide hides the dialog.
func (d *Dialog) Hide() {
	d.visible = false
	if d.input != nil {
		d.input.Reset()
	}
	if d.textArea != nil {
		d.textArea.Reset()
	}
}

// Visible returns true if the dialog is visible.
func (d *Dialog) Visible() bool {
	return d.visible
}

// GetInputValue returns the current input value.
func (d *Dialog) GetInputValue() string {
	if d.textArea != nil {
		return d.textArea.Value()
	}
	if d.input != nil {
		return d.input.Value()
	}
	return ""
}

// SelectedAction returns the selected button's action.
func (d *Dialog) SelectedAction() string {
	if d.selectedButton >= 0 && d.selectedButton < len(d.buttons) {
		return d.buttons[d.selectedButton].Action
	}
	return ""
}

// HandleKey handles key presses.
func (d *Dialog) HandleKey(key string) string {
	// Handle text area input
	if d.hasTextArea && d.textArea != nil {
		switch key {
		case "tab":
			// Move focus from text area to buttons
			if d.textArea.Focused() && len(d.buttons) > 0 {
				d.textArea.Blur()
				d.selectedButton = 0
			} else if !d.textArea.Focused() {
				d.textArea.Focus()
				d.selectedButton = -1
			}
			return ""
		case "ctrl+enter":
			// Submit with first primary button (use ctrl+enter for text areas since enter adds newlines)
			for _, btn := range d.buttons {
				if btn.Primary {
					return btn.Action
				}
			}
			if len(d.buttons) > 0 {
				return d.buttons[0].Action
			}
			return ""
		case "left":
			if !d.textArea.Focused() && d.selectedButton > 0 {
				d.selectedButton--
			} else if d.textArea.Focused() {
				d.textArea.HandleKey(key)
			}
			return ""
		case "right":
			if !d.textArea.Focused() && d.selectedButton < len(d.buttons)-1 {
				d.selectedButton++
			} else if d.textArea.Focused() {
				d.textArea.HandleKey(key)
			}
			return ""
		case "esc":
			return "cancel"
		default:
			if d.textArea.Focused() {
				d.textArea.HandleKey(key)
			} else if key == "enter" && d.selectedButton >= 0 {
				return d.buttons[d.selectedButton].Action
			}
			return ""
		}
	}

	// Handle single-line input
	if d.hasInput && d.input != nil {
		switch key {
		case "tab":
			// Move focus from input to buttons
			if d.input.Focused() && len(d.buttons) > 0 {
				d.input.Blur()
				d.selectedButton = 0
			} else if !d.input.Focused() {
				d.input.Focus()
				d.selectedButton = -1
			}
			return ""
		case "enter":
			if !d.input.Focused() && d.selectedButton >= 0 {
				return d.buttons[d.selectedButton].Action
			}
			// If input focused, submit with first primary button
			for _, btn := range d.buttons {
				if btn.Primary {
					return btn.Action
				}
			}
			if len(d.buttons) > 0 {
				return d.buttons[0].Action
			}
			return ""
		case "left":
			if !d.input.Focused() && d.selectedButton > 0 {
				d.selectedButton--
			}
			return ""
		case "right":
			if !d.input.Focused() && d.selectedButton < len(d.buttons)-1 {
				d.selectedButton++
			}
			return ""
		case "esc":
			return "cancel"
		default:
			if d.input.Focused() {
				d.input.HandleKey(key)
			}
			return ""
		}
	}

	// No input, just buttons
	switch key {
	case "tab", "right":
		if d.selectedButton < len(d.buttons)-1 {
			d.selectedButton++
		} else {
			d.selectedButton = 0
		}
	case "shift+tab", "left":
		if d.selectedButton > 0 {
			d.selectedButton--
		} else {
			d.selectedButton = len(d.buttons) - 1
		}
	case "enter":
		if d.selectedButton >= 0 && d.selectedButton < len(d.buttons) {
			return d.buttons[d.selectedButton].Action
		}
	case "esc":
		return "cancel"
	}
	return ""
}

// View renders the dialog.
func (d *Dialog) View() string {
	if !d.visible {
		return ""
	}

	t := theme.Current

	// Title bar with more prominent styling
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(1, 2).
		MarginBottom(1)

	// Content
	contentStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Padding(0, 2).
		MarginBottom(1)

	// Build content sections
	var sections []string

	// Title
	sections = append(sections, titleStyle.Render(d.title))

	// Content text
	if d.content != "" {
		sections = append(sections, contentStyle.Render(d.content))
	}

	// Text area (multi-line with word wrap)
	if d.hasTextArea && d.textArea != nil {
		labelStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 2)
		textAreaSection := lipgloss.JoinVertical(lipgloss.Left,
			labelStyle.Render(d.textAreaLabel),
			"  "+d.textArea.View(),
		)
		sections = append(sections, textAreaSection)
	}

	// Input field (single-line)
	if d.hasInput && d.input != nil {
		labelStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 2).
			MarginBottom(0)
		inputWrapper := lipgloss.NewStyle().
			Padding(0, 2)
		inputSection := lipgloss.JoinVertical(lipgloss.Left,
			labelStyle.Render(d.inputLabel),
			inputWrapper.Render(d.input.View()),
		)
		sections = append(sections, inputSection)
	}

	// Buttons
	if len(d.buttons) > 0 {
		var btnViews []string
		for i, btn := range d.buttons {
			var style lipgloss.Style
			if i == d.selectedButton {
				if btn.Primary {
					style = lipgloss.NewStyle().
						Background(t.Primary).
						Foreground(t.Background).
						Bold(true).
						Padding(0, 3)
				} else {
					style = lipgloss.NewStyle().
						Background(t.Surface).
						Foreground(t.Primary).
						Bold(true).
						Padding(0, 3)
				}
			} else {
				if btn.Primary {
					style = lipgloss.NewStyle().
						Foreground(t.Primary).
						Padding(0, 3)
				} else {
					style = lipgloss.NewStyle().
						Foreground(t.TextMuted).
						Padding(0, 3)
				}
			}
			btnViews = append(btnViews, style.Render(btn.Label))
		}
		buttonRow := strings.Join(btnViews, "   ")
		sections = append(sections, lipgloss.NewStyle().Padding(1, 2).Render(buttonRow))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Dialog box
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Background(t.Background).
		Width(d.width)

	return dialogStyle.Render(content)
}

// CenterIn returns the dialog centered in the given dimensions.
func (d *Dialog) CenterIn(screenWidth, screenHeight int) string {
	dialog := d.View()
	if dialog == "" {
		return ""
	}

	dialogWidth := lipgloss.Width(dialog)
	dialogHeight := lipgloss.Height(dialog)

	// Calculate padding
	padLeft := (screenWidth - dialogWidth) / 2
	padTop := (screenHeight - dialogHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		PaddingTop(padTop).
		Render(dialog)
}

// NewJobDialog creates a dialog configured for entering a new job description.
// The dialog is 86 characters wide (16 chars wider than the original 70) with
// a multi-line text area that uses word wrapping to avoid cutting off words.
func NewJobDialog() *Dialog {
	d := NewDialog("New Job")
	d.SetSize(86, 16)
	d.SetTextArea("Job Description (Ctrl+Enter to submit):")
	d.AddButton("Create", "create", true)
	d.AddButton("Cancel", "cancel", false)
	return d
}
