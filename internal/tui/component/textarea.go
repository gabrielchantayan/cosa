// Package component provides UI components for the TUI.
package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// TextArea is a multi-line text input component with word wrapping.
type TextArea struct {
	value   string
	cursor  int
	width   int
	height  int
	focused bool

	// Optional placeholder
	placeholder string

	// Scroll offset for viewing
	scrollOffset int
}

// NewTextArea creates a new text area component.
func NewTextArea() *TextArea {
	return &TextArea{
		width:  40,
		height: 5,
	}
}

// SetSize sets the text area dimensions.
func (t *TextArea) SetSize(width, height int) {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	t.width = width
	t.height = height
}

// SetPlaceholder sets the placeholder text.
func (t *TextArea) SetPlaceholder(placeholder string) {
	t.placeholder = placeholder
}

// Focus focuses the input.
func (t *TextArea) Focus() {
	t.focused = true
}

// Blur unfocuses the input.
func (t *TextArea) Blur() {
	t.focused = false
}

// Focused returns true if focused.
func (t *TextArea) Focused() bool {
	return t.focused
}

// Value returns the current value.
func (t *TextArea) Value() string {
	return t.value
}

// SetValue sets the value.
func (t *TextArea) SetValue(value string) {
	t.value = value
	t.cursor = len(value)
}

// Reset clears the input.
func (t *TextArea) Reset() {
	t.value = ""
	t.cursor = 0
	t.scrollOffset = 0
}

// HandleKey handles key presses.
func (t *TextArea) HandleKey(key string) {
	if !t.focused {
		return
	}

	switch key {
	case "backspace":
		if t.cursor > 0 && len(t.value) > 0 {
			t.value = t.value[:t.cursor-1] + t.value[t.cursor:]
			t.cursor--
		}
	case "delete":
		if t.cursor < len(t.value) {
			t.value = t.value[:t.cursor] + t.value[t.cursor+1:]
		}
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		if t.cursor < len(t.value) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len(t.value)
	case "ctrl+u":
		// Clear to beginning
		t.value = t.value[t.cursor:]
		t.cursor = 0
	case "ctrl+k":
		// Clear to end
		t.value = t.value[:t.cursor]
	case "enter":
		// Insert newline
		t.value = t.value[:t.cursor] + "\n" + t.value[t.cursor:]
		t.cursor++
	default:
		// Insert character
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			t.value = t.value[:t.cursor] + key + t.value[t.cursor:]
			t.cursor++
		} else if key == "space" {
			t.value = t.value[:t.cursor] + " " + t.value[t.cursor:]
			t.cursor++
		}
	}
}

// wrapText wraps text to fit within the given width, respecting word boundaries.
// Words longer than the width are broken at the width boundary.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{}
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		var currentLine strings.Builder
		currentWidth := 0

		for _, word := range words {
			wordLen := len(word)

			// If word is longer than width, break it
			if wordLen > width {
				// Flush current line first
				if currentWidth > 0 {
					lines = append(lines, currentLine.String())
					currentLine.Reset()
					currentWidth = 0
				}
				// Break the long word
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				if len(word) > 0 {
					currentLine.WriteString(word)
					currentWidth = len(word)
				}
				continue
			}

			// Check if word fits on current line
			if currentWidth == 0 {
				currentLine.WriteString(word)
				currentWidth = wordLen
			} else if currentWidth+1+wordLen <= width {
				currentLine.WriteString(" ")
				currentLine.WriteString(word)
				currentWidth += 1 + wordLen
			} else {
				// Word doesn't fit, start new line
				lines = append(lines, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(word)
				currentWidth = wordLen
			}
		}

		// Flush remaining content
		if currentWidth > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return lines
}

// View renders the text area.
func (t *TextArea) View() string {
	thm := theme.Current

	// Styles
	var borderColor lipgloss.Color
	if t.focused {
		borderColor = thm.BorderActive
	} else {
		borderColor = thm.Border
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(t.width).
		Height(t.height)

	contentWidth := t.width - 4 // Account for borders and padding

	// Content
	var content string
	if t.value == "" && !t.focused && t.placeholder != "" {
		// Show placeholder with word wrap
		wrappedPlaceholder := wrapText(t.placeholder, contentWidth)
		content = lipgloss.NewStyle().
			Foreground(thm.TextMuted).
			Render(strings.Join(wrappedPlaceholder, "\n"))
	} else {
		// Wrap the text for display
		wrappedLines := wrapText(t.value, contentWidth)

		if t.focused {
			// Find cursor position in wrapped text
			content = t.renderWithCursor(wrappedLines, contentWidth)
		} else {
			content = lipgloss.NewStyle().
				Foreground(thm.Text).
				Render(strings.Join(wrappedLines, "\n"))
		}
	}

	// Pad content to fill height
	contentLines := strings.Split(content, "\n")
	visibleHeight := t.height - 2 // Account for borders
	for len(contentLines) < visibleHeight {
		contentLines = append(contentLines, "")
	}

	// Scroll if needed
	if len(contentLines) > visibleHeight {
		// Find which line the cursor is on and adjust scroll
		cursorLine := t.findCursorLine(contentWidth)
		if cursorLine < t.scrollOffset {
			t.scrollOffset = cursorLine
		} else if cursorLine >= t.scrollOffset+visibleHeight {
			t.scrollOffset = cursorLine - visibleHeight + 1
		}
		contentLines = contentLines[t.scrollOffset:]
		if len(contentLines) > visibleHeight {
			contentLines = contentLines[:visibleHeight]
		}
	}

	// Pad each line to width
	for i, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			contentLines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

// findCursorLine finds which wrapped line the cursor is on.
func (t *TextArea) findCursorLine(width int) int {
	if t.cursor == 0 {
		return 0
	}

	// Get text up to cursor
	textToCursor := t.value[:t.cursor]
	wrapped := wrapText(textToCursor, width)
	if len(wrapped) == 0 {
		return 0
	}
	return len(wrapped) - 1
}

// renderWithCursor renders the wrapped text with cursor visible.
func (t *TextArea) renderWithCursor(wrappedLines []string, width int) string {
	thm := theme.Current
	textStyle := lipgloss.NewStyle().Foreground(thm.Text)
	cursorStyle := lipgloss.NewStyle().
		Background(thm.Primary).
		Foreground(thm.Background)

	// Find cursor position in original text
	if t.cursor >= len(t.value) {
		// Cursor at end
		if len(wrappedLines) == 0 {
			return cursorStyle.Render(" ")
		}
		// Append cursor to last line
		result := make([]string, len(wrappedLines))
		for i, line := range wrappedLines {
			if i == len(wrappedLines)-1 {
				result[i] = textStyle.Render(line) + cursorStyle.Render(" ")
			} else {
				result[i] = textStyle.Render(line)
			}
		}
		return strings.Join(result, "\n")
	}

	// Map cursor position to wrapped line and column
	pos := 0
	for lineIdx, line := range wrappedLines {
		lineLen := len(line)
		if pos+lineLen > t.cursor || (lineIdx == len(wrappedLines)-1 && pos+lineLen == t.cursor) {
			// Cursor is on this line
			colInLine := t.cursor - pos
			result := make([]string, len(wrappedLines))
			for i, l := range wrappedLines {
				if i == lineIdx {
					before := l[:colInLine]
					cursorChar := string(l[colInLine])
					after := ""
					if colInLine+1 < len(l) {
						after = l[colInLine+1:]
					}
					result[i] = textStyle.Render(before) + cursorStyle.Render(cursorChar) + textStyle.Render(after)
				} else {
					result[i] = textStyle.Render(l)
				}
			}
			return strings.Join(result, "\n")
		}
		pos += lineLen
		// Account for the space/newline that was removed during wrapping
		if pos < len(t.value) && (t.value[pos] == ' ' || t.value[pos] == '\n') {
			pos++
		}
	}

	// Fallback
	return textStyle.Render(strings.Join(wrappedLines, "\n")) + cursorStyle.Render(" ")
}
