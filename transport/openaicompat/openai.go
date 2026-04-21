// Package openaicompat is an LLMTransport for the OpenAI chat completions
// API and any compatible endpoint (Z.ai, Groq, Mistral, Together, xAI, etc).
//
// Usage:
//
//	tr := openaicompat.New("https://api.openai.com/v1", os.Getenv("OPENAI_API_KEY"))
//	agent := &agentic.Agent{Transport: tr, Model: "gpt-4o", ...}
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CivNode/agentic"
)

// Transport implements agentic.LLMTransport against OpenAI-compatible APIs.
type Transport struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// New returns a Transport for the given base URL (without trailing slash)
// and API key.
func New(baseURL, apiKey string) *Transport {
	return &Transport{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Client:  http.DefaultClient,
	}
}

// SupportsTools always returns true. The OpenAI function-calling API is
// universally supported by providers in this dialect; if a specific model
// does not support it, the agent will get an API error from Chat and
// surface it normally.
func (t *Transport) SupportsTools(ctx context.Context, model string) (bool, error) {
	return true, nil
}

type oaiRequest struct {
	Model     string                   `json:"model"`
	Messages  []map[string]interface{} `json:"messages"`
	Tools     []map[string]interface{} `json:"tools,omitempty"`
	MaxTokens int                      `json:"max_tokens,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Role      string          `json:"role"`
			Content   json.RawMessage `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (t *Transport) Chat(ctx context.Context, req agentic.ChatRequest) (agentic.ChatResponse, error) {
	body := oaiRequest{
		Model:     req.Model,
		Messages:  encodeMessages(req.Messages),
		MaxTokens: req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		body.Tools = encodeTools(req.Tools)
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if t.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+t.APIKey)
	}
	httpResp, err := t.Client.Do(httpReq)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("openai: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(httpResp.Body)
		return agentic.ChatResponse{}, fmt.Errorf("openai: status %d: %s", httpResp.StatusCode, errBody.String())
	}
	var out oaiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return agentic.ChatResponse{}, fmt.Errorf("openai: empty choices")
	}
	choice := out.Choices[0]
	var content string
	if len(choice.Message.Content) > 0 && string(choice.Message.Content) != "null" {
		_ = json.Unmarshal(choice.Message.Content, &content)
	}
	resp := agentic.ChatResponse{
		Message: agentic.Message{
			Role:    agentic.Role(choice.Message.Role),
			Content: content,
		},
		Usage: agentic.Usage{
			InputTokens:  out.Usage.PromptTokens,
			OutputTokens: out.Usage.CompletionTokens,
		},
	}
	for _, tc := range choice.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, agentic.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(tc.Function.Arguments),
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
		if m.Role == agentic.RoleTool {
			entry["tool_call_id"] = m.ToolCallID
			entry["name"] = m.Name
		}
		// Preserve assistant tool calls on replay. OpenAI rejects tool-role
		// messages whose tool_call_id refers to a tool_call the assistant
		// turn no longer declares — without this we get 400s on iteration 2.
		if m.Role == agentic.RoleAssistant && len(m.ToolCalls) > 0 {
			tcs := make([]map[string]interface{}, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				tcs[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": string(tc.Args),
					},
				}
			}
			entry["tool_calls"] = tcs
			// When assistant produced only tool_calls, content should be
			// null rather than empty string per OpenAI's schema.
			if m.Content == "" {
				entry["content"] = nil
			}
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
