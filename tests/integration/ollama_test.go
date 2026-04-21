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

// TestOllamaIntegration_MultiRound exercises the two-round loop that the
// tool-replay fix was introduced for: the model must call the tool, receive
// the result, and produce a coherent final answer. Before the fix this case
// either looped forever or apologised; after the fix it should stop with
// StopReasonFinal and a non-empty FinalMessage that references the result.
func TestOllamaIntegration_MultiRound(t *testing.T) {
	model := os.Getenv("AGENTIC_TEST_MODEL")
	if model == "" {
		model = "qwen3:8b"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	agent := &agentic.Agent{
		Transport: ollama.New("http://localhost:11434"),
		Model:     model,
		Tools:     []agentic.Tool{&echoTool{}},
		SystemPrompt: "Use the echo tool exactly once to get the magic word. " +
			"When the tool returns, state the magic word in your final reply and stop.",
		MaxIterations: 8,
	}
	result, err := agent.Run(ctx, `Call echo with s="ORANGE-42", then tell me the magic word in a single short sentence.`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stopped != agentic.StopReasonFinal {
		t.Fatalf("Stopped = %q, want final (iterations=%d, final=%q)", result.Stopped, result.Iterations, result.FinalMessage)
	}
	if len(result.ToolCalls) == 0 {
		t.Fatal("expected at least one tool call")
	}
	if result.FinalMessage == "" {
		t.Error("FinalMessage is empty — model did not produce a coherent reply after tool result")
	}
}
