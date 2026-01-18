package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"cosa/internal/protocol"
)

// Client connects to the Cosa daemon.
type Client struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex

	// Request ID counter
	nextID int64

	// Pending requests waiting for response
	pending   map[int64]chan *protocol.Response
	pendingMu sync.Mutex

	// Notification handler
	onNotification func(*protocol.Request)

	// Event channel for streaming
	events   chan *LedgerEvent
	eventsMu sync.Mutex
}

// LedgerEvent represents an event from the ledger for streaming.
type LedgerEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Connect establishes a connection to the daemon.
func Connect(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	c := &Client{
		socketPath: socketPath,
		conn:       conn,
		pending:    make(map[int64]chan *protocol.Response),
		events:     make(chan *LedgerEvent, 100),
	}

	go c.readLoop()

	return c, nil
}

// Close closes the connection to the daemon.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// OnNotification sets a handler for incoming notifications.
func (c *Client) OnNotification(handler func(*protocol.Request)) {
	c.onNotification = handler
}

// Call sends a request and waits for a response.
func (c *Client) Call(method string, params interface{}) (*protocol.Response, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	reqID := protocol.NewIntID(id)

	req, err := protocol.NewRequest(reqID, method, params)
	if err != nil {
		return nil, err
	}

	// Register pending response channel
	respCh := make(chan *protocol.Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Send request
	if err := c.send(req); err != nil {
		return nil, err
	}

	// Wait for response
	resp := <-respCh
	return resp, nil
}

// Notify sends a notification (no response expected).
func (c *Client) Notify(method string, params interface{}) error {
	req, err := protocol.NewNotification(method, params)
	if err != nil {
		return err
	}
	return c.send(req)
}

// Subscribe subscribes to real-time events.
func (c *Client) Subscribe(events []string) error {
	resp, err := c.Call(protocol.MethodSubscribe, protocol.SubscribeParams{Events: events})
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("subscribe failed: %s", resp.Error.Message)
	}
	return nil
}

// ReadEvent reads the next event from the subscription.
func (c *Client) ReadEvent() (*LedgerEvent, error) {
	event, ok := <-c.events
	if !ok {
		return nil, fmt.Errorf("connection closed")
	}
	return event, nil
}

// Status gets the daemon status.
func (c *Client) Status() (*protocol.StatusResult, error) {
	resp, err := c.Call(protocol.MethodStatus, nil)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("status failed: %s", resp.Error.Message)
	}

	var result protocol.StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Shutdown requests the daemon to shut down.
func (c *Client) Shutdown() error {
	resp, err := c.Call(protocol.MethodShutdown, nil)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("shutdown failed: %s", resp.Error.Message)
	}
	return nil
}

func (c *Client) send(req *protocol.Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err = c.conn.Write(append(data, '\n'))
	return err
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Try to parse as response first
		var resp protocol.Response
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != nil {
			// It's a response
			var id int64
			if resp.ID.Num != nil {
				id = *resp.ID.Num
			}

			c.pendingMu.Lock()
			if ch, ok := c.pending[id]; ok {
				ch <- &resp
			}
			c.pendingMu.Unlock()
			continue
		}

		// Try to parse as notification
		var notif protocol.Request
		if err := json.Unmarshal(line, &notif); err == nil && notif.ID == nil {
			if c.onNotification != nil {
				c.onNotification(&notif)
			}

			// If it's a log entry notification, parse and send to events channel
			if notif.Method == protocol.NotifyLogEntry && notif.Params != nil {
				var event LedgerEvent
				if err := json.Unmarshal(notif.Params, &event); err == nil {
					select {
					case c.events <- &event:
					default:
						// Channel full, drop event
					}
				}
			}
		}
	}
}

// IsRunning checks if the daemon is running by attempting to connect.
func IsRunning(socketPath string) bool {
	client, err := Connect(socketPath)
	if err != nil {
		return false
	}
	defer client.Close()

	_, err = client.Status()
	return err == nil
}
