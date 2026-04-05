# Implementation Prompt

Use this prompt to start a Claude Code session that implements the SDK.

---

You are implementing `claude-code-sdk-go` — a Go SDK for the Claude Code remote session protocol. This is the first Go implementation of the direct WebSocket/HTTP protocol to claude.ai (no CLI subprocess wrapper).

## Context

Read these files first:
- `CLAUDE.md` — project rules and architecture
- `docs/PLAN.md` — complete implementation plan with protocol details, API shapes, and test specs

## What to build

Implement all 4 phases in order. Each phase has specific files, types, and tests defined in the plan.

**Phase 1:** Core client + session CRUD + environments + auth
**Phase 2:** WebSocket streaming (connect, send message, receive events, keep-alive, reconnect)
**Phase 3:** Controls (tool permissions, interrupt, permission mode, model switching)
**Phase 4:** Convenience Chat wrapper (auto-allow tools, text-only streaming)

## Rules

- TDD: write tests first, then implementation
- `go test ./... -v -race` must pass after each phase
- Single package `claudecode` — no sub-packages
- Only dependency: `nhooyr.io/websocket`
- Standard library for everything else
- `if/t.Errorf` for tests, no assertion libraries
- Context on every public method
- Wrap errors with `fmt.Errorf("context: %w", err)`

## Auth investigation needed

The plan assumes OAuth tokens are stored in `~/.claude/`. You need to investigate the actual file format:
```bash
ls -la ~/.claude/
find ~/.claude/ -name "*.json" -o -name "*credential*" -o -name "*token*" -o -name "*auth*"
```
Read whatever files exist and determine how to extract the OAuth bearer token and organization UUID.

## Verification

After each phase, run:
```bash
go build ./...
go test ./... -v -race
go vet ./...
```

## Commit style

One commit per phase:
```
feat: Phase 1 — core client, session CRUD, environments, auth
feat: Phase 2 — WebSocket streaming, keep-alive, reconnect
feat: Phase 3 — tool permissions, interrupt, controls
feat: Phase 4 — Chat convenience wrapper
```
