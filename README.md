# claude-code-sdk-go

Go SDK for the Claude Code remote session protocol.

## Why this exists

Claude Code's remote sessions let you create AI-powered coding agents that run in cloud or bridge environments. The official TypeScript SDK wraps the CLI as a subprocess. This SDK takes a different approach: it speaks the native HTTP + WebSocket protocol directly.

What that gives you:

- **Real-time streaming** — assistant responses arrive as they're generated, token by token
- **Tool permission control** — approve, deny, or auto-allow tool use with typed handlers
- **Session management** — create, list, query, and update sessions programmatically
- **Environment discovery** — enumerate bridge and cloud environments
- **No CLI dependency** — pure Go, no subprocess, no shell wrapping

## Install

```bash
go get github.com/tfournet/claude-code-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    claudecode "github.com/tfournet/claude-code-sdk-go"
)

func main() {
    ctx := context.Background()

    // Create client from CLI credentials (~/.claude/.credentials.json)
    client, err := claudecode.NewClientFromCLI("your-org-uuid")
    if err != nil {
        log.Fatal(err)
    }

    // Create a session
    session, err := client.CreateSession(ctx, claudecode.SessionOpts{
        Title:        "My Session",
        Model:        "claude-sonnet-4-6",
        AllowedTools: []string{"Read", "Grep", "Glob"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Connect via WebSocket
    conn, err := client.Connect(ctx, session.ID)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Use Chat for simple text streaming with auto-allowed tools
    chat := claudecode.NewChat(conn, claudecode.WithAutoAllow())
    ch, err := chat.Send(ctx, "What files are in the current directory?")
    if err != nil {
        log.Fatal(err)
    }

    for text := range ch {
        fmt.Print(text)
    }
    fmt.Println()
}
```

## Architecture

The SDK mirrors the Claude Code remote protocol's dual-channel design:

- **HTTP** (client -> server): session CRUD, sending user messages, control responses
- **WebSocket** (server -> client): streaming assistant output, tool permission requests, keep-alive

```
             HTTP POST /v1/sessions/{id}/events
  Client  ─────────────────────────────────────>  claude.ai
          <─────────────────────────────────────
             WSS /v1/sessions/ws/{id}/subscribe
```

User messages are always sent via HTTP POST. Responses stream back over the WebSocket as they're generated.

## API Layers

### Low-level: Client + Conn

Full control over sessions, events, and tool permissions.

```go
client := claudecode.NewClient(token, orgUUID)

// Session CRUD
session, _ := client.CreateSession(ctx, claudecode.SessionOpts{...})
sessions, _ := client.ListSessions(ctx)
session, _ = client.GetSession(ctx, session.ID)

title := "New Title"
client.UpdateSession(ctx, session.ID, claudecode.SessionUpdate{Title: &title})

// List environments
envs, _ := client.ListEnvironments(ctx)

// WebSocket connection — caller must Close when done
conn, _ := client.Connect(ctx, session.ID)
defer conn.Close()

// Send message and handle raw events
events, _ := conn.SendMessage(ctx, "Hello")
for evt := range events {
    switch evt.Type {
    case "assistant":
        if evt.Message != nil && len(evt.Message.Content) > 0 {
            fmt.Print(evt.Message.Content[0].Text)
        }
    case "control_request":
        conn.AllowTool(ctx, evt.RequestID, "tool-use-id")
    case "result":
        fmt.Println("\nDone")
    }
}

// Controls
conn.Interrupt(ctx)
conn.SetPermissionMode(ctx, "bypassPermissions")
conn.SetModel(ctx, "claude-opus-4-6")
```

> **Note:** `SendMessage` and `Events()` both consume from the same internal channel.
> Only use one at a time on a given `Conn`.

### High-level: Chat

Auto-handles tool permissions and returns text-only streaming.

```go
// Auto-allow all tools
chat := claudecode.NewChat(conn, claudecode.WithAutoAllow())
texts, _ := chat.Send(ctx, "Summarize this codebase")
for chunk := range texts {
    fmt.Print(chunk)
}

// Custom tool handler — deny dangerous tools
chat = claudecode.NewChat(conn,
    claudecode.WithToolHandler(func(tr claudecode.ToolRequest) bool {
        if tr.ToolName == "Bash" {
            fmt.Printf("Denied Bash: %v\n", tr.Input["command"])
            return false
        }
        return true
    }),
    claudecode.WithErrorHandler(func(err error) {
        log.Printf("tool permission error: %v", err)
    }),
)
```

## Authentication

The SDK reads OAuth credentials from `~/.claude/.credentials.json`, the same file used by the Claude Code CLI. Run `claude` and log in first.

```go
// Automatic — reads token from CLI credentials
client, err := claudecode.NewClientFromCLI("your-org-uuid")

// Manual — provide token directly
client := claudecode.NewClient("sk-ant-oat01-...", "your-org-uuid")
```

The organization UUID is required and can be found in the claude.ai URL when logged into your organization.

## Environments

Connect to bridge environments (local machines running the Claude Code bridge) or Anthropic cloud:

```go
envs, _ := client.ListEnvironments(ctx)
for _, env := range envs {
    fmt.Printf("%s (%s) — %s, online=%v\n", env.Name, env.Kind, env.Machine, env.Online)
}

// Use a bridge environment
session, _ := client.CreateSession(ctx, claudecode.SessionOpts{
    EnvironmentID: envs[0].ID,
    Title:         "Bridge Session",
})
```

## Error Handling

The SDK uses sentinel errors for expected conditions and typed errors for API failures:

```go
_, err := claudecode.ReadCLICredentials()
if errors.Is(err, claudecode.ErrNoCredentials) {
    fmt.Println("Run 'claude' and log in first")
}
if errors.Is(err, claudecode.ErrTokenExpired) {
    fmt.Println("Token expired, re-authenticate")
}

var apiErr *claudecode.APIError
if errors.As(err, &apiErr) {
    fmt.Printf("API %d: %s\n", apiErr.StatusCode, apiErr.Body)
}
```

## Dependencies

- [`nhooyr.io/websocket`](https://github.com/nhooyr/websocket) — context-aware WebSocket client
- Standard library for everything else

## Testing

```bash
go test ./... -v -race
```

Tests use `httptest.NewServer` to mock both HTTP and WebSocket endpoints. No external services required.
