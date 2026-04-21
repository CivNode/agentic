package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CivNode/agentic"
)

func TestSupportsTools_AlwaysTrue(t *testing.T) {
	tr := New("http://example.com/v1", "key")
	ok, err := tr.SupportsTools(context.Background(), "gpt-4o")
	if err != nil {
		t.Fatalf("SupportsTools: %v", err)
	}
	if !ok {
		t.Error("OpenAI-compat should always report true")
	}
}

func TestChat_ToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer testkey" {
			t.Errorf("auth = %q", auth)
		}
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []map[string]interface{}{{
						"id":   "call_abc",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "lookup",
							"arguments": `{"id":42}`,
						},
					}},
				},
			}},
			"usage": map[string]int{"prompt_tokens": 8, "completion_tokens": 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL, "testkey")
	resp, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model:    "gpt-4o",
		Messages: []agentic.Message{{Role: agentic.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "lookup" {
		t.Errorf("ToolCalls = %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_abc" {
		t.Errorf("ID = %q", resp.ToolCalls[0].ID)
	}
}

// TestChat_ReplaysAssistantToolCalls verifies that assistant messages with
// ToolCalls round-trip correctly. OpenAI 400s on subsequent requests if a
// tool-role message references a tool_call_id that no prior assistant turn
// declared — so we must preserve tool_calls on replay.
func TestChat_ReplaysAssistantToolCalls(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message": map[string]interface{}{"role": "assistant", "content": "done"},
			}},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL, "k")
	_, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model: "gpt-4o",
		Messages: []agentic.Message{
			{Role: agentic.RoleUser, Content: "hi"},
			{
				Role:    agentic.RoleAssistant,
				Content: "",
				ToolCalls: []agentic.ToolCall{
					{ID: "call_abc", Name: "lookup", Args: json.RawMessage(`{"id":42}`)},
				},
			},
			{Role: agentic.RoleTool, Name: "lookup", ToolCallID: "call_abc", Content: "result=42"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	msgs, _ := captured["messages"].([]interface{})
	assistant, _ := msgs[1].(map[string]interface{})
	tcs, ok := assistant["tool_calls"].([]interface{})
	if !ok || len(tcs) != 1 {
		t.Fatalf("assistant replay missing tool_calls: %+v", assistant)
	}
	tc, _ := tcs[0].(map[string]interface{})
	if tc["id"] != "call_abc" {
		t.Errorf("replay tool id = %v, want call_abc", tc["id"])
	}
	if tc["type"] != "function" {
		t.Errorf("replay tool type = %v", tc["type"])
	}
	// Content should be nil (JSON null), not empty string, when only tool calls present.
	if assistant["content"] != nil {
		t.Errorf("assistant content should be null when only tool_calls; got %v", assistant["content"])
	}
}
