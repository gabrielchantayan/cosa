// Package util provides utility functions for the TUI.
package util

import "strings"

// WrapText wraps text to fit within the given width, returning lines.
// It performs word-aware wrapping to avoid cutting words in the middle.
// This is suitable for single-line text that needs to be displayed wrapped.
func WrapText(text string, width int) []string {
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

		// Look for a space to break at (word boundary)
		breakAt := end
		foundSpace := false
		for i := end; i > start; i-- {
			if runes[i] == ' ' {
				breakAt = i
				foundSpace = true
				break
			}
		}

		if foundSpace {
			// Break at the space, don't include the space in the line
			lines = append(lines, string(runes[start:breakAt]))
			start = breakAt + 1 // Skip the space
		} else {
			// No space found - word is longer than width, must break mid-word
			lines = append(lines, string(runes[start:end]))
			start = end
		}
	}

	if len(lines) == 0 {
		lines = []string{""}
	}

	return lines
}

// WrapTextMultiline wraps text to fit within the given width, respecting
// existing newlines and word boundaries. Words longer than the width are
// broken at the width boundary. This is suitable for multi-line text areas.
func WrapTextMultiline(text string, width int) []string {
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
