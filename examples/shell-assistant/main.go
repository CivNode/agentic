// Command shell-assistant is an agentic demonstration that exposes a
// run_command tool. DO NOT run untrusted prompts against this — the agent
// can execute arbitrary shell commands.
//
// Usage:
//
//	go run ./examples/shell-assistant "what files are in the current directory?"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/transport/ollama"
)

type runCommandTool struct{}

func (*runCommandTool) Name() string { return "run_command" }
func (*runCommandTool) Description() string {
	return "Run a shell command and return its stdout and stderr. Use for listing files, reading file contents, etc."
}
func (*runCommandTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"cmd":{"type":"string","description":"shell command to execute"}},
		"required":["cmd"]
	}`)
}
func (*runCommandTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Cmd string `json:"cmd"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", in.Cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("exit error: %v\n%s", err, string(out)), nil
	}
	return string(out), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: shell-assistant \"prompt\"")
		os.Exit(2)
	}
	model := os.Getenv("AGENTIC_MODEL")
	if model == "" {
		model = "qwen3.5:27b"
	}
	agent := &agentic.Agent{
		Transport:    ollama.New("http://localhost:11434"),
		Model:        model,
		Tools:        []agentic.Tool{&runCommandTool{}},
		SystemPrompt: "You are a shell assistant. Use run_command to inspect the user's system. Keep commands simple and read-only. Never run destructive commands.",
	}
	result, err := agent.Run(context.Background(), os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result.FinalMessage)
}
