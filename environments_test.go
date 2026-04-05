package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListEnvironments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/v1/environment_providers/private/organizations/org-abc/environments"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"environment_id": "env-1",
				"name":           "My Bridge",
				"kind":           "bridge",
				"state":          "active",
				"bridge_info": map[string]any{
					"online":       true,
					"machine_name": "devbox",
					"directory":    "/home/user/project",
				},
			},
			{
				"environment_id": "env-2",
				"name":           "Cloud",
				"kind":           "anthropic_cloud",
				"state":          "active",
			},
		})
	}))
	defer srv.Close()

	c := NewClient("tok", "org-abc", WithBaseURL(srv.URL))
	envs, err := c.ListEnvironments(context.Background())
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}

	// Bridge environment with bridge_info
	if envs[0].ID != "env-1" {
		t.Errorf("envs[0].ID = %q, want %q", envs[0].ID, "env-1")
	}
	if envs[0].Kind != "bridge" {
		t.Errorf("envs[0].Kind = %q, want %q", envs[0].Kind, "bridge")
	}
	if !envs[0].Online {
		t.Error("envs[0].Online = false, want true")
	}
	if envs[0].Machine != "devbox" {
		t.Errorf("envs[0].Machine = %q, want %q", envs[0].Machine, "devbox")
	}
	if envs[0].Directory != "/home/user/project" {
		t.Errorf("envs[0].Directory = %q, want %q", envs[0].Directory, "/home/user/project")
	}

	// Cloud environment without bridge_info
	if envs[1].ID != "env-2" {
		t.Errorf("envs[1].ID = %q, want %q", envs[1].ID, "env-2")
	}
	if envs[1].Online {
		t.Error("envs[1].Online = true, want false (no bridge_info)")
	}
}
