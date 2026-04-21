package agentic

import (
	"context"
	"encoding/json"
)

// LLMTransport is how the agent talks to a model. Implement this to support
// any LLM backend — Ollama, OpenAI-compatible, Anthropic, or your own.
//
// The agent calls SupportsTools once per model and caches the result. If
// true, the agent uses the transport's native tool-calling mechanism. If
// false, the agent falls back to prompt-based tool-use with XML tags.
type LLMTransport interface {
	SupportsTools(ctx context.Context, model string) (bool, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// Message is a single turn in the conversation.
type Message struct {
	Role       Role       // system | user | assistant | tool
	Content    string     // the text of the message
	ToolCallID string     // populated when Role == RoleTool
	ToolCalls  []ToolCall // populated when Role == RoleAssistant and the LLM requested tools
	Name       string     // optional — some APIs use this for tool responses
}

// Role is the sender of a Message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ChatRequest is a single round-trip to the LLM.
type ChatRequest struct {
	Model     string
	Messages  []Message
	Tools     []Tool // ignored by transports that do not support tools
	MaxTokens int    // 0 means transport default (see each transport's docs)
}

// ChatResponse is what the LLM returned. On native tool-use, ToolCalls is
// populated and Message.Content may be empty. On fallback tool-use, ToolCalls
// is empty at this layer — the Agent parses them from Message.Content.
type ChatResponse struct {
	Message   Message
	ToolCalls []ToolCall
	Usage     Usage
}

// ToolSpec is a JSON-serializable view of a Tool used when building request
// bodies for native tool-calling.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"parameters"`
}

// BuildToolSpecs converts a slice of Tool into their wire representation.
func BuildToolSpecs(tools []Tool) []ToolSpec {
	out := make([]ToolSpec, len(tools))
	for i, t := range tools {
		out[i] = ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		}
	}
	return out
}
