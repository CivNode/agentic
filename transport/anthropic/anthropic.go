// Package anthropic is an LLMTransport for the Anthropic Messages API.
//
// Usage:
//
//	tr := anthropic.New("https://api.anthropic.com", os.Getenv("ANTHROPIC_API_KEY"))
//	agent := &agentic.Agent{Transport: tr, Model: "claude-sonnet-4-6", ...}
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CivNode/agentic"
)

const apiVersion = "2023-06-01"

type Transport struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func New(baseURL, apiKey string) *Transport {
	return &Transport{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Client:  http.DefaultClient,
	}
}

func (t *Transport) SupportsTools(ctx context.Context, model string) (bool, error) {
	return true, nil
}

type anthReq struct {
	Model     string                   `json:"model"`
	MaxTokens int                      `json:"max_tokens"`
	System    string                   `json:"system,omitempty"`
	Messages  []map[string]interface{} `json:"messages"`
	Tools     []map[string]interface{} `json:"tools,omitempty"`
}

type anthResp struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (t *Transport) Chat(ctx context.Context, req agentic.ChatRequest) (agentic.ChatResponse, error) {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 4096
	}
	body := anthReq{
		Model:     req.Model,
		MaxTokens: maxTok,
	}
	// Extract system prompt(s); Anthropic uses a top-level field.
	var msgs []agentic.Message
	for _, m := range req.Messages {
		if m.Role == agentic.RoleSystem {
			if body.System != "" {
				body.System += "\n\n"
			}
			body.System += m.Content
		} else {
			msgs = append(msgs, m)
		}
	}
	body.Messages = encodeMessages(msgs)
	if len(req.Tools) > 0 {
		body.Tools = encodeTools(req.Tools)
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.BaseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", t.APIKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	httpResp, err := t.Client.Do(httpReq)
	if err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(httpResp.Body)
		return agentic.ChatResponse{}, fmt.Errorf("anthropic: status %d: %s", httpResp.StatusCode, errBody.String())
	}

	var out anthResp
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return agentic.ChatResponse{}, fmt.Errorf("decode: %w", err)
	}

	resp := agentic.ChatResponse{
		Message: agentic.Message{Role: agentic.RoleAssistant},
		Usage: agentic.Usage{
			InputTokens:  out.Usage.InputTokens,
			OutputTokens: out.Usage.OutputTokens,
		},
	}
	var textBuilder strings.Builder
	for _, block := range out.Content {
		switch block.Type {
		case "text":
			textBuilder.WriteString(block.Text)
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, agentic.ToolCall{
				ID:   block.ID,
				Name: block.Name,
				Args: block.Input,
			})
		}
	}
	resp.Message.Content = textBuilder.String()
	return resp, nil
}

func encodeMessages(msgs []agentic.Message) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == agentic.RoleTool {
			out = append(out, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{{
					"type":        "tool_result",
					"tool_use_id": m.ToolCallID,
					"content":     m.Content,
				}},
			})
			continue
		}
		out = append(out, map[string]interface{}{
			"role":    string(m.Role),
			"content": m.Content,
		})
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
			"name":         s.Name,
			"description":  s.Description,
			"input_schema": schema,
		}
	}
	return out
}
