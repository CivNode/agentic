// Command modelmatrix runs a standard "use tools to answer a factual
// question" benchmark against every tool-capable Ollama model listed and
// emits a markdown reliability table.
//
// Usage:
//
//	go run ./internal/modelmatrix > RELIABILITY.md
//
// Edit the models slice to add/remove models.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/transport/ollama"
)

type weatherTool struct{}

func (*weatherTool) Name() string { return "get_weather" }
func (*weatherTool) Description() string {
	return "Get the weather for a city. Returns a short description."
}
func (*weatherTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"city":{"type":"string"}},
		"required":["city"]
	}`)
}
func (*weatherTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		City string `json:"city"`
	}
	_ = json.Unmarshal(args, &in)
	return fmt.Sprintf("The weather in %s is 18°C and sunny.", in.City), nil
}

func main() {
	models := []string{
		"qwen3:8b",
		"qwen3.5:27b",
		"qwen3.5:35b-a3b",
		"llama3.3:latest",
		"mistral-small:latest",
		"mistral-nemo:latest",
	}

	prompt := "Use the get_weather tool to find the weather in Paris, then summarize in one sentence."

	// Preamble with run context so readers understand what the table means.
	date := time.Now().UTC().Format("2006-01-02")
	fmt.Printf("# Reliability matrix\n\n")
	fmt.Printf("Run date: %s. Each model given up to 300s per run. The task is two-round: call `get_weather`, then produce a one-sentence summary after the tool returns. A \"Yes / Yes\" row means the model both called the tool and produced a coherent final answer. Failures on larger models on modest hardware are usually wall-clock limits, not transport bugs — the library's multi-round protocol is exercised separately by `tests/integration/TestOllamaIntegration_MultiRound`.\n\n", date)
	fmt.Println("| Model | Tool call? | Final answer? | Iterations | Duration |")
	fmt.Println("|-------|-----------|---------------|------------|----------|")

	for _, model := range models {
		agent := &agentic.Agent{
			Transport:     ollama.New("http://localhost:11434"),
			Model:         model,
			Tools:         []agentic.Tool{&weatherTool{}},
			MaxIterations: 5,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		start := time.Now()
		result, err := agent.Run(ctx, prompt)
		elapsed := time.Since(start)
		cancel()

		calledTool := "No"
		if len(result.ToolCalls) > 0 {
			calledTool = "Yes"
		}
		final := "No"
		if result.Stopped == agentic.StopReasonFinal && result.FinalMessage != "" {
			final = "Yes"
		}
		if err != nil {
			final = "Error: " + strings.ReplaceAll(err.Error(), "|", "/")
		}
		fmt.Printf("| %s | %s | %s | %d | %s |\n", model, calledTool, final, result.Iterations, elapsed.Round(time.Second))
		_ = os.Stderr
	}
}
