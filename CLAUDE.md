# claude-code-sdk-go

Go SDK for the Claude Code remote session protocol. Direct WebSocket/HTTP communication with claude.ai — no CLI subprocess wrapper.

## Quick Reference

```bash
go build ./...                    # build
go test ./... -v -race            # test
go vet ./...                      # lint
```

## Architecture

```
claudecode/
  client.go          Client struct, auth, HTTP helpers
  session.go         Session CRUD (create, list, get, update)
  conn.go            WebSocket connection, streaming, keep-alive
  events.go          Event types, JSON parsing
  controls.go        Tool permissions, interrupt, set_model
  environments.go    Environment listing
  auth.go            OAuth token from CLI credentials
  errors.go          Error types
```

Single package: `claudecode`. No sub-packages. No frameworks.

## Non-Negotiable Rules

- **Standard library + one WebSocket dep.** No other external dependencies.
- **OAuth auth only.** No browser cookie support. Read token from `~/.claude/` credentials.
- **Streaming via channels.** `SendMessage` returns `<-chan Event`, caller ranges over it.
- **Context everywhere.** Every public method takes `context.Context` for cancellation/timeout.
- **Fail closed.** Auth failures, WebSocket drops, malformed JSON → return error, never silently continue.

## Testing

- Mock HTTP/WebSocket server for unit tests
- Use `httptest.NewServer` and `httptest.NewTLSServer`
- Table-driven tests for JSON parsing
- No assertion libraries — use `if/t.Errorf`
- Always run with `-race`

## Dependencies

- `nhooyr.io/websocket` (WebSocket client — better API than gorilla, context-aware)
- Standard library for everything else
