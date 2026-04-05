package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// newChatTestServer creates a server that supports both WebSocket and event POSTs,
// capturing control responses for verification.
func newChatTestServer(t *testing.T, wsHandler func(*websocket.Conn), controlResponses chan<- []map[string]any) (*httptest.Server, *Client) {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/sessions/ws/", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "done")

		_, msg, _ := ws.Read(r.Context())
		_ = msg
		resp, _ := json.Marshal(map[string]any{
			"type":     "control_response",
			"response": map[string]string{"subtype": "success"},
		})
		ws.Write(r.Context(), websocket.MessageText, resp)

		if wsHandler != nil {
			wsHandler(ws)
		}
	})

	mux.HandleFunc("/v1/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var body struct {
				Events []map[string]any `json:"events"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if controlResponses != nil && len(body.Events) > 0 {
				// Only forward control_response events
				first := body.Events[0]
				if t, ok := first["type"].(string); ok && t == "control_response" {
					controlResponses <- body.Events
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	client := NewClient("tok", "org", WithBaseURL(srv.URL))
	return srv, client
}

func TestChat_Send(t *testing.T) {
	srv, client := newChatTestServer(t, func(ws *websocket.Conn) {
		ctx := context.Background()

		// Stream 3 assistant events with growing text
		texts := []string{"He", "Hello", "Hello!"}
		for i, text := range texts {
			evt := Event{
				Type: "assistant",
				Message: &Message{
					ID:      "msg-1",
					Role:    "assistant",
					Content: []Content{{Type: "text", Text: text}},
				},
			}
			if i == len(texts)-1 {
				stop := "end_turn"
				evt.Message.StopReason = &stop
			}
			data, _ := json.Marshal(evt)
			ws.Write(ctx, websocket.MessageText, data)
			time.Sleep(10 * time.Millisecond)
		}

		result, _ := json.Marshal(Event{Type: "result", Subtype: "success"})
		ws.Write(ctx, websocket.MessageText, result)
	}, nil)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	chat := NewChat(conn)
	ch, err := chat.Send(ctx, "Hi")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var chunks []string
	for text := range ch {
		chunks = append(chunks, text)
	}

	full := strings.Join(chunks, "")
	if full != "Hello!" {
		t.Errorf("full text = %q, want %q", full, "Hello!")
	}
}

func TestChat_AutoAllow(t *testing.T) {
	controlCh := make(chan []map[string]any, 1)

	srv, client := newChatTestServer(t, func(ws *websocket.Conn) {
		ctx := context.Background()

		// Send a tool permission request
		toolReq := Event{
			Type:      "control_request",
			RequestID: "req-1",
			Request:   json.RawMessage(`{"subtype":"can_use_tool","tool_name":"Bash","tool_use_id":"tu-1","input":{"command":"ls"}}`),
		}
		data, _ := json.Marshal(toolReq)
		ws.Write(ctx, websocket.MessageText, data)

		time.Sleep(100 * time.Millisecond)

		// Then send assistant response and result
		stop := "end_turn"
		evt := Event{
			Type: "assistant",
			Message: &Message{
				ID:         "msg-1",
				Role:       "assistant",
				Content:    []Content{{Type: "text", Text: "Done"}},
				StopReason: &stop,
			},
		}
		data, _ = json.Marshal(evt)
		ws.Write(ctx, websocket.MessageText, data)

		result, _ := json.Marshal(Event{Type: "result", Subtype: "success"})
		ws.Write(ctx, websocket.MessageText, result)
	}, controlCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	chat := NewChat(conn, WithAutoAllow())
	ch, err := chat.Send(ctx, "Run ls")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Drain text channel
	for range ch {
	}

	// Verify control response was sent with "allow"
	select {
	case events := <-controlCh:
		resp := events[0]["response"].(map[string]any)
		inner := resp["response"].(map[string]any)
		if inner["behavior"] != "allow" {
			t.Errorf("behavior = %v, want allow", inner["behavior"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for control response")
	}
}

func TestChat_CustomHandler(t *testing.T) {
	controlCh := make(chan []map[string]any, 1)

	srv, client := newChatTestServer(t, func(ws *websocket.Conn) {
		ctx := context.Background()

		toolReq := Event{
			Type:      "control_request",
			RequestID: "req-2",
			Request:   json.RawMessage(`{"subtype":"can_use_tool","tool_name":"Bash","tool_use_id":"tu-2","input":{"command":"rm -rf /"}}`),
		}
		data, _ := json.Marshal(toolReq)
		ws.Write(ctx, websocket.MessageText, data)

		time.Sleep(100 * time.Millisecond)

		stop := "end_turn"
		evt := Event{
			Type: "assistant",
			Message: &Message{
				ID:         "msg-1",
				Role:       "assistant",
				Content:    []Content{{Type: "text", Text: "OK"}},
				StopReason: &stop,
			},
		}
		data, _ = json.Marshal(evt)
		ws.Write(ctx, websocket.MessageText, data)

		result, _ := json.Marshal(Event{Type: "result", Subtype: "success"})
		ws.Write(ctx, websocket.MessageText, result)
	}, controlCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	var handlerCalled bool
	chat := NewChat(conn, WithToolHandler(func(tr ToolRequest) bool {
		handlerCalled = true
		if tr.ToolName != "Bash" {
			t.Errorf("ToolName = %q, want Bash", tr.ToolName)
		}
		return false // deny
	}))

	ch, err := chat.Send(ctx, "Do something dangerous")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	for range ch {
	}

	if !handlerCalled {
		t.Error("custom tool handler was not called")
	}

	select {
	case events := <-controlCh:
		resp := events[0]["response"].(map[string]any)
		inner := resp["response"].(map[string]any)
		if inner["behavior"] != "deny" {
			t.Errorf("behavior = %v, want deny", inner["behavior"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for control response")
	}
}
