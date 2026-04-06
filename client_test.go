package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-token", "org-123")

	if c.token != "test-token" {
		t.Errorf("token = %q, want %q", c.token, "test-token")
	}
	if c.orgUUID != "org-123" {
		t.Errorf("orgUUID = %q, want %q", c.orgUUID, "org-123")
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	custom := &http.Client{}
	c := NewClient("tok", "org", WithBaseURL("https://custom.api"), WithHTTPClient(custom))

	if c.baseURL != "https://custom.api" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://custom.api")
	}
	if c.http != custom {
		t.Error("HTTP client was not set by WithHTTPClient")
	}
}

func TestNewClientFromCLI_NoOrg(t *testing.T) {
	_, err := NewClientFromCLI("")
	if !errors.Is(err, ErrNoOrganization) {
		t.Errorf("err = %v, want ErrNoOrganization", err)
	}
}

func TestAuthHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewClient("my-token", "my-org", WithBaseURL(srv.URL))
	ctx := context.Background()

	resp, err := c.do(ctx, "GET", "/test", nil)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	checks := map[string]string{
		headerAuthorization:    "Bearer my-token",
		headerAnthropicVersion: "2023-06-01",
		headerAnthropicBeta:    "ccr-byoc-2025-07-29",
		headerClientFeature:    "ccr",
		headerClientPlatform:   "web_claude_ai",
		headerClientVersion:    "1.0.0",
		headerOrgUUID:          "my-org",
		headerContentType:      "application/json",
		"User-Agent":           "claude-code-sdk-go/1.0.0",
		"Accept":               "application/json",
		"Accept-Language":      "en-US,en;q=0.9",
	}
	for header, want := range checks {
		got := gotHeaders.Get(header)
		if got != want {
			t.Errorf("header %s = %q, want %q", header, got, want)
		}
	}

	// Accept-Encoding is set by the transport, not by us. Just verify it's present.
	if gotHeaders.Get("Accept-Encoding") == "" {
		t.Error("Accept-Encoding header is missing")
	}
}

func TestDoJSON_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient("tok", "org", WithBaseURL(srv.URL))
	var out map[string]any
	err := c.doJSON(context.Background(), "GET", "/fail", nil, &out)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

func TestNewClient_DefaultHasCookieJar(t *testing.T) {
	c := NewClient("tok", "org")

	if c.http.Jar == nil {
		t.Error("default HTTP client has no cookie jar")
	}
}

func TestNewClient_DefaultHasTimeout(t *testing.T) {
	c := NewClient("tok", "org")

	if c.http.Timeout == 0 {
		t.Error("default HTTP client has no timeout")
	}
}
