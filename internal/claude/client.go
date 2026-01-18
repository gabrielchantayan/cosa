// Package claude implements integration with the Claude Code CLI.
package claude

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client manages a Claude Code CLI process.
type Client struct {
	binary    string
	model     string
	maxTurns  int
	sessionID string
	workdir   string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu     sync.Mutex
	events chan Event
	done   chan struct{}
}

// ClientConfig configures a Claude client.
type ClientConfig struct {
	Binary   string
	Model    string
	MaxTurns int
	Workdir  string
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
		binary:   cfg.Binary,
		model:    cfg.Model,
		maxTurns: cfg.MaxTurns,
		workdir:  cfg.Workdir,
		events:   make(chan Event, 100),
		done:     make(chan struct{}),
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
	args := c.buildArgs(prompt)

	c.cmd = exec.CommandContext(ctx, c.binary, args...)
	if c.workdir != "" {
		c.cmd.Dir = c.workdir
	}

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
		if line == "" {
			continue
		}

		event, err := parser.ParseLine(line)
		if err != nil {
			c.events <- Event{
				Type:  EventError,
				Error: fmt.Sprintf("parse error: %v", err),
			}
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
		c.events <- Event{
			Type:    EventError,
			Error:   scanner.Text(),
		}
	}
}

func (c *Client) waitForExit() {
	if c.cmd != nil {
		c.cmd.Wait()
	}
	close(c.done)
	close(c.events)
}
