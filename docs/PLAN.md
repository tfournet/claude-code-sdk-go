# Implementation Plan

## Protocol Reference

Source: `claude-remote-protocol` npm package v0.3.2 (reverse-engineered from claude.ai frontend traffic).

### Base URL
`https://claude.ai` (configurable via `WithBaseURL`)

### Authentication
OAuth bearer token from Claude Code CLI stored credentials (`~/.claude/`).

Every HTTP request requires these headers:
```
Authorization: Bearer {oauth_token}
anthropic-version: 2023-06-01
anthropic-beta: ccr-byoc-2025-07-29
anthropic-client-feature: ccr
anthropic-client-platform: web_claude_ai
anthropic-client-version: 1.0.0
x-organization-uuid: {org_uuid}
Content-Type: application/json
```

### Dual-Channel Architecture
- **HTTP REST** (client → server): user messages, control responses, session CRUD
- **WebSocket** (server → client): streaming assistant output, tool permissions, keep-alive

User messages are ALWAYS sent via HTTP POST, never over WebSocket.

---

## Phase 1: Core Client + Session CRUD

### Files to create

#### `go.mod`
```
module github.com/tfournet/claude-code-sdk-go
go 1.22
require nhooyr.io/websocket v1.8.17
```

#### `client.go`
```go
type Client struct {
    baseURL  string // default "https://claude.ai"
    orgUUID  string
    token    string // OAuth bearer token
    http     *http.Client
}

type Option func(*Client)
func WithBaseURL(url string) Option
func WithHTTPClient(c *http.Client) Option

func NewClient(token, orgUUID string, opts ...Option) *Client
func NewClientFromCLI() (*Client, error)  // reads ~/.claude/ credentials

// Internal helpers
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error)
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error
```

The `do` method sets all required headers and handles JSON marshaling.

#### `auth.go`
```go
// ReadCLICredentials finds the OAuth token and org UUID from ~/.claude/
// The exact file format needs investigation — check what files exist in
// ~/.claude/ after running `claude /login`.
func ReadCLICredentials() (token, orgUUID string, err error)
```

Look in `~/.claude/` for files containing OAuth tokens. Common locations:
- `~/.claude/credentials.json`
- `~/.claude/.credentials`
- `~/.claude/config.json`
The exact format needs to be reverse-engineered from the CLI's stored data.

#### `session.go`
```go
type Session struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Status      string    `json:"status"` // active, idle, archived
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Model       string    // extracted from session_context
    Environment string    // environment_id
}

type SessionOpts struct {
    Title        string
    EnvironmentID string
    Model        string   // default "claude-sonnet-4-6"
    SystemPrompt string
    AllowedTools []string // e.g. ["Read", "Grep", "Bash", "Glob"]
    Sources      []Source
}

type Source struct {
    GitRepo string // e.g. "https://github.com/rewstapp/riftwing"
}

// POST /v1/sessions
func (c *Client) CreateSession(ctx context.Context, opts SessionOpts) (*Session, error)

// GET /v1/sessions
func (c *Client) ListSessions(ctx context.Context) ([]Session, error)

// GET /v1/sessions/{id}
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error)

// PATCH /v1/sessions/{id}
func (c *Client) UpdateSession(ctx context.Context, id string, updates map[string]any) error
```

Create session POST body format:
```json
{
  "title": "...",
  "environment_id": "env_01...",
  "events": [],
  "session_context": {
    "model": "claude-sonnet-4-6",
    "allowed_tools": ["Read", "Grep", "Bash"],
    "sources": [{"git_repository": {"url": "https://github.com/..."}}],
    "custom_system_prompt": "..."
  }
}
```

#### `environments.go`
```go
type Environment struct {
    ID        string `json:"environment_id"`
    Name      string `json:"name"`
    Kind      string `json:"kind"`      // "bridge" or "anthropic_cloud"
    State     string `json:"state"`     // "active", etc.
    Online    bool   // extracted from bridge_info.online
    Machine   string // extracted from bridge_info.machine_name
    Directory string // extracted from bridge_info.directory
}

// GET /v1/environment_providers/private/organizations/{org}/environments
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error)
```

#### `events.go`
```go
type Event struct {
    Type      string   `json:"type"`      // assistant, user, result, control_request, control_response, keep_alive
    SessionID string   `json:"session_id"`
    UUID      string   `json:"uuid"`
    CreatedAt string   `json:"created_at"`
    Message   *Message `json:"message,omitempty"`
    // Result fields (for type=result)
    Subtype   string   `json:"subtype,omitempty"`
    IsError   bool     `json:"is_error,omitempty"`
    // Control fields
    RequestID string          `json:"request_id,omitempty"`
    Request   json.RawMessage `json:"request,omitempty"`
    Response  json.RawMessage `json:"response,omitempty"`
}

type Message struct {
    ID         string    `json:"id"`
    Role       string    `json:"role"`
    Model      string    `json:"model"`
    Content    []Content `json:"content"`
    StopReason *string   `json:"stop_reason"` // null while streaming, "end_turn" when done
    Usage      *Usage    `json:"usage,omitempty"`
}

type Content struct {
    Type    string   `json:"type"` // text, tool_use, thinking
    Text    string   `json:"text,omitempty"`
    ToolUse *ToolUse `json:"omitempty"` // populated when type=tool_use
}

type ToolUse struct {
    ID    string         `json:"id"`
    Name  string         `json:"name"`
    Input map[string]any `json:"input"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

### Tests for Phase 1

- `TestNewClient` — creates client with correct defaults
- `TestNewClientFromCLI` — reads credentials (mock the file)
- `TestCreateSession` — mock HTTP server, verify request body format and headers
- `TestListSessions` — mock returns paginated list
- `TestGetSession` — mock returns session by ID
- `TestListEnvironments` — mock returns environments with bridge_info parsing
- `TestAuthHeaders` — every request has all required headers

---

## Phase 2: WebSocket Streaming

#### `conn.go`
```go
type Conn struct {
    sessionID string
    client    *Client
    ws        *websocket.Conn
    events    chan Event
    done      chan struct{}
    closeOnce sync.Once
}

// Connect opens a WebSocket and sends the initialize control request.
// wss://claude.ai/v1/sessions/ws/{id}/subscribe?organization_uuid={org}
func (c *Client) Connect(ctx context.Context, sessionID string) (*Conn, error)

// SendMessage sends a user message via HTTP POST and returns a channel
// that streams events (assistant chunks, result, control requests) from
// the WebSocket. The channel closes when stop_reason is set or ctx is cancelled.
func (sc *Conn) SendMessage(ctx context.Context, content string) (<-chan Event, error)

// Events returns the raw event channel for custom handling.
func (sc *Conn) Events() <-chan Event

// Close closes the WebSocket connection.
func (sc *Conn) Close() error
```

WebSocket lifecycle:
1. Dial `wss://claude.ai/v1/sessions/ws/{id}/subscribe?organization_uuid={org}&replay=true`
2. Send `{"type":"control_request","request_id":"...","request":{"subtype":"initialize"}}`
3. Wait for `control_response` with `subtype: "success"`
4. Start keep-alive goroutine (send `{"type":"keep_alive"}` every 50s)
5. Start read loop: read newline-delimited JSON, parse Event, send to channel
6. On disconnect: exponential backoff reconnect (2^n * 1000ms + jitter, max 5 retries)

Message sending:
1. Generate UUID for the event
2. HTTP POST to `/v1/sessions/{id}/events` with:
```json
{
  "events": [{
    "type": "user",
    "uuid": "{uuid}",
    "session_id": "{session_id}",
    "parent_tool_use_id": null,
    "message": {"role": "user", "content": "{content}"}
  }]
}
```
3. Stream response events arrive on the WebSocket

Streaming detection:
- Same `message.id` appears multiple times as content builds
- `stop_reason: null` → still streaming
- `stop_reason: "end_turn"` → done, close the per-message channel
- `type: "result"` → agent turn complete

### Tests for Phase 2

- `TestConnect` — mock WebSocket server, verify initialize handshake
- `TestSendMessage_Streaming` — mock WS sends 3 assistant events with growing content, verify channel delivers them
- `TestSendMessage_StopReason` — channel closes when stop_reason is "end_turn"
- `TestKeepAlive` — verify keep_alive sent within 50s window
- `TestReconnect` — drop WS, verify client reconnects with backoff
- `TestConnClose` — verify clean shutdown

---

## Phase 3: Controls

#### `controls.go`
```go
// AllowTool responds to a can_use_tool control request with "allow".
func (sc *Conn) AllowTool(requestID, toolUseID string) error

// DenyTool responds with "deny".
func (sc *Conn) DenyTool(requestID, toolUseID string) error

// Interrupt sends an interrupt control request.
func (sc *Conn) Interrupt() error

// SetPermissionMode sets the session permission mode.
// Modes: "default", "acceptEdits", "bypassPermissions", "plan"
func (sc *Conn) SetPermissionMode(mode string) error

// SetModel changes the model mid-session.
func (sc *Conn) SetModel(model string) error
```

Tool permission flow:
1. Server sends `control_request` with `subtype: "can_use_tool"` over WebSocket
2. Contains: `tool_name`, `tool_use_id`, `input`, `description`
3. Client responds via HTTP POST to `/v1/sessions/{id}/events`:
```json
{
  "events": [{
    "type": "control_response",
    "response": {
      "subtype": "success",
      "request_id": "{request_id}",
      "response": {
        "behavior": "allow",
        "updatedInput": {},
        "toolUseID": "{tool_use_id}"
      }
    }
  }]
}
```

### Tests for Phase 3

- `TestAllowTool` — verify correct HTTP POST body
- `TestDenyTool` — verify deny behavior
- `TestInterrupt` — verify interrupt control request format
- `TestSetPermissionMode` — verify mode change

---

## Phase 4: Convenience Layer

#### `chat.go`
```go
// Chat is a high-level wrapper that handles tool permissions automatically
// and provides a simple send/receive text interface.
type Chat struct {
    conn          *Conn
    autoAllow     bool // auto-allow all tool permissions
    onToolRequest func(ToolRequest) bool // custom permission handler
}

type ChatOption func(*Chat)
func WithAutoAllow() ChatOption // auto-allow all tools
func WithToolHandler(fn func(ToolRequest) bool) ChatOption

// NewChat wraps a Conn with convenience features.
func NewChat(conn *Conn, opts ...ChatOption) *Chat

// Send sends a message and returns streaming text chunks.
// Handles tool permissions automatically based on options.
func (ch *Chat) Send(ctx context.Context, message string) (<-chan string, error)
```

The `Send` method:
1. Calls `conn.SendMessage`
2. Ranges over events
3. For `assistant` events: extract new text (delta from previous), send to output channel
4. For `control_request` (tool permission): auto-allow or call handler
5. For `result`: close output channel
6. Returns only text strings, not full Event structs

### Tests for Phase 4

- `TestChat_Send` — verify text-only output channel
- `TestChat_AutoAllow` — tool requests auto-allowed
- `TestChat_CustomHandler` — custom handler called, can deny
