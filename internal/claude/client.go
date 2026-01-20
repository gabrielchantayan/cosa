// Package claude implements integration with the Claude Code CLI.
package claude

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Client manages a Claude Code CLI process.
type Client struct {
	binary    string
	model     string
	maxTurns  int
	sessionID string
	workdir   string
	mcpConfig string // Path to MCP config file

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu           sync.Mutex
	events       chan Event
	done         chan struct{}
	stderrBuffer []string // Collects stderr for error reporting
}

// ClientConfig configures a Claude client.
type ClientConfig struct {
	Binary    string
	Model     string
	MaxTurns  int
	Workdir   string
	MCPConfig string // Path to MCP config file (optional)
}

// NewClient creates a new Claude Code client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.Binary == "" {
		cfg.Binary = "claude"
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 100
	}

	return &Client{
		binary:    cfg.Binary,
		model:     cfg.Model,
		maxTurns:  cfg.MaxTurns,
		workdir:   cfg.Workdir,
		mcpConfig: cfg.MCPConfig,
		events:    make(chan Event, 100),
		done:      make(chan struct{}),
	}
}

// Events returns a channel of events from the Claude session.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Done returns a channel that closes when the session ends.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// SessionID returns the current session ID (if resumed).
func (c *Client) SessionID() string {
	return c.sessionID
}

// Start begins a new Claude session with the given prompt.
func (c *Client) Start(ctx context.Context, prompt string) error {
	// Reset channels for new session (in case client is reused)
	c.mu.Lock()
	c.events = make(chan Event, 100)
	c.done = make(chan struct{})
	c.stderrBuffer = nil
	c.mu.Unlock()

	args := c.buildArgs(prompt)

	// Use 'script' to allocate a PTY for Claude
	// This is needed because Node.js (Claude) buffers stdout when connected to a pipe
	// but writes immediately when connected to a terminal/PTY
	claudeCmd := c.binary
	for _, arg := range args {
		// Escape single quotes in arguments
		escaped := strings.ReplaceAll(arg, "'", "'\"'\"'")
		claudeCmd += " '" + escaped + "'"
	}

	c.cmd = exec.CommandContext(ctx, "script", "-q", "/dev/null", "/bin/bash", "-c", claudeCmd)
	if c.workdir != "" {
		c.cmd.Dir = c.workdir
	}

	// Set up clean environment for Claude:
	// - Filter NODE_OPTIONS to prevent debugger from blocking startup
	env := filterEnv(os.Environ(), "NODE_OPTIONS")
	c.cmd.Env = env

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// Start reading output
	go c.readOutput()
	go c.readErrors()
	go c.waitForExit()

	return nil
}

// Resume continues an existing session.
func (c *Client) Resume(ctx context.Context, sessionID string, prompt string) error {
	c.sessionID = sessionID
	return c.Start(ctx, prompt)
}

// SendInput sends additional input to the running session.
func (c *Client) SendInput(input string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return fmt.Errorf("session not started")
	}

	_, err := fmt.Fprintln(c.stdin, input)
	return err
}

// Stop terminates the Claude session.
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}

	return nil
}

func (c *Client) buildArgs(prompt string) []string {
	args := []string{
		"--print",
		"--verbose",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
	}

	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	if c.maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.maxTurns))
	}

	if c.sessionID != "" {
		args = append(args, "--resume", c.sessionID)
	}

	if c.mcpConfig != "" {
		args = append(args, "--mcp-config", c.mcpConfig)
	}

	args = append(args, "-p", prompt)

	return args
}

func (c *Client) readOutput() {
	parser := NewParser()
	scanner := bufio.NewScanner(c.stdout)

	// Handle very long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Filter PTY control characters - find the JSON start
		// PTY output may have control chars like ^D or escape sequences
		jsonStart := strings.Index(line, "{")
		if jsonStart == -1 {
			// Not a JSON line, skip (likely terminal control sequences)
			continue
		}
		if jsonStart > 0 {
			line = line[jsonStart:]
		}

		event, err := parser.ParseLine(line)
		if err != nil {
			// Silently skip lines that don't parse as valid JSON
			// These are likely terminal control sequences
			continue
		}

		if event != nil {
			// Capture session ID from init message
			if event.Type == EventInit && event.SessionID != "" {
				c.sessionID = event.SessionID
			}
			c.events <- *event
		}
	}
}

func (c *Client) readErrors() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		c.mu.Lock()
		c.stderrBuffer = append(c.stderrBuffer, line)
		c.mu.Unlock()
		c.events <- Event{
			Type:  EventError,
			Error: line,
		}
	}
}

// StderrOutput returns all collected stderr output.
func (c *Client) StderrOutput() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stderrBuffer) == 0 {
		return ""
	}
	return strings.Join(c.stderrBuffer, "\n")
}

func (c *Client) waitForExit() {
	if c.cmd != nil {
		c.cmd.Wait()
	}
	close(c.done)
	close(c.events)
}

// filterEnv returns a copy of env with the specified keys removed.
func filterEnv(env []string, keys ...string) []string {
	result := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, key := range keys {
			if strings.HasPrefix(e, key+"=") {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, e)
		}
	}
	return result
}
