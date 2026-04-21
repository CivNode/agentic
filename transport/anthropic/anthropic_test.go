package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CivNode/agentic"
)

func TestSupportsTools_AlwaysTrue(t *testing.T) {
	tr := New("http://example.com", "key")
	ok, _ := tr.SupportsTools(context.Background(), "claude-sonnet-4-6")
	if !ok {
		t.Error("Anthropic should always report true")
	}
}

func TestChat_ToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "testkey" {
			t.Errorf("x-api-key header missing")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("anthropic-version missing")
		}
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "tu_1", "name": "lookup", "input": map[string]interface{}{"id": 42}},
			},
			"stop_reason": "tool_use",
			"usage":       map[string]int{"input_tokens": 12, "output_tokens": 4},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL, "testkey")
	resp, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []agentic.Message{{Role: agentic.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "lookup" {
		t.Errorf("ToolCalls = %+v", resp.ToolCalls)
	}
}
