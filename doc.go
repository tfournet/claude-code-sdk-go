// Package claudecode provides a Go SDK for the Claude Code remote session protocol.
//
// It communicates directly with claude.ai over HTTP and WebSocket — no CLI subprocess,
// no shell wrapping. The SDK supports session management, real-time streaming of
// assistant responses, tool permission control, and a high-level Chat convenience wrapper.
//
// Architecture:
//
// The protocol uses a dual-channel design:
//   - HTTP REST (client → server): user messages, control responses, session CRUD
//   - WebSocket (server → client): streaming assistant output, tool permissions, keep-alive
//
// Quick start:
//
//	client, _ := claudecode.NewClientFromCLI("your-org-uuid")
//	session, _ := client.CreateSession(ctx, claudecode.SessionOpts{Title: "My Session"})
//	conn, _ := client.Connect(ctx, session.ID)
//	defer conn.Close()
//
//	chat := claudecode.NewChat(conn, claudecode.WithAutoAllow())
//	texts, _ := chat.Send(ctx, "Hello!")
//	for chunk := range texts {
//	    fmt.Print(chunk)
//	}
package claudecode
