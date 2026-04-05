package claudecode

import (
	"context"
	"fmt"
	"time"
)

// Session represents a Claude Code remote session.
type Session struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Model       string    `json:"-"`
	Environment string    `json:"-"`
}

// SessionOpts configures a new session.
type SessionOpts struct {
	Title         string
	EnvironmentID string
	Model         string
	SystemPrompt  string
	AllowedTools  []string
	Sources       []Source
}

// SessionUpdate contains fields that can be patched on a session.
// Only non-nil pointer fields are included in the PATCH request.
type SessionUpdate struct {
	Title  *string `json:"title,omitempty"`
	Status *string `json:"status,omitempty"`
}

// Source represents a code source (e.g., a git repository).
type Source struct {
	GitRepo string
}

// createSessionRequest is the POST /v1/sessions request body.
type createSessionRequest struct {
	Title         string         `json:"title"`
	EnvironmentID string         `json:"environment_id,omitempty"`
	Events        []any          `json:"events"`
	SessionCtx    sessionContext `json:"session_context"`
}

type sessionContext struct {
	Model              string        `json:"model"`
	AllowedTools       []string      `json:"allowed_tools,omitempty"`
	Sources            []sourceEntry `json:"sources,omitempty"`
	CustomSystemPrompt string        `json:"custom_system_prompt,omitempty"`
}

type sourceEntry struct {
	GitRepository gitRepo `json:"git_repository"`
}

type gitRepo struct {
	URL string `json:"url"`
}

// CreateSession creates a new remote session.
func (c *Client) CreateSession(ctx context.Context, opts SessionOpts) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	var sources []sourceEntry
	for _, s := range opts.Sources {
		sources = append(sources, sourceEntry{GitRepository: gitRepo{URL: s.GitRepo}})
	}

	reqBody := createSessionRequest{
		Title:         opts.Title,
		EnvironmentID: opts.EnvironmentID,
		Events:        []any{},
		SessionCtx: sessionContext{
			Model:              model,
			AllowedTools:       opts.AllowedTools,
			Sources:            sources,
			CustomSystemPrompt: opts.SystemPrompt,
		},
	}

	var session Session
	if err := c.doJSON(ctx, "POST", "/v1/sessions", reqBody, &session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	session.Model = model
	session.Environment = opts.EnvironmentID
	return &session, nil
}

// ListSessions returns all sessions for the organization.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.doJSON(ctx, "GET", "/v1/sessions", nil, &sessions); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

// GetSession retrieves a session by ID.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	var session Session
	if err := c.doJSON(ctx, "GET", "/v1/sessions/"+id, nil, &session); err != nil {
		return nil, fmt.Errorf("get session %s: %w", id, err)
	}
	return &session, nil
}

// UpdateSession patches a session with the given fields.
func (c *Client) UpdateSession(ctx context.Context, id string, updates SessionUpdate) error {
	if err := c.doJSON(ctx, "PATCH", "/v1/sessions/"+id, updates, nil); err != nil {
		return fmt.Errorf("update session %s: %w", id, err)
	}
	return nil
}
