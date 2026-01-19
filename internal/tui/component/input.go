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

	// Multiline support
	multiline bool
	minHeight int
	maxHeight int
}

// NewInput creates a new input component.
func NewInput() *Input {
	return &Input{
		width:     40,
		minHeight: 1,
		maxHeight: 5,
	}
}

// SetMultiline enables multiline input mode.
func (i *Input) SetMultiline(enabled bool) {
	i.multiline = enabled
}

// SetHeightLimits sets the min and max height for multiline input.
func (i *Input) SetHeightLimits(min, max int) {
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}
	i.minHeight = min
	i.maxHeight = max
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

// wrapText wraps text to fit within the given width, returning lines.
func wrapText(text string, width int) []string {
	if width <= 0 || text == "" {
		return []string{text}
	}

	var lines []string
	runes := []rune(text)
	start := 0

	for start < len(runes) {
		end := start + width
		if end >= len(runes) {
			lines = append(lines, string(runes[start:]))
			break
		}
		lines = append(lines, string(runes[start:end]))
		start = end
	}

	if len(lines) == 0 {
		lines = []string{""}
	}

	return lines
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

	displayWidth := i.width - 4 // Account for borders and padding

	// For multiline mode, wrap text and grow vertically
	if i.multiline {
		return i.viewMultiline(t, borderColor, displayWidth)
	}

	// Single-line mode (original behavior)
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

// viewMultiline renders the input in multiline mode.
func (i *Input) viewMultiline(t theme.Theme, borderColor lipgloss.Color, displayWidth int) string {
	textStyle := lipgloss.NewStyle().Foreground(t.Text)
	cursorStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background)

	lines := wrapText(i.value, displayWidth)
	numLines := len(lines)
	if numLines < i.minHeight {
		numLines = i.minHeight
	}
	if numLines > i.maxHeight {
		numLines = i.maxHeight
	}

	// Find cursor position in wrapped text
	cursorLine := 0
	cursorCol := i.cursor
	for idx, line := range lines {
		if cursorCol <= len([]rune(line)) {
			cursorLine = idx
			break
		}
		cursorCol -= len([]rune(line))
		cursorLine = idx + 1
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
		if len(lines) > 0 {
			cursorCol = len([]rune(lines[cursorLine]))
		} else {
			cursorCol = 0
		}
	}

	// Determine visible line range
	startLine := 0
	if cursorLine >= numLines {
		startLine = cursorLine - numLines + 1
	}
	endLine := startLine + numLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var renderedLines []string
	for idx := startLine; idx < endLine; idx++ {
		line := lines[idx]
		lineRunes := []rune(line)

		if i.focused && idx == cursorLine {
			beforeCursor := ""
			cursorChar := " "
			afterCursor := ""

			if cursorCol < len(lineRunes) {
				beforeCursor = string(lineRunes[:cursorCol])
				cursorChar = string(lineRunes[cursorCol])
				if cursorCol+1 < len(lineRunes) {
					afterCursor = string(lineRunes[cursorCol+1:])
				}
			} else {
				beforeCursor = line
			}

			renderedLine := textStyle.Render(beforeCursor) +
				cursorStyle.Render(cursorChar) +
				textStyle.Render(afterCursor)
			renderedLines = append(renderedLines, renderedLine)
		} else {
			renderedLines = append(renderedLines, textStyle.Render(line))
		}
	}

	// Pad with empty lines if needed
	for len(renderedLines) < numLines {
		if i.focused && len(renderedLines) == cursorLine && i.value == "" {
			renderedLines = append(renderedLines, cursorStyle.Render(" "))
		} else {
			renderedLines = append(renderedLines, "")
		}
	}

	// Handle placeholder
	if i.value == "" && !i.focused && i.placeholder != "" {
		renderedLines = []string{lipgloss.NewStyle().Foreground(t.TextMuted).Render(i.placeholder)}
		for len(renderedLines) < i.minHeight {
			renderedLines = append(renderedLines, "")
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, renderedLines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(i.width).
		Padding(0, 1)

	return boxStyle.Render(content)
}
