package claudecode

import "encoding/json"

// Event represents a streaming event from the Claude Code session protocol.
type Event struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	UUID      string          `json:"uuid,omitempty"`
	CreatedAt string          `json:"created_at,omitempty"`
	Message   *Message        `json:"message,omitempty"`
	Subtype   string          `json:"subtype,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Request   json.RawMessage `json:"request,omitempty"`
	Response  json.RawMessage `json:"response,omitempty"`
}

// Message represents an assistant or user message within an event.
type Message struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"`
	Model      string    `json:"model,omitempty"`
	Content    []Content `json:"content"`
	StopReason *string   `json:"stop_reason"`
	Usage      *Usage    `json:"usage,omitempty"`
}

// Content represents a single content block in a message.
type Content struct {
	Type    string   `json:"type"`
	Text    string   `json:"text,omitempty"`
	ToolUse *ToolUse `json:"tool_use,omitempty"`
}

// ToolUse represents a tool invocation by the assistant.
type ToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// Usage tracks token consumption for a message.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
