package claudecode

import "context"

// AllowTool responds to a can_use_tool control request with "allow".
func (conn *Conn) AllowTool(ctx context.Context, requestID, toolUseID string) error {
	return conn.postEvents(ctx, []map[string]any{
		{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": requestID,
				"response": map[string]any{
					"behavior":     "allow",
					"updatedInput": map[string]any{},
					"toolUseID":    toolUseID,
				},
			},
		},
	})
}

// DenyTool responds to a can_use_tool control request with "deny".
func (conn *Conn) DenyTool(ctx context.Context, requestID, toolUseID string) error {
	return conn.postEvents(ctx, []map[string]any{
		{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": requestID,
				"response": map[string]any{
					"behavior":  "deny",
					"toolUseID": toolUseID,
				},
			},
		},
	})
}

// Interrupt sends an interrupt control request to stop the current generation.
func (conn *Conn) Interrupt(ctx context.Context) error {
	return conn.postEvents(ctx, []map[string]any{
		{
			"type":       "control_request",
			"request_id": generateUUID(),
			"request": map[string]string{
				"subtype": "interrupt",
			},
		},
	})
}

// SetPermissionMode changes the session's permission mode.
// Valid modes: "default", "acceptEdits", "bypassPermissions", "plan".
func (conn *Conn) SetPermissionMode(ctx context.Context, mode string) error {
	return conn.postEvents(ctx, []map[string]any{
		{
			"type":       "control_request",
			"request_id": generateUUID(),
			"request": map[string]string{
				"subtype":         "set_permission_mode",
				"permission_mode": mode,
			},
		},
	})
}

// SetModel changes the model used for the session.
func (conn *Conn) SetModel(ctx context.Context, model string) error {
	return conn.postEvents(ctx, []map[string]any{
		{
			"type":       "control_request",
			"request_id": generateUUID(),
			"request": map[string]string{
				"subtype": "set_model",
				"model":   model,
			},
		},
	})
}
