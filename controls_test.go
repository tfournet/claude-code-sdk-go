package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// newControlTestServer creates a server that captures POSTed events for verification.
func newControlTestServer(t *testing.T, gotEvents chan<- []map[string]any) (*httptest.Server, *Client) {
	t.Helper()

	mux := http.NewServeMux()

	// WebSocket endpoint with initialize handshake
	mux.HandleFunc("/v1/sessions/ws/", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "done")

		_, msg, err := ws.Read(r.Context())
		if err != nil {
			return
		}
		_ = msg
		resp, _ := json.Marshal(map[string]any{
			"type":     "control_response",
			"response": map[string]string{"subtype": "success"},
		})
		ws.Write(r.Context(), websocket.MessageText, resp)

		// Hold connection open
		ws.Read(r.Context())
	})

	// Capture POSTed events
	mux.HandleFunc("/v1/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var body struct {
				Events []map[string]any `json:"events"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if gotEvents != nil {
				gotEvents <- body.Events
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

func TestAllowTool(t *testing.T) {
	eventsCh := make(chan []map[string]any, 1)
	srv, client := newControlTestServer(t, eventsCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := conn.AllowTool(ctx, "req-42", "tu-7"); err != nil {
		t.Fatalf("AllowTool: %v", err)
	}

	select {
	case events := <-eventsCh:
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		evt := events[0]
		if evt["type"] != "control_response" {
			t.Errorf("type = %v, want control_response", evt["type"])
		}
		resp, ok := evt["response"].(map[string]any)
		if !ok {
			t.Fatal("response is not a map")
		}
		inner, ok := resp["response"].(map[string]any)
		if !ok {
			t.Fatal("response.response is not a map")
		}
		if inner["behavior"] != "allow" {
			t.Errorf("behavior = %v, want allow", inner["behavior"])
		}
		if inner["toolUseID"] != "tu-7" {
			t.Errorf("toolUseID = %v, want tu-7", inner["toolUseID"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for events")
	}
}

func TestDenyTool(t *testing.T) {
	eventsCh := make(chan []map[string]any, 1)
	srv, client := newControlTestServer(t, eventsCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := conn.DenyTool(ctx, "req-99", "tu-5"); err != nil {
		t.Fatalf("DenyTool: %v", err)
	}

	select {
	case events := <-eventsCh:
		evt := events[0]
		resp := evt["response"].(map[string]any)
		inner := resp["response"].(map[string]any)
		if inner["behavior"] != "deny" {
			t.Errorf("behavior = %v, want deny", inner["behavior"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestInterrupt(t *testing.T) {
	eventsCh := make(chan []map[string]any, 1)
	srv, client := newControlTestServer(t, eventsCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := conn.Interrupt(ctx); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	select {
	case events := <-eventsCh:
		evt := events[0]
		if evt["type"] != "control_request" {
			t.Errorf("type = %v, want control_request", evt["type"])
		}
		req := evt["request"].(map[string]any)
		if req["subtype"] != "interrupt" {
			t.Errorf("subtype = %v, want interrupt", req["subtype"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestSetPermissionMode(t *testing.T) {
	eventsCh := make(chan []map[string]any, 1)
	srv, client := newControlTestServer(t, eventsCh)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := conn.SetPermissionMode(ctx, "bypassPermissions"); err != nil {
		t.Fatalf("SetPermissionMode: %v", err)
	}

	select {
	case events := <-eventsCh:
		req := events[0]["request"].(map[string]any)
		if req["subtype"] != "set_permission_mode" {
			t.Errorf("subtype = %v, want set_permission_mode", req["subtype"])
		}
		if req["permission_mode"] != "bypassPermissions" {
			t.Errorf("permission_mode = %v, want bypassPermissions", req["permission_mode"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
