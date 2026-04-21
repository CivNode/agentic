package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CivNode/agentic"
)

func TestSupportsTools_KnownModel(t *testing.T) {
	tr := New("http://localhost:11434")
	ok, _ := tr.SupportsTools(context.Background(), "qwen3.5:27b")
	if !ok {
		t.Error("qwen3.5:27b should support tools")
	}
	ok, _ = tr.SupportsTools(context.Background(), "gemma3:4b")
	if ok {
		t.Error("gemma3 should not support tools")
	}
}

func TestChat_NativeToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q", r.URL.Path)
		}
		resp := map[string]interface{}{
			"model": "qwen3.5:27b",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]interface{}{
					{
						"function": map[string]interface{}{
							"name":      "lookup",
							"arguments": map[string]interface{}{"id": 42},
						},
					},
				},
			},
			"done":              true,
			"prompt_eval_count": 10,
			"eval_count":        5,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL)
	resp, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model:    "qwen3.5:27b",
		Messages: []agentic.Message{{Role: agentic.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "lookup" {
		t.Errorf("ToolCalls[0].Name = %q", resp.ToolCalls[0].Name)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d", resp.Usage.InputTokens)
	}
}

// TestChat_ReplaysAssistantToolCalls verifies that when an assistant message
// with ToolCalls is sent back in a subsequent request, the transport encodes
// those ToolCalls in the provider's expected shape. Without this, the
// follow-up round has no record of the tool call the assistant made, and the
// model loses coherent history.
func TestChat_ReplaysAssistantToolCalls(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		resp := map[string]interface{}{
			"message": map[string]interface{}{"role": "assistant", "content": "ok"},
			"done":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL)
	_, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model: "qwen3.5:27b",
		Messages: []agentic.Message{
			{Role: agentic.RoleUser, Content: "look something up"},
			{
				Role:    agentic.RoleAssistant,
				Content: "",
				ToolCalls: []agentic.ToolCall{
					{ID: "call_0", Name: "lookup", Args: json.RawMessage(`{"id":42}`)},
				},
			},
			{Role: agentic.RoleTool, Name: "lookup", ToolCallID: "call_0", Content: "result=42"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	msgs, _ := captured["messages"].([]interface{})
	if len(msgs) != 3 {
		t.Fatalf("captured %d messages, want 3", len(msgs))
	}
	assistant, _ := msgs[1].(map[string]interface{})
	tcs, ok := assistant["tool_calls"].([]interface{})
	if !ok || len(tcs) != 1 {
		t.Fatalf("assistant replay missing tool_calls: %+v", assistant)
	}
	fn, _ := tcs[0].(map[string]interface{})["function"].(map[string]interface{})
	if fn["name"] != "lookup" {
		t.Errorf("replay tool name = %v", fn["name"])
	}
}
