// Package component provides UI components for the TUI.
package component

import (
	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// Input is a text input component.
type Input struct {
	value   string
	cursor  int
	width   int
	focused bool

	// Optional placeholder
	placeholder string
}

// NewInput creates a new input component.
func NewInput() *Input {
	return &Input{
		width: 40,
	}
}

// SetWidth sets the input width.
func (i *Input) SetWidth(width int) {
	if width < 10 {
		width = 10
	}
	i.width = width
}

// SetPlaceholder sets the placeholder text.
func (i *Input) SetPlaceholder(placeholder string) {
	i.placeholder = placeholder
}

// Focus focuses the input.
func (i *Input) Focus() {
	i.focused = true
}

// Blur unfocuses the input.
func (i *Input) Blur() {
	i.focused = false
}

// Focused returns true if focused.
func (i *Input) Focused() bool {
	return i.focused
}

// Value returns the current value.
func (i *Input) Value() string {
	return i.value
}

// SetValue sets the value.
func (i *Input) SetValue(value string) {
	i.value = value
	i.cursor = len(value)
}

// Reset clears the input.
func (i *Input) Reset() {
	i.value = ""
	i.cursor = 0
}

// HandleKey handles key presses.
func (i *Input) HandleKey(key string) {
	if !i.focused {
		return
	}

	switch key {
	case "backspace":
		if i.cursor > 0 && len(i.value) > 0 {
			i.value = i.value[:i.cursor-1] + i.value[i.cursor:]
			i.cursor--
		}
	case "delete":
		if i.cursor < len(i.value) {
			i.value = i.value[:i.cursor] + i.value[i.cursor+1:]
		}
	case "left":
		if i.cursor > 0 {
			i.cursor--
		}
	case "right":
		if i.cursor < len(i.value) {
			i.cursor++
		}
	case "home", "ctrl+a":
		i.cursor = 0
	case "end", "ctrl+e":
		i.cursor = len(i.value)
	case "ctrl+u":
		// Clear to beginning
		i.value = i.value[i.cursor:]
		i.cursor = 0
	case "ctrl+k":
		// Clear to end
		i.value = i.value[:i.cursor]
	case "ctrl+w":
		// Delete word backward
		if i.cursor > 0 {
			// Find start of word
			pos := i.cursor - 1
			for pos > 0 && i.value[pos-1] == ' ' {
				pos--
			}
			for pos > 0 && i.value[pos-1] != ' ' {
				pos--
			}
			i.value = i.value[:pos] + i.value[i.cursor:]
			i.cursor = pos
		}
	default:
		// Insert character
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			i.value = i.value[:i.cursor] + key + i.value[i.cursor:]
			i.cursor++
		} else if key == "space" {
			i.value = i.value[:i.cursor] + " " + i.value[i.cursor:]
			i.cursor++
		}
	}
}

// View renders the input.
func (i *Input) View() string {
	t := theme.Current

	// Styles
	var borderColor lipgloss.Color
	if i.focused {
		borderColor = t.BorderActive
	} else {
		borderColor = t.Border
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(i.width).
		Padding(0, 1)

	// Content
	var content string
	if i.value == "" && !i.focused && i.placeholder != "" {
		// Show placeholder
		content = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(i.placeholder)
	} else if i.focused {
		// Show with cursor
		textStyle := lipgloss.NewStyle().Foreground(t.Text)
		cursorStyle := lipgloss.NewStyle().
			Background(t.Primary).
			Foreground(t.Background)

		beforeCursor := i.value[:i.cursor]
		afterCursor := ""
		cursorChar := " "

		if i.cursor < len(i.value) {
			cursorChar = string(i.value[i.cursor])
			afterCursor = i.value[i.cursor+1:]
		}

		content = textStyle.Render(beforeCursor) +
			cursorStyle.Render(cursorChar) +
			textStyle.Render(afterCursor)
	} else {
		content = lipgloss.NewStyle().
			Foreground(t.Text).
			Render(i.value)
	}

	// Truncate if too long
	displayWidth := i.width - 4 // Account for borders and padding
	if lipgloss.Width(content) > displayWidth {
		// Scroll to show cursor
		runes := []rune(i.value)
		start := 0
		if i.cursor > displayWidth-2 {
			start = i.cursor - displayWidth + 2
		}
		end := start + displayWidth - 1
		if end > len(runes) {
			end = len(runes)
		}

		visibleValue := string(runes[start:end])
		relativeCursor := i.cursor - start

		if i.focused && relativeCursor >= 0 && relativeCursor <= len(visibleValue) {
			textStyle := lipgloss.NewStyle().Foreground(t.Text)
			cursorStyle := lipgloss.NewStyle().
				Background(t.Primary).
				Foreground(t.Background)

			beforeCursor := visibleValue[:relativeCursor]
			afterCursor := ""
			cursorChar := " "

			if relativeCursor < len(visibleValue) {
				cursorChar = string(visibleValue[relativeCursor])
				afterCursor = visibleValue[relativeCursor+1:]
			}

			content = textStyle.Render(beforeCursor) +
				cursorStyle.Render(cursorChar) +
				textStyle.Render(afterCursor)
		} else {
			content = lipgloss.NewStyle().
				Foreground(t.Text).
				Render(visibleValue)
		}
	}

	return boxStyle.Render(content)
}
