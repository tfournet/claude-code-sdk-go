package claudecode

import (
	"encoding/json"
	"testing"
)

func TestEvent_UnmarshalAssistant(t *testing.T) {
	raw := `{
		"type": "assistant",
		"session_id": "sess-1",
		"uuid": "evt-1",
		"message": {
			"id": "msg-1",
			"role": "assistant",
			"model": "claude-sonnet-4-6",
			"content": [{"type": "text", "text": "Hello!"}],
			"stop_reason": null
		}
	}`

	var evt Event
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != "assistant" {
		t.Errorf("Type = %q, want %q", evt.Type, "assistant")
	}
	if evt.Message == nil {
		t.Fatal("Message is nil")
	}
	if evt.Message.ID != "msg-1" {
		t.Errorf("Message.ID = %q, want %q", evt.Message.ID, "msg-1")
	}
	if len(evt.Message.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(evt.Message.Content))
	}
	if evt.Message.Content[0].Text != "Hello!" {
		t.Errorf("Content[0].Text = %q, want %q", evt.Message.Content[0].Text, "Hello!")
	}
	if evt.Message.StopReason != nil {
		t.Errorf("StopReason = %v, want nil", *evt.Message.StopReason)
	}
}

func TestEvent_UnmarshalResult(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"session_id": "sess-1"
	}`

	var evt Event
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != "result" {
		t.Errorf("Type = %q, want %q", evt.Type, "result")
	}
	if evt.Subtype != "success" {
		t.Errorf("Subtype = %q, want %q", evt.Subtype, "success")
	}
	if evt.IsError {
		t.Error("IsError = true, want false")
	}
}

func TestEvent_UnmarshalToolUse(t *testing.T) {
	raw := `{
		"type": "assistant",
		"message": {
			"id": "msg-2",
			"role": "assistant",
			"content": [{
				"type": "tool_use",
				"tool_use": {
					"id": "tu-1",
					"name": "Read",
					"input": {"file_path": "/tmp/test.go"}
				}
			}],
			"stop_reason": null
		}
	}`

	var evt Event
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Message.Content[0].Type != "tool_use" {
		t.Errorf("Content[0].Type = %q, want %q", evt.Message.Content[0].Type, "tool_use")
	}
	tu := evt.Message.Content[0].ToolUse
	if tu == nil {
		t.Fatal("ToolUse is nil")
	}
	if tu.Name != "Read" {
		t.Errorf("ToolUse.Name = %q, want %q", tu.Name, "Read")
	}
	if tu.Input["file_path"] != "/tmp/test.go" {
		t.Errorf("ToolUse.Input[file_path] = %v, want /tmp/test.go", tu.Input["file_path"])
	}
}

func TestEvent_UnmarshalStopReason(t *testing.T) {
	raw := `{
		"type": "assistant",
		"message": {
			"id": "msg-3",
			"role": "assistant",
			"content": [{"type": "text", "text": "Done."}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 100, "output_tokens": 50}
		}
	}`

	var evt Event
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Message.StopReason == nil {
		t.Fatal("StopReason is nil, want end_turn")
	}
	if *evt.Message.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", *evt.Message.StopReason, "end_turn")
	}
	if evt.Message.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if evt.Message.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", evt.Message.Usage.InputTokens)
	}
}

func TestEvent_UnmarshalControlRequest(t *testing.T) {
	raw := `{
		"type": "control_request",
		"request_id": "req-1",
		"request": {"subtype": "can_use_tool", "tool_name": "Bash"}
	}`

	var evt Event
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != "control_request" {
		t.Errorf("Type = %q, want %q", evt.Type, "control_request")
	}
	if evt.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want %q", evt.RequestID, "req-1")
	}
	if evt.Request == nil {
		t.Fatal("Request is nil")
	}
}
