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
