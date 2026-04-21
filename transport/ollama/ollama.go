// Package ollama is an LLMTransport implementation for Ollama's /api/chat
// endpoint.
//
// Usage:
//
//	tr := ollama.New("http://localhost:11434")
//	agent := &agentic.Agent{Transport: tr, Model: "qwen3.5:27b", ...}
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CivNode/agentic"
)

// knownToolSupport lists Ollama models we know support native tool-calling.
// Used as a fast path before probing. Unknown models fall back to the
// parser — not ideal but safe.
var knownToolSupport = map[string]bool{
	// Qwen family — strong tool-call support.
	"qwen3.5": true,
	"qwen3":   true,
	"qwen2.5": true,
	// Llama 3.1+ — Meta trained for tool-use.
	"llama3.1": true,
	"llama3.2": true,
	"llama3.3": true,
	// Mistral family.
	"mistral-nemo":  true,
	"mistral-small": true,
	// Cohere.
	"command-r":      true,
	"command-r-plus": true,
	// Granite.
	"granite3": true,
}

// Transport implements agentic.LLMTransport against Ollama.
type Transport struct {
	BaseURL string
	Client  *http.Client
}

// New returns a Transport pointing at the given Ollama base URL (e.g.
// "http://localhost:11434").
func New(baseURL string) *Transport {
	return &Transport{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  http.DefaultClient,
	}
}

// SupportsTools reports whether the given model supports native tool-calling.
// Uses the knownToolSupport table; unknown models return false so the agent
// uses the fallback parser.
func (t *Transport) SupportsTools(ctx context.Context, model string) (bool, error) {
	base := strings.SplitN(model, ":", 2)[0]
	return knownToolSupport[base], nil
}

type ollamaChatRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Tools    []map[string]interface{} `json:"tools,omitempty"`
	Stream   bool                     `json:"stream"`
	Options  map[string]interface{}   `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Done            bool `json:"done"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
}

// Chat sends a single round-trip to /api/chat. Streaming is not enabled at
// the wire level (stream=false); the agent consumes tokens-at-a-time via
// OnEvent from RunResult events, not from the transport.
func (t *Transport) Chat(ctx context.Context, req agentic.ChatRequest) (agentic.ChatResponse, error) {
	body := ollamaChatRequest{
		Model:    req.Model,
		Messages: encodeMessages(req.Messages),
		Stream:   false,
	}
	if req.MaxTokens > 0 {
		body.Options = map[string]interface{}{"num_predict": req.MaxTokens}
	}
	if len(req.Tools) > 0 {
		body.Tools = encodeTools(req.Tools)
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.BaseURL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.Client.Do(httpReq)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("ollama: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		return agentic.ChatResponse{}, fmt.Errorf("ollama: status %d", httpResp.StatusCode)
	}

	var out ollamaChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}

	resp := agentic.ChatResponse{
		Message: agentic.Message{
			Role:    agentic.Role(out.Message.Role),
			Content: out.Message.Content,
		},
		Usage: agentic.Usage{
			InputTokens:  out.PromptEvalCount,
			OutputTokens: out.EvalCount,
		},
	}
	for i, tc := range out.Message.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		resp.ToolCalls = append(resp.ToolCalls, agentic.ToolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Name: tc.Function.Name,
			Args: argsJSON,
		})
	}
	return resp, nil
}

func encodeMessages(msgs []agentic.Message) []map[string]interface{} {
	out := make([]map[string]interface{}, len(msgs))
	for i, m := range msgs {
		entry := map[string]interface{}{
			"role":    string(m.Role),
			"content": m.Content,
		}
		if m.Role == agentic.RoleTool && m.Name != "" {
			entry["name"] = m.Name
		}
		// Preserve assistant tool calls on replay so the model has coherent
		// history. Without this, Ollama treats the assistant turn as if it
		// never called any tool and the subsequent tool-role message is
		// dangling; qwen/llama then apologise or re-call in a loop.
		if m.Role == agentic.RoleAssistant && len(m.ToolCalls) > 0 {
			tcs := make([]map[string]interface{}, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal(tc.Args, &args)
				tcs[j] = map[string]interface{}{
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": args,
					},
				}
			}
			entry["tool_calls"] = tcs
		}
		out[i] = entry
	}
	return out
}

func encodeTools(tools []agentic.Tool) []map[string]interface{} {
	specs := agentic.BuildToolSpecs(tools)
	out := make([]map[string]interface{}, len(specs))
	for i, s := range specs {
		var schema map[string]interface{}
		_ = json.Unmarshal(s.Schema, &schema)
		out[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        s.Name,
				"description": s.Description,
				"parameters":  schema,
			},
		}
	}
	return out
}
