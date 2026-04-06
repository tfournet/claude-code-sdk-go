package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

const (
	defaultBaseURL = "https://claude.ai"

	headerAuthorization    = "Authorization"
	headerAnthropicVersion = "anthropic-version"
	headerAnthropicBeta    = "anthropic-beta"
	headerClientFeature    = "anthropic-client-feature"
	headerClientPlatform   = "anthropic-client-platform"
	headerClientVersion    = "anthropic-client-version"
	headerOrgUUID          = "x-organization-uuid"
	headerContentType      = "Content-Type"
)

// Client communicates with the Claude Code remote session API.
type Client struct {
	baseURL string
	orgUUID string
	token   string
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the default base URL (https://claude.ai).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// NewClient creates a Client with the given OAuth token and organization UUID.
func NewClient(token, orgUUID string, opts ...Option) *Client {
	jar, _ := cookiejar.New(nil)
	c := &Client{
		baseURL: defaultBaseURL,
		token:   token,
		orgUUID: orgUUID,
		http:    &http.Client{Jar: jar, Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewClientFromCLI creates a Client by reading credentials from ~/.claude/.
// The orgUUID must still be provided since it is not stored locally.
func NewClientFromCLI(orgUUID string, opts ...Option) (*Client, error) {
	if orgUUID == "" {
		return nil, ErrNoOrganization
	}
	token, err := ReadCLICredentials()
	if err != nil {
		return nil, err
	}
	return NewClient(token, orgUUID, opts...), nil
}

// do executes an HTTP request with all required Claude API headers.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set(headerAuthorization, "Bearer "+c.token)
	req.Header.Set(headerAnthropicVersion, "2023-06-01")
	req.Header.Set(headerAnthropicBeta, "ccr-byoc-2025-07-29")
	req.Header.Set(headerClientFeature, "ccr")
	req.Header.Set(headerClientPlatform, "web_claude_ai")
	req.Header.Set(headerClientVersion, "1.0.0")
	req.Header.Set(headerOrgUUID, c.orgUUID)
	req.Header.Set(headerContentType, "application/json")
	req.Header.Set("User-Agent", "claude-code-sdk-go/1.0.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	return c.http.Do(req)
}

// doJSON executes an HTTP request and decodes the JSON response into out.
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
