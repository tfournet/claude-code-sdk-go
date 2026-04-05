package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cliCredentialsFile is the structure of ~/.claude/.credentials.json.
type cliCredentialsFile struct {
	ClaudeAIOAuth *oauthCredentials `json:"claudeAiOauth"`
}

type oauthCredentials struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"` // Unix milliseconds
}

// ReadCLICredentials reads the OAuth token from the Claude CLI credentials file.
// It returns the access token or an error if credentials are not found or expired.
func ReadCLICredentials() (token string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	path := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoCredentials, err)
	}

	var creds cliCredentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("parse credentials: %w", err)
	}

	if creds.ClaudeAIOAuth == nil || creds.ClaudeAIOAuth.AccessToken == "" {
		return "", ErrNoCredentials
	}

	if creds.ClaudeAIOAuth.ExpiresAt > 0 {
		expiresAt := time.UnixMilli(creds.ClaudeAIOAuth.ExpiresAt)
		if time.Now().After(expiresAt) {
			return "", ErrTokenExpired
		}
	}

	return creds.ClaudeAIOAuth.AccessToken, nil
}
