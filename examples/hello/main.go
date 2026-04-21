// Command hello is a minimal agentic demonstration. Given one tool
// (say_time), the agent answers "what time is it?" correctly.
//
// Usage:
//
//	go run ./examples/hello
//
// Requires Ollama running at localhost:11434 with a tool-capable model.
// Default model is qwen3.5:8b; override with AGENTIC_MODEL env.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/transport/ollama"
)

type sayTimeTool struct{}

func (*sayTimeTool) Name() string { return "say_time" }
func (*sayTimeTool) Description() string {
	return "Return the current server time as an RFC3339 string."
}
func (*sayTimeTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (*sayTimeTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

func main() {
	model := os.Getenv("AGENTIC_MODEL")
	if model == "" {
		model = "qwen3.5:8b"
	}
	agent := &agentic.Agent{
		Transport:    ollama.New("http://localhost:11434"),
		Model:        model,
		Tools:        []agentic.Tool{&sayTimeTool{}},
		SystemPrompt: "You are a terse assistant. Use the say_time tool to answer time questions.",
		OnEvent: func(e agentic.Event) {
			switch e.Type {
			case agentic.EventToolStart:
				fmt.Fprintf(os.Stderr, "[tool: %s]\n", e.ToolName)
			case agentic.EventFinal:
				// final message printed by main
			}
		},
	}
	result, err := agent.Run(context.Background(), "What time is it?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result.FinalMessage)
}
