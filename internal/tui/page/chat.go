package page

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/protocol"
	"cosa/internal/tui/styles"
	"cosa/internal/tui/theme"
)

// ChatMessage represents a message in the chat.
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// JobCounts represents job statistics.
type JobCounts struct {
	Pending    int
	InProgress int
	Completed  int
	Failed     int
}

// Chat is the chat page for conversing with The Underboss.
type Chat struct {
	styles styles.Styles
	width  int
	height int

	// Messages
	messages      []ChatMessage
	messageScroll int

	// Input
	inputBuffer string
	inputCursor int

	// Focus: 0=messages, 1=input (input by default)
	focusSection int

	// Loading state
	loading      bool
	loadingFrame int

	// Workers sidebar
	workers   []protocol.WorkerInfo
	jobCounts JobCounts

	// Session
	sessionID string
	isResumed bool

	// Callbacks
	onSendMessage func(string)
	onCancel      func()
}

// NewChat creates a new chat page.
func NewChat() *Chat {
	return &Chat{
		styles:       styles.New(),
		messages:     make([]ChatMessage, 0),
		focusSection: 1, // Start focused on input
	}
}

// SetSize sets the page dimensions.
func (c *Chat) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetOnSendMessage sets the callback for when a message is sent.
func (c *Chat) SetOnSendMessage(fn func(string)) {
	c.onSendMessage = fn
}

// SetOnCancel sets the callback for when the chat is cancelled.
func (c *Chat) SetOnCancel(fn func()) {
	c.onCancel = fn
}

// SetSessionID sets the chat session ID.
func (c *Chat) SetSessionID(id string, resumed bool) {
	c.sessionID = id
	c.isResumed = resumed
}

// SetWorkers updates the workers sidebar.
func (c *Chat) SetWorkers(workers []protocol.WorkerInfo) {
	c.workers = workers
}

// SetJobCounts updates the job counts.
func (c *Chat) SetJobCounts(counts JobCounts) {
	c.jobCounts = counts
}

// AddMessage adds a message to the chat.
func (c *Chat) AddMessage(role, content string) {
	c.messages = append(c.messages, ChatMessage{
		Role:    role,
		Content: content,
	})
	// Auto-scroll to bottom
	c.scrollToBottom()
}

// SetLoading sets the loading state.
func (c *Chat) SetLoading(loading bool) {
	c.loading = loading
	if loading {
		c.loadingFrame = 0
	}
}

// TickLoading advances the loading animation frame.
func (c *Chat) TickLoading() {
	c.loadingFrame++
}

// IsLoading returns true if waiting for a response.
func (c *Chat) IsLoading() bool {
	return c.loading
}

// IsInputMode returns true if the chat is in input mode.
func (c *Chat) IsInputMode() bool {
	return c.focusSection == 1 && !c.loading
}

// ClearInput clears the input buffer.
func (c *Chat) ClearInput() {
	c.inputBuffer = ""
	c.inputCursor = 0
}

// GetInput returns the current input and clears it.
func (c *Chat) GetInput() string {
	input := c.inputBuffer
	c.ClearInput()
	return input
}

// HandleKey handles key presses.
func (c *Chat) HandleKey(key string) string {
	if c.loading {
		// Only allow escape during loading
		if key == "esc" {
			if c.onCancel != nil {
				c.onCancel()
			}
			return "cancel"
		}
		return ""
	}

	if c.focusSection == 1 {
		// Input mode
		return c.handleInputKey(key)
	}

	// Message viewing mode
	switch key {
	case "j", "down":
		c.scrollDown()
	case "k", "up":
		c.scrollUp()
	case "g":
		c.scrollToTop()
	case "G":
		c.scrollToBottom()
	case "tab":
		c.focusSection = 1
	case "esc":
		if c.inputBuffer == "" {
			return "exit"
		}
	}
	return ""
}

func (c *Chat) handleInputKey(key string) string {
	switch key {
	case "enter":
		if c.inputBuffer != "" {
			return "send"
		}
	case "esc":
		if c.inputBuffer == "" {
			return "exit"
		}
		c.focusSection = 0
	case "backspace":
		if c.inputCursor > 0 && len(c.inputBuffer) > 0 {
			c.inputBuffer = c.inputBuffer[:c.inputCursor-1] + c.inputBuffer[c.inputCursor:]
			c.inputCursor--
		}
	case "left":
		if c.inputCursor > 0 {
			c.inputCursor--
		}
	case "right":
		if c.inputCursor < len(c.inputBuffer) {
			c.inputCursor++
		}
	case "ctrl+a", "home":
		c.inputCursor = 0
	case "ctrl+e", "end":
		c.inputCursor = len(c.inputBuffer)
	case "ctrl+u":
		c.inputBuffer = c.inputBuffer[c.inputCursor:]
		c.inputCursor = 0
	case "ctrl+k":
		c.inputBuffer = c.inputBuffer[:c.inputCursor]
	case "tab":
		c.focusSection = 0
	default:
		if len(key) == 1 && key[0] >= 32 {
			c.inputBuffer = c.inputBuffer[:c.inputCursor] + key + c.inputBuffer[c.inputCursor:]
			c.inputCursor++
		}
	}
	return ""
}

func (c *Chat) scrollDown() {
	maxScroll := c.maxMessageScroll()
	if c.messageScroll < maxScroll {
		c.messageScroll++
	}
}

func (c *Chat) scrollUp() {
	if c.messageScroll > 0 {
		c.messageScroll--
	}
}

func (c *Chat) scrollToTop() {
	c.messageScroll = 0
}

func (c *Chat) scrollToBottom() {
	c.messageScroll = c.maxMessageScroll()
}

func (c *Chat) maxMessageScroll() int {
	chatHeight := c.height - 4 // Header, input, footer, padding
	contentHeight := chatHeight - 3 // Account for panel title and borders
	totalLines := c.countMessageLines()
	if totalLines <= contentHeight {
		return 0
	}
	return totalLines - contentHeight
}

func (c *Chat) countMessageLines() int {
	chatWidth := c.width * 70 / 100
	contentWidth := chatWidth - 4 // Account for borders and padding

	total := 0
	for _, msg := range c.messages {
		lines := c.wrapText(msg.Content, contentWidth-2) // Account for message padding
		total += len(lines) + 2 // +1 for role line, +1 for blank line after
	}
	return total
}

// View renders the chat page.
func (c *Chat) View() string {
	t := theme.Current

	// Header
	header := c.renderHeader()

	// Calculate layout dimensions (70% chat, 30% sidebar like dashboard but flipped)
	chatWidth := c.width * 70 / 100
	sidebarWidth := c.width - chatWidth - 3 // Match dashboard's -3 spacing

	// Content height accounting for header (1), input (1), footer (1), and padding (1)
	chatHeight := c.height - 4
	if chatHeight < 4 {
		chatHeight = 4
	}

	// Chat panel (messages)
	chatPanel := c.renderChatPanel(chatWidth, chatHeight)

	// Sidebar (workers + job counts)
	sidebarPanel := c.renderSidebar(sidebarWidth, chatHeight)

	// Join columns
	content := lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, sidebarPanel)

	// Input area
	inputArea := c.renderInput()

	// Footer
	footer := c.renderFooter()

	// Combine all sections vertically
	result := lipgloss.JoinVertical(lipgloss.Left, header, content, inputArea, footer)

	return lipgloss.NewStyle().
		Background(t.Background).
		Width(c.width).
		Height(c.height).
		Render(result)
}

func (c *Chat) renderHeader() string {
	t := theme.Current

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	sessionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	title := titleStyle.Render(" THE UNDERBOSS ")
	sessionInfo := ""
	if c.sessionID != "" {
		status := "new session"
		if c.isResumed {
			status = "resumed"
		}
		sessionInfo = sessionStyle.Render(fmt.Sprintf(" [%s]", status))
	}

	header := title + sessionInfo

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(c.width).
		Padding(0, 1).
		Render(header)
}

func (c *Chat) renderChatPanel(width, height int) string {
	t := theme.Current

	// Ensure minimum dimensions
	if width < 6 {
		width = 6
	}
	if height < 4 {
		height = 4
	}

	// Content area dimensions (matching dashboard's renderPanel)
	contentWidth := width - 4  // Account for borders and padding
	contentHeight := height - 3 // Account for title and borders

	var lines []string
	for _, msg := range c.messages {
		msgLines := c.renderMessage(msg, contentWidth)
		lines = append(lines, msgLines...)
		lines = append(lines, "") // Blank line between messages
	}

	// Add loading indicator
	if c.loading {
		spinner := c.getLoadingSpinner()
		loadingStyle := lipgloss.NewStyle().Foreground(t.Primary).Italic(true)
		lines = append(lines, loadingStyle.Render(fmt.Sprintf(" %s The Underboss is thinking...", spinner)))
	}

	// Apply scroll
	visibleLines := lines
	if len(lines) > contentHeight {
		start := c.messageScroll
		end := start + contentHeight
		if end > len(lines) {
			end = len(lines)
			start = end - contentHeight
			if start < 0 {
				start = 0
			}
		}
		visibleLines = lines[start:end]
	}

	// Pad or truncate content to fill height
	for len(visibleLines) < contentHeight {
		visibleLines = append(visibleLines, "")
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	// Ensure each line is the right width
	for i, line := range visibleLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			visibleLines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	content := strings.Join(visibleLines, "\n")

	// Border style based on focus
	borderColor := t.Border
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	if c.focusSection == 0 {
		borderColor = t.BorderActive
		titleStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	}

	titleStr := titleStyle.Render(" CHAT ")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2).
		Render(content)

	// Add title to top border (matching dashboard's renderPanel)
	panelLines := strings.Split(panel, "\n")
	if len(panelLines) > 0 {
		firstLine := panelLines[0]
		titleWidth := lipgloss.Width(titleStr)
		if len(firstLine) > titleWidth+4 {
			panelLines[0] = firstLine[:2] + titleStr + firstLine[2+titleWidth:]
		}
	}

	return strings.Join(panelLines, "\n")
}

func (c *Chat) renderMessage(msg ChatMessage, width int) []string {
	t := theme.Current
	var lines []string

	// Role indicator
	var roleStyle lipgloss.Style
	var roleName string
	if msg.Role == "user" {
		roleStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		roleName = "You"
	} else {
		roleStyle = lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
		roleName = "The Underboss"
	}
	lines = append(lines, " "+roleStyle.Render(roleName+":"))

	// Content
	contentStyle := lipgloss.NewStyle().Foreground(t.Text)
	wrappedLines := c.wrapText(msg.Content, width-2)
	for _, line := range wrappedLines {
		// Check for tool use markers
		if strings.HasPrefix(line, "[Using tool:") {
			toolStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
			lines = append(lines, " "+toolStyle.Render(line))
		} else {
			lines = append(lines, " "+contentStyle.Render(line))
		}
	}

	return lines
}

func (c *Chat) wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	var result []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			result = append(result, "")
			continue
		}

		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}

		var line string
		for _, word := range words {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				result = append(result, line)
				line = word
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}

func (c *Chat) renderSidebar(width, height int) string {
	t := theme.Current

	// Ensure minimum dimensions
	if width < 6 {
		width = 6
	}
	if height < 4 {
		height = 4
	}

	// Content area dimensions (matching dashboard's renderPanel)
	contentWidth := width - 4  // Account for borders and padding
	contentHeight := height - 3 // Account for title and borders

	// Workers section
	workersTitleStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	workersTitle := workersTitleStyle.Render("WORKERS")

	var contentLines []string
	contentLines = append(contentLines, " "+workersTitle)
	contentLines = append(contentLines, "")

	if len(c.workers) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		contentLines = append(contentLines, " "+mutedStyle.Render("No workers"))
	} else {
		for _, w := range c.workers {
			line := c.renderWorkerLine(w, contentWidth-2)
			contentLines = append(contentLines, " "+line)
		}
	}

	// Jobs section
	contentLines = append(contentLines, "")
	jobsTitleStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	jobsTitle := jobsTitleStyle.Render("JOBS")
	contentLines = append(contentLines, " "+jobsTitle)
	contentLines = append(contentLines, "")

	pendingStyle := lipgloss.NewStyle().Foreground(t.Warning)
	progressStyle := lipgloss.NewStyle().Foreground(t.Primary)

	jobLine := fmt.Sprintf(" %s pending / %s in progress",
		pendingStyle.Render(fmt.Sprintf("%d", c.jobCounts.Pending)),
		progressStyle.Render(fmt.Sprintf("%d", c.jobCounts.InProgress)))
	contentLines = append(contentLines, jobLine)

	// Pad or truncate content to fill height
	for len(contentLines) < contentHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > contentHeight {
		contentLines = contentLines[:contentHeight]
	}

	// Ensure each line is the right width
	for i, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			contentLines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	content := strings.Join(contentLines, "\n")

	// Title with padding
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	titleStr := titleStyle.Render(" INFO ")

	// Render panel
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(width - 2).
		Height(height - 2).
		Render(content)

	// Add title to top border (matching dashboard's renderPanel)
	panelLines := strings.Split(panel, "\n")
	if len(panelLines) > 0 {
		firstLine := panelLines[0]
		titleWidth := lipgloss.Width(titleStr)
		if len(firstLine) > titleWidth+4 {
			panelLines[0] = firstLine[:2] + titleStr + firstLine[2+titleWidth:]
		}
	}

	return strings.Join(panelLines, "\n")
}

func (c *Chat) renderWorkerLine(w protocol.WorkerInfo, width int) string {
	t := theme.Current

	// Status indicator
	var indicator string
	var statusStyle lipgloss.Style
	switch w.Status {
	case "running":
		indicator = "*"
		statusStyle = lipgloss.NewStyle().Foreground(t.Success)
	case "idle":
		indicator = "*"
		statusStyle = lipgloss.NewStyle().Foreground(t.Primary)
	default:
		indicator = "o"
		statusStyle = lipgloss.NewStyle().Foreground(t.TextMuted)
	}

	nameStyle := lipgloss.NewStyle().Foreground(t.Text)
	jobStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	name := nameStyle.Render(w.Name)
	status := statusStyle.Render(indicator)

	line := fmt.Sprintf("%s %s", status, name)

	if w.CurrentJobDesc != "" {
		desc := w.CurrentJobDesc
		maxDescLen := width - len(w.Name) - 5
		if len(desc) > maxDescLen && maxDescLen > 3 {
			desc = desc[:maxDescLen-2] + ".."
		}
		line += " - " + jobStyle.Render(desc)
	} else {
		line += " - " + jobStyle.Render("idle")
	}

	return line
}

func (c *Chat) renderInput() string {
	t := theme.Current

	promptStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	// Build input with cursor
	beforeCursor := c.inputBuffer[:c.inputCursor]
	afterCursor := ""
	if c.inputCursor < len(c.inputBuffer) {
		afterCursor = c.inputBuffer[c.inputCursor+1:]
	}

	cursorStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background)

	cursor := cursorStyle.Render(" ")
	if c.inputCursor < len(c.inputBuffer) {
		cursor = cursorStyle.Render(string(c.inputBuffer[c.inputCursor]))
	}

	var input string
	if c.loading {
		input = promptStyle.Render(" > ") + inputStyle.Render("(waiting for response...)")
	} else if c.focusSection == 1 {
		input = promptStyle.Render(" > ") + inputStyle.Render(beforeCursor) + cursor + inputStyle.Render(afterCursor)
	} else {
		input = promptStyle.Render(" > ") + inputStyle.Render(c.inputBuffer)
	}

	sendHint := ""
	if !c.loading && c.inputBuffer != "" {
		hintStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		sendHint = hintStyle.Render(" [Enter to send]")
	}

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(c.width).
		Padding(0, 1).
		Render(input + sendHint)
}

func (c *Chat) renderFooter() string {
	t := theme.Current

	var keys []struct {
		key  string
		desc string
	}

	if c.loading {
		keys = []struct {
			key  string
			desc string
		}{
			{"Esc", "cancel"},
		}
	} else {
		keys = []struct {
			key  string
			desc string
		}{
			{"Enter", "send"},
			{"Tab", "switch focus"},
			{"j/k", "scroll"},
			{"Esc", "back"},
		}
	}

	var parts []string
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	for _, k := range keys {
		parts = append(parts, keyStyle.Render(k.key)+" "+descStyle.Render(k.desc))
	}

	footer := " " + strings.Join(parts, "  │  ")

	return lipgloss.NewStyle().
		Background(t.Surface).
		Width(c.width).
		Render(footer)
}

func (c *Chat) getLoadingSpinner() string {
	spinners := []string{"|", "/", "-", "\\"}
	return spinners[c.loadingFrame%len(spinners)]
}
