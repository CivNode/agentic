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

// TestChat_ReplaysAssistantToolUse verifies that assistant messages with
// ToolCalls round-trip as tool_use content blocks on replay. Anthropic 400s
// with "tool_result blocks must follow tool_use blocks" otherwise.
func TestChat_ReplaysAssistantToolUse(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		resp := map[string]interface{}{
			"content":     []map[string]interface{}{{"type": "text", "text": "done"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := New(srv.URL, "k")
	_, err := tr.Chat(context.Background(), agentic.ChatRequest{
		Model: "claude-sonnet-4-6",
		Messages: []agentic.Message{
			{Role: agentic.RoleUser, Content: "hi"},
			{
				Role:    agentic.RoleAssistant,
				Content: "Let me look that up.",
				ToolCalls: []agentic.ToolCall{
					{ID: "tu_1", Name: "lookup", Args: json.RawMessage(`{"id":42}`)},
				},
			},
			{Role: agentic.RoleTool, Name: "lookup", ToolCallID: "tu_1", Content: "result=42"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	msgs, _ := captured["messages"].([]interface{})
	assistant, _ := msgs[1].(map[string]interface{})
	blocks, ok := assistant["content"].([]interface{})
	if !ok || len(blocks) != 2 {
		t.Fatalf("assistant content should be 2 blocks (text + tool_use), got %+v", assistant["content"])
	}
	textBlock, _ := blocks[0].(map[string]interface{})
	if textBlock["type"] != "text" {
		t.Errorf("block[0].type = %v, want text", textBlock["type"])
	}
	toolBlock, _ := blocks[1].(map[string]interface{})
	if toolBlock["type"] != "tool_use" || toolBlock["id"] != "tu_1" || toolBlock["name"] != "lookup" {
		t.Errorf("block[1] = %+v", toolBlock)
	}
}
