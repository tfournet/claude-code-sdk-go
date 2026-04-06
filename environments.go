package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
)

// Environment represents a Claude Code execution environment.
type Environment struct {
	ID        string `json:"environment_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Online    bool   `json:"-"`
	Machine   string `json:"-"`
	Directory string `json:"-"`
}

// rawEnvironment captures the full JSON shape including nested bridge_info.
type rawEnvironment struct {
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	State         string          `json:"state"`
	BridgeInfo    json.RawMessage `json:"bridge_info"`
}

type bridgeInfo struct {
	Online      bool   `json:"online"`
	MachineName string `json:"machine_name"`
	Directory   string `json:"directory"`
}

// environmentsResponse wraps the JSON returned by the environments endpoint.
// The API returns {"environments": [...]}, not a bare array.
type environmentsResponse struct {
	Environments []rawEnvironment `json:"environments"`
}

// ListEnvironments returns all environments for the organization.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	// The CLI uses GET /v1/environment_providers with x-organization-uuid header
	// (set by the do method). The old path (/v1/environment_providers/private/
	// organizations/{orgId}/environments) returns 403 "Invalid authorization".
	const path = "/v1/environment_providers"

	var resp environmentsResponse
	if err := c.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	envs := make([]Environment, len(resp.Environments))
	for i, r := range resp.Environments {
		envs[i] = Environment{
			ID:    r.EnvironmentID,
			Name:  r.Name,
			Kind:  r.Kind,
			State: r.State,
		}
		if len(r.BridgeInfo) > 0 {
			var bi bridgeInfo
			if err := json.Unmarshal(r.BridgeInfo, &bi); err == nil {
				envs[i].Online = bi.Online
				envs[i].Machine = bi.MachineName
				envs[i].Directory = bi.Directory
			}
		}
	}
	return envs, nil
}
