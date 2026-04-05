package claudecode

import (
	"context"
	"encoding/json"
	"strings"
)

// ToolRequest contains details about a tool the assistant wants to use.
type ToolRequest struct {
	RequestID string
	ToolName  string
	ToolUseID string
	Input     map[string]any
}

// Chat is a high-level wrapper around Conn that handles tool permissions
// automatically and provides a simple text streaming interface.
type Chat struct {
	conn      *Conn
	autoAllow bool
	onTool    func(ToolRequest) bool
	// OnError is called when a tool permission response fails to send.
	// If nil, errors are silently discarded.
	OnError func(error)
}

// ChatOption configures a Chat.
type ChatOption func(*Chat)

// WithAutoAllow configures Chat to automatically allow all tool requests.
func WithAutoAllow() ChatOption {
	return func(ch *Chat) { ch.autoAllow = true }
}

// WithToolHandler sets a custom handler for tool permission requests.
// Return true to allow, false to deny.
func WithToolHandler(fn func(ToolRequest) bool) ChatOption {
	return func(ch *Chat) { ch.onTool = fn }
}

// WithErrorHandler sets a callback for errors during tool permission handling.
func WithErrorHandler(fn func(error)) ChatOption {
	return func(ch *Chat) { ch.OnError = fn }
}

// NewChat wraps a Conn with convenience features.
func NewChat(conn *Conn, opts ...ChatOption) *Chat {
	ch := &Chat{conn: conn}
	for _, o := range opts {
		o(ch)
	}
	return ch
}

// Send sends a message and returns a channel of text chunks.
// Tool permissions are handled automatically based on the Chat options.
// The channel closes when the assistant's turn is complete.
func (ch *Chat) Send(ctx context.Context, message string) (<-chan string, error) {
	eventCh, err := ch.conn.SendMessage(ctx, message)
	if err != nil {
		return nil, err
	}

	out := make(chan string, 64)
	go func() {
		defer close(out)
		var lastText string

		for evt := range eventCh {
			switch evt.Type {
			case "assistant":
				if evt.Message == nil {
					continue
				}
				var fullText string
				for _, c := range evt.Message.Content {
					if c.Type == "text" {
						fullText += c.Text
					}
				}
				// Compute delta — verify prefix match to handle non-append cases safely
				if len(fullText) > len(lastText) && strings.HasPrefix(fullText, lastText) {
					delta := fullText[len(lastText):]
					lastText = fullText
					select {
					case out <- delta:
					case <-ctx.Done():
						return
					}
				} else if fullText != lastText {
					// Content changed in a non-append way; send the full new text
					lastText = fullText
					select {
					case out <- fullText:
					case <-ctx.Done():
						return
					}
				}

			case "control_request":
				ch.handleToolRequest(ctx, evt)

			case "result":
				return
			}
		}
	}()

	return out, nil
}

// handleToolRequest processes a tool permission control request.
func (ch *Chat) handleToolRequest(ctx context.Context, evt Event) {
	if evt.Request == nil {
		return
	}

	var req struct {
		Subtype   string         `json:"subtype"`
		ToolName  string         `json:"tool_name"`
		ToolUseID string         `json:"tool_use_id"`
		Input     map[string]any `json:"input"`
	}
	if err := json.Unmarshal(evt.Request, &req); err != nil {
		return
	}

	if req.Subtype != "can_use_tool" {
		return
	}

	tr := ToolRequest{
		RequestID: evt.RequestID,
		ToolName:  req.ToolName,
		ToolUseID: req.ToolUseID,
		Input:     req.Input,
	}

	allow := ch.autoAllow
	if ch.onTool != nil {
		allow = ch.onTool(tr)
	}

	var err error
	if allow {
		err = ch.conn.AllowTool(ctx, tr.RequestID, tr.ToolUseID)
	} else {
		err = ch.conn.DenyTool(ctx, tr.RequestID, tr.ToolUseID)
	}
	if err != nil && ch.OnError != nil {
		ch.OnError(err)
	}
}
