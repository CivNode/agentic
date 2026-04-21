// Package agentic is a reliable, LLM-agnostic tool-use agent runtime.
//
// Build an agent by providing:
//   - An LLMTransport (Ollama, OpenAI-compat, Anthropic, or your own)
//   - A model name
//   - A set of Tools
//
// The agent loops, calling the LLM and invoking tools until the LLM stops
// asking for tool calls or MaxIterations is hit.
//
// See examples/ for runnable demonstrations.
package agentic

import (
	"context"
	"encoding/json"
)

// Tool is anything the agent can invoke. Implementations must be safe for
// concurrent use since the agent may invoke the same tool multiple times
// across iterations.
type Tool interface {
	// Name is the tool identifier passed to the LLM. Must match
	// ^[a-z][a-z0-9_]*$ for maximum LLM compatibility.
	Name() string

	// Description is a short sentence shown to the LLM so it knows when
	// to use the tool.
	Description() string

	// Schema is the JSON Schema for the tool's arguments. The agent
	// validates LLM-provided arguments against this before Invoke is
	// called.
	Schema() json.RawMessage

	// Invoke runs the tool with the given arguments and returns the
	// result as a string. The string is fed back to the LLM as a tool
	// message. Errors are also surfaced to the LLM.
	Invoke(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolCall is a single tool invocation requested by the LLM.
type ToolCall struct {
	ID   string          // opaque identifier; native transports provide this
	Name string          // tool name
	Args json.RawMessage // raw arguments — validate before Invoke
}

// RunResult is returned by Agent.Run at the end of a loop.
type RunResult struct {
	FinalMessage string     // the model's final reply after all tool calls
	Iterations   int        // number of LLM round-trips performed
	ToolCalls    []ToolCall // every tool call made during the run
	Usage        Usage      // aggregated token counts across the run
	Stopped      StopReason // why the loop ended
}

// StopReason explains why Agent.Run returned.
type StopReason string

const (
	StopReasonFinal         StopReason = "final"          // LLM produced final answer
	StopReasonMaxIterations StopReason = "max_iterations" // hit MaxIterations cap
	StopReasonCancelled     StopReason = "cancelled"      // context cancelled
	StopReasonError         StopReason = "error"          // fatal error
)

// Usage tracks LLM token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}
