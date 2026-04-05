package claudecode

import (
	"errors"
	"fmt"
)

// Sentinel errors for expected conditions.
var (
	ErrNoCredentials   = errors.New("claudecode: no CLI credentials found")
	ErrTokenExpired    = errors.New("claudecode: OAuth token has expired")
	ErrNoOrganization  = errors.New("claudecode: organization UUID required")
	ErrSessionNotFound = errors.New("claudecode: session not found")
	ErrNotConnected    = errors.New("claudecode: not connected")
)

// APIError represents an HTTP error response from the Claude API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("claudecode: API error %d: %s", e.StatusCode, e.Body)
}
