package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DefaultMaxIterations is the default cap on LLM round-trips per Run.
const DefaultMaxIterations = 20

// Agent runs a tool-use loop over any LLMTransport.
//
// Usage:
//
//	agent := &Agent{
//	    Transport: ollama.New("http://localhost:11434"),
//	    Model:     "qwen3.5:27b",
//	    Tools:     []Tool{mySearchTool},
//	    SystemPrompt: "You are a research assistant.",
//	}
//	result, err := agent.Run(ctx, "What happened in the Byzantine iconoclasm?")
type Agent struct {
	Transport     LLMTransport
	Model         string
	Tools         []Tool
	SystemPrompt  string
	MaxIterations int         // defaults to DefaultMaxIterations if zero
	OnEvent       func(Event) // called with each streamed event; may be nil
}

// Event is emitted during a Run to allow streaming UIs to react in real time.
type Event struct {
	Type     EventType
	Content  string          // for EventToolResult, EventFinal
	ToolName string          // for EventToolStart, EventToolResult
	ToolArgs json.RawMessage // for EventToolStart
	Err      error           // for EventError
}

// EventType categorizes an Event.
type EventType string

const (
	EventToken      EventType = "token"       // incremental text from the LLM
	EventToolStart  EventType = "tool_start"  // agent began invoking a tool
	EventToolResult EventType = "tool_result" // tool returned
	EventFinal      EventType = "final"       // final answer produced
	EventError      EventType = "error"       // non-fatal error
)

// Run executes the agent loop. It blocks until the LLM produces a final
// answer, MaxIterations is hit, or ctx is cancelled.
func (a *Agent) Run(ctx context.Context, userPrompt string) (RunResult, error) {
	if a.Transport == nil {
		return RunResult{}, errors.New("agent: Transport is nil")
	}
	if a.Model == "" {
		return RunResult{}, errors.New("agent: Model is empty")
	}
	maxIter := a.MaxIterations
	if maxIter <= 0 {
		maxIter = DefaultMaxIterations
	}

	// Determine tool-use path.
	supportsNative := false
	if len(a.Tools) > 0 {
		ok, err := a.Transport.SupportsTools(ctx, a.Model)
		if err != nil {
			return RunResult{}, fmt.Errorf("probe SupportsTools: %w", err)
		}
		supportsNative = ok
	}

	// Build initial messages.
	messages := []Message{}
	systemPrompt := a.SystemPrompt
	if !supportsNative && len(a.Tools) > 0 {
		fallback := renderFallbackToolPrompt(a.Tools)
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + fallback
		} else {
			systemPrompt = fallback
		}
	}
	if systemPrompt != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: systemPrompt})
	}
	messages = append(messages, Message{Role: RoleUser, Content: userPrompt})

	result := RunResult{}
	toolIndex := make(map[string]Tool)
	for _, t := range a.Tools {
		toolIndex[t.Name()] = t
	}

	for iter := 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			result.Stopped = StopReasonCancelled
			return result, err
		}

		req := ChatRequest{
			Model:    a.Model,
			Messages: messages,
		}
		if supportsNative {
			req.Tools = a.Tools
		}
		resp, err := a.Transport.Chat(ctx, req)
		if err != nil {
			// Distinguish context cancellation from genuine errors so callers
			// can tell "user cancelled" from "model or network failed".
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				result.Stopped = StopReasonCancelled
			} else {
				result.Stopped = StopReasonError
			}
			return result, fmt.Errorf("iteration %d: chat: %w", iter, err)
		}
		result.Iterations++
		result.Usage.InputTokens += resp.Usage.InputTokens
		result.Usage.OutputTokens += resp.Usage.OutputTokens

		assistantMsg := resp.Message
		toolCalls := resp.ToolCalls

		// On fallback path, parse tool calls from body.
		if !supportsNative && len(toolCalls) == 0 && assistantMsg.Content != "" {
			parsed, remaining, perr := parseToolCalls(assistantMsg.Content)
			if perr != nil {
				// LLM produced malformed tags — feed error back and let it retry.
				errMsg := fmt.Sprintf("malformed tool call: %v — please emit valid JSON inside <call> tags", perr)
				messages = append(messages, assistantMsg)
				messages = append(messages, Message{Role: RoleUser, Content: errMsg})
				if a.OnEvent != nil {
					a.OnEvent(Event{Type: EventError, Err: perr})
				}
				continue
			}
			toolCalls = parsed
			assistantMsg.Content = strings.TrimSpace(remaining)
		}

		// Attach tool calls to the assistant message so the next iteration's
		// replay carries them. Transports require this to reconstruct the
		// provider-specific tool_calls / tool_use shape on the wire.
		assistantMsg.ToolCalls = toolCalls
		messages = append(messages, assistantMsg)

		if len(toolCalls) == 0 {
			// Final answer.
			result.FinalMessage = assistantMsg.Content
			result.Stopped = StopReasonFinal
			if a.OnEvent != nil {
				a.OnEvent(Event{Type: EventFinal, Content: assistantMsg.Content})
			}
			return result, nil
		}

		// Execute each tool call.
		for _, call := range toolCalls {
			result.ToolCalls = append(result.ToolCalls, call)

			tool, ok := toolIndex[call.Name]
			if !ok {
				errStr := fmt.Sprintf("unknown tool %q", call.Name)
				messages = append(messages, Message{
					Role:       RoleTool,
					Name:       call.Name,
					ToolCallID: call.ID,
					Content:    "error: " + errStr,
				})
				if a.OnEvent != nil {
					a.OnEvent(Event{Type: EventError, Err: errors.New(errStr), ToolName: call.Name})
				}
				continue
			}

			if err := validateArgs(tool.Schema(), call.Args); err != nil {
				errStr := fmt.Sprintf("invalid arguments: %v", err)
				messages = append(messages, Message{
					Role:       RoleTool,
					Name:       call.Name,
					ToolCallID: call.ID,
					Content:    "error: " + errStr,
				})
				if a.OnEvent != nil {
					a.OnEvent(Event{Type: EventError, Err: err, ToolName: call.Name})
				}
				continue
			}

			if a.OnEvent != nil {
				a.OnEvent(Event{Type: EventToolStart, ToolName: call.Name, ToolArgs: call.Args})
			}
			out, err := tool.Invoke(ctx, call.Args)
			if err != nil {
				out = "error: " + err.Error()
			}
			if a.OnEvent != nil {
				a.OnEvent(Event{Type: EventToolResult, ToolName: call.Name, Content: out})
			}
			messages = append(messages, Message{
				Role:       RoleTool,
				Name:       call.Name,
				ToolCallID: call.ID,
				Content:    out,
			})
		}
	}

	// Max iterations hit. Leave FinalMessage empty so callers can distinguish
	// this from a genuine final answer via result.Stopped.
	result.Stopped = StopReasonMaxIterations
	return result, nil
}
