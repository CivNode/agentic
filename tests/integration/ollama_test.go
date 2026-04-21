//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/transport/ollama"
)

type echoTool struct{}

func (*echoTool) Name() string        { return "echo" }
func (*echoTool) Description() string { return "Return the input string unchanged." }
func (*echoTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"s":{"type":"string"}},
		"required":["s"]
	}`)
}
func (*echoTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		S string `json:"s"`
	}
	_ = json.Unmarshal(args, &in)
	return in.S, nil
}

func TestOllamaIntegration_EchoToolCall(t *testing.T) {
	model := os.Getenv("AGENTIC_TEST_MODEL")
	if model == "" {
		model = "qwen3.5:8b"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	agent := &agentic.Agent{
		Transport:    ollama.New("http://localhost:11434"),
		Model:        model,
		Tools:        []agentic.Tool{&echoTool{}},
		SystemPrompt: "Use the echo tool when the user asks you to echo something. Reply with the exact string the tool returned.",
	}
	result, err := agent.Run(ctx, `Please echo the string "hello world" using the echo tool.`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.ToolCalls) == 0 {
		t.Fatal("expected at least one tool call")
	}
	if result.ToolCalls[0].Name != "echo" {
		t.Errorf("tool name = %q", result.ToolCalls[0].Name)
	}
}
