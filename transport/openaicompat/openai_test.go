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
