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

// ListEnvironments returns all environments for the organization.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	path := fmt.Sprintf("/v1/environment_providers/private/organizations/%s/environments", c.orgUUID)

	var raw []rawEnvironment
	if err := c.doJSON(ctx, "GET", path, nil, &raw); err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	envs := make([]Environment, len(raw))
	for i, r := range raw {
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
