package claudecode

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const (
	keepAliveInterval = 50 * time.Second
	maxReconnectTries = 5
	maxErrorBodySize  = 1 << 20 // 1 MB
)

// Conn is a live WebSocket connection to a Claude Code session.
// Events stream from the server over the WebSocket; user messages
// are sent via HTTP POST through the parent Client.
//
// SendMessage and Events consume from the same internal channel.
// Only one may be used at a time — do not call SendMessage concurrently
// or mix SendMessage with Events on the same Conn.
//
// The caller must call Close when done to release resources.
type Conn struct {
	sessionID  string
	client     *Client
	ws         *websocket.Conn
	events     chan Event
	done       chan struct{}
	cancelRead context.CancelFunc // cancels readLoop's context
	closeOnce  sync.Once
	mu         sync.Mutex // guards ws during reconnect
}

// Connect opens a WebSocket to the session and performs the initialize handshake.
// The caller must call Close on the returned Conn to stop background goroutines.
func (c *Client) Connect(ctx context.Context, sessionID string) (*Conn, error) {
	readCtx, cancelRead := context.WithCancel(context.Background())

	conn := &Conn{
		sessionID:  sessionID,
		client:     c,
		events:     make(chan Event, 64),
		done:       make(chan struct{}),
		cancelRead: cancelRead,
	}

	if err := conn.dial(ctx); err != nil {
		cancelRead()
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := conn.initialize(ctx); err != nil {
		cancelRead()
		conn.ws.Close(websocket.StatusNormalClosure, "init failed")
		return nil, fmt.Errorf("initialize: %w", err)
	}

	go conn.readLoop(readCtx)
	go conn.keepAlive()

	return conn, nil
}

// dial opens the WebSocket connection.
func (conn *Conn) dial(ctx context.Context) error {
	wsURL, err := toWebSocketURL(conn.client.baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/sessions/ws/%s/subscribe?organization_uuid=%s&replay=true",
		wsURL, conn.sessionID, conn.client.orgUUID)

	headers := http.Header{}
	headers.Set(headerAuthorization, "Bearer "+conn.client.token)
	headers.Set(headerAnthropicVersion, "2023-06-01")
	headers.Set(headerAnthropicBeta, "ccr-byoc-2025-07-29")
	headers.Set(headerClientFeature, "ccr")
	headers.Set(headerClientPlatform, "web_claude_ai")
	headers.Set(headerClientVersion, "1.0.0")
	headers.Set(headerOrgUUID, conn.client.orgUUID)

	ws, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	conn.mu.Lock()
	conn.ws = ws
	conn.mu.Unlock()
	return nil
}

// toWebSocketURL converts an HTTP(S) URL to a WS(S) URL by changing only the scheme.
func toWebSocketURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	return u.String(), nil
}

// initialize sends the initialize control request and waits for success.
func (conn *Conn) initialize(ctx context.Context) error {
	initMsg := map[string]any{
		"type":       "control_request",
		"request_id": generateUUID(),
		"request":    map[string]string{"subtype": "initialize"},
	}

	data, err := json.Marshal(initMsg)
	if err != nil {
		return fmt.Errorf("marshal init: %w", err)
	}

	conn.mu.Lock()
	ws := conn.ws
	conn.mu.Unlock()

	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write init: %w", err)
	}

	// Wait for control_response. Relies on ctx for timeout.
	for {
		_, msg, err := ws.Read(ctx)
		if err != nil {
			return fmt.Errorf("read init response: %w", err)
		}
		var evt Event
		if err := json.Unmarshal(msg, &evt); err != nil {
			continue
		}
		if evt.Type == "control_response" {
			return nil
		}
	}
}

// readLoop reads newline-delimited JSON events from the WebSocket and sends them to the events channel.
func (conn *Conn) readLoop(ctx context.Context) {
	defer close(conn.events)

	for {
		select {
		case <-conn.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		conn.mu.Lock()
		ws := conn.ws
		conn.mu.Unlock()

		_, msg, err := ws.Read(ctx)
		if err != nil {
			select {
			case <-conn.done:
				return
			case <-ctx.Done():
				return
			default:
			}
			// Attempt reconnect
			if conn.reconnect() {
				continue
			}
			return
		}

		// Messages may be newline-delimited
		scanner := bufio.NewScanner(strings.NewReader(string(msg)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var evt Event
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}
			select {
			case conn.events <- evt:
			case <-conn.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

// keepAlive sends periodic keep_alive messages over the WebSocket.
func (conn *Conn) keepAlive() {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-conn.done:
			return
		case <-ticker.C:
			conn.mu.Lock()
			ws := conn.ws
			conn.mu.Unlock()

			msg, _ := json.Marshal(map[string]string{"type": "keep_alive"})
			if err := ws.Write(context.Background(), websocket.MessageText, msg); err != nil {
				// Write failed — connection is likely broken.
				// readLoop will detect the failure and trigger reconnect.
				return
			}
		}
	}
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
func (conn *Conn) reconnect() bool {
	for attempt := 0; attempt < maxReconnectTries; attempt++ {
		// Exponential backoff with jitter: 2^attempt * 1s + random 0-1s
		jitter, _ := rand.Int(rand.Reader, big.NewInt(1000))
		backoff := time.Duration(1<<uint(attempt))*time.Second + time.Duration(jitter.Int64())*time.Millisecond

		select {
		case <-conn.done:
			return false
		case <-time.After(backoff):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := conn.dial(ctx)
		cancel()
		if err != nil {
			continue
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		err = conn.initialize(ctx2)
		cancel2()
		if err != nil {
			continue
		}

		return true
	}
	return false
}

// SendMessage sends a user message via HTTP POST and returns a channel that
// streams the resulting events. The channel closes when a result event arrives.
//
// Only one SendMessage call may be active at a time. Do not use concurrently
// with Events() — both consume from the same internal channel.
func (conn *Conn) SendMessage(ctx context.Context, content string) (<-chan Event, error) {
	uuid := generateUUID()

	body := map[string]any{
		"events": []map[string]any{
			{
				"type":               "user",
				"uuid":               uuid,
				"session_id":         conn.sessionID,
				"parent_tool_use_id": nil,
				"message": map[string]string{
					"role":    "user",
					"content": content,
				},
			},
		},
	}

	path := fmt.Sprintf("/v1/sessions/%s/events", conn.sessionID)
	resp, err := conn.client.do(ctx, "POST", path, body)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	// Create a filtered channel that delivers events for this turn
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		for evt := range conn.events {
			select {
			case out <- evt:
			case <-ctx.Done():
				return
			}

			if evt.Type == "result" {
				return
			}
		}
	}()

	return out, nil
}

// Events returns the raw event channel for custom handling.
//
// This channel is mutually exclusive with SendMessage — do not use both
// on the same Conn. The channel closes when the connection is closed.
func (conn *Conn) Events() <-chan Event {
	return conn.events
}

// Close closes the WebSocket connection and stops background goroutines.
func (conn *Conn) Close() error {
	conn.closeOnce.Do(func() {
		close(conn.done)
		conn.cancelRead()
	})
	conn.mu.Lock()
	ws := conn.ws
	conn.mu.Unlock()
	if ws != nil {
		return ws.Close(websocket.StatusNormalClosure, "client closed")
	}
	return nil
}

// postEvents sends events via HTTP POST to the session events endpoint.
func (conn *Conn) postEvents(ctx context.Context, events []map[string]any) error {
	body := map[string]any{"events": events}
	path := fmt.Sprintf("/v1/sessions/%s/events", conn.sessionID)
	resp, err := conn.client.do(ctx, "POST", path, body)
	if err != nil {
		return fmt.Errorf("post events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}
	return nil
}

// generateUUID produces a UUID v4 using crypto/rand.
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
