package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSession(t *testing.T) {
	var gotBody createSessionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/sessions" {
			t.Errorf("path = %s, want /v1/sessions", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)

		json.NewEncoder(w).Encode(map[string]any{
			"id":     "sess-123",
			"title":  "Test Session",
			"status": "active",
		})
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	session, err := c.CreateSession(context.Background(), SessionOpts{
		Title:         "Test Session",
		EnvironmentID: "env-1",
		Model:         "claude-sonnet-4-6",
		SystemPrompt:  "You are helpful",
		AllowedTools:  []string{"Read", "Grep"},
		Sources:       []Source{{GitRepo: "https://github.com/example/repo"}},
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if session.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", session.ID, "sess-123")
	}

	// Verify request body structure
	if gotBody.Title != "Test Session" {
		t.Errorf("body.Title = %q, want %q", gotBody.Title, "Test Session")
	}
	if gotBody.EnvironmentID != "env-1" {
		t.Errorf("body.EnvironmentID = %q, want %q", gotBody.EnvironmentID, "env-1")
	}
	if gotBody.SessionCtx.Model != "claude-sonnet-4-6" {
		t.Errorf("body.SessionCtx.Model = %q, want %q", gotBody.SessionCtx.Model, "claude-sonnet-4-6")
	}
	if len(gotBody.SessionCtx.AllowedTools) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(gotBody.SessionCtx.AllowedTools))
	}
	if gotBody.SessionCtx.CustomSystemPrompt != "You are helpful" {
		t.Errorf("CustomSystemPrompt = %q, want %q", gotBody.SessionCtx.CustomSystemPrompt, "You are helpful")
	}
}

func TestCreateSession_DefaultModel(t *testing.T) {
	var gotBody createSessionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{"id": "s1", "status": "active"})
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	_, err := c.CreateSession(context.Background(), SessionOpts{Title: "Minimal"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if gotBody.SessionCtx.Model != "claude-sonnet-4-6" {
		t.Errorf("default model = %q, want claude-sonnet-4-6", gotBody.SessionCtx.Model)
	}
}

func TestListSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "s1", "title": "Session 1", "status": "active"},
			{"id": "s2", "title": "Session 2", "status": "idle"},
		})
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	sessions, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].ID != "s1" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "s1")
	}
}

func TestGetSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions/sess-42" {
			t.Errorf("path = %s, want /v1/sessions/sess-42", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "sess-42",
			"title":  "Found",
			"status": "active",
		})
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	s, err := c.GetSession(context.Background(), "sess-42")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.ID != "sess-42" {
		t.Errorf("ID = %q, want %q", s.ID, "sess-42")
	}
}

func TestUpdateSession(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody SessionUpdate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	title := "Updated Title"
	err := c.UpdateSession(context.Background(), "sess-7", SessionUpdate{Title: &title})
	if err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	if gotMethod != "PATCH" {
		t.Errorf("method = %s, want PATCH", gotMethod)
	}
	if gotPath != "/v1/sessions/sess-7" {
		t.Errorf("path = %s, want /v1/sessions/sess-7", gotPath)
	}
	if gotBody.Title == nil || *gotBody.Title != "Updated Title" {
		t.Errorf("body.Title = %v, want %q", gotBody.Title, "Updated Title")
	}
}

func TestUpdateSession_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	title := "x"
	err := c.UpdateSession(context.Background(), "bad-id", SessionUpdate{Title: &title})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
