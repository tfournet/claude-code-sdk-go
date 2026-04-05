package claudecode

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCLICredentials(t *testing.T) {
	// Create a temp dir to simulate ~/.claude/
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	creds := cliCredentialsFile{
		ClaudeAIOAuth: &oauthCredentials{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			ExpiresAt:    time.Now().Add(1 * time.Hour).UnixMilli(),
		},
	}
	data, _ := json.Marshal(creds)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	token, err := ReadCLICredentials()
	if err != nil {
		t.Fatalf("ReadCLICredentials: %v", err)
	}
	if token != "test-access-token" {
		t.Errorf("token = %q, want %q", token, "test-access-token")
	}
}

func TestReadCLICredentials_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0700)

	creds := cliCredentialsFile{
		ClaudeAIOAuth: &oauthCredentials{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour).UnixMilli(),
		},
	}
	data, _ := json.Marshal(creds)
	os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0600)

	_, err := ReadCLICredentials()
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestReadCLICredentials_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := ReadCLICredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
