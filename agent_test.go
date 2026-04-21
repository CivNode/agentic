package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func TestNewAgent_Defaults(t *testing.T) {
	a := &Agent{
		Transport: &fakeTransport{},
		Model:     "test-model",
	}
	if a.MaxIterations == 0 {
		a.MaxIterations = DefaultMaxIterations
	}
	if a.MaxIterations != 20 {
		t.Errorf("DefaultMaxIterations = %d, want 20", a.MaxIterations)
	}
}

func TestAgent_RunStubReturnsError(t *testing.T) {
	// Stub test — before we implement Run it should at least compile.
	a := &Agent{
		Transport: &fakeTransport{chatResult: ChatResponse{Message: Message{Role: RoleAssistant, Content: "hi"}}},
		Model:     "test",
	}
	_, err := a.Run(context.Background(), "hello")
	if err != nil {
		// Not testing behavior yet — just that the method exists and doesn't panic.
		_ = err
	}
}

type scriptedTransport struct {
	mu            sync.Mutex
	supportsTools bool
	responses     []ChatResponse // consumed in order
	errors        []error        // parallel to responses; nil = no error
	callCount     int
}

func (t *scriptedTransport) SupportsTools(ctx context.Context, model string) (bool, error) {
	return t.supportsTools, nil
}
func (t *scriptedTransport) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	idx := t.callCount
	t.callCount++
	if idx >= len(t.responses) {
		return ChatResponse{}, fmt.Errorf("scriptedTransport: no response for call %d", idx)
	}
	var err error
	if idx < len(t.errors) {
		err = t.errors[idx]
	}
	return t.responses[idx], err
}

func TestAgent_NativePath_FinalAnswerFirstShot(t *testing.T) {
	tr := &scriptedTransport{
		supportsTools: true,
		responses: []ChatResponse{
			{Message: Message{Role: RoleAssistant, Content: "The answer is 42."}},
		},
	}
	a := &Agent{Transport: tr, Model: "m"}
	result, err := a.Run(context.Background(), "what?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stopped != StopReasonFinal {
		t.Errorf("Stopped = %q, want final", result.Stopped)
	}
	if result.FinalMessage != "The answer is 42." {
		t.Errorf("FinalMessage = %q", result.FinalMessage)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", result.Iterations)
	}
}

func TestAgent_NativePath_SingleToolCall(t *testing.T) {
	tool := &fakeTool{name: "lookup", result: "42"}
	tr := &scriptedTransport{
		supportsTools: true,
		responses: []ChatResponse{
			{
				Message:   Message{Role: RoleAssistant, Content: ""},
				ToolCalls: []ToolCall{{ID: "c1", Name: "lookup", Args: json.RawMessage(`{}`)}},
			},
			{Message: Message{Role: RoleAssistant, Content: "The answer is 42."}},
		},
	}
	a := &Agent{Transport: tr, Model: "m", Tools: []Tool{tool}}
	result, err := a.Run(context.Background(), "what?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stopped != StopReasonFinal {
		t.Errorf("Stopped = %q", result.Stopped)
	}
	if len(result.ToolCalls) != 1 {
		t.Errorf("ToolCalls = %d, want 1", len(result.ToolCalls))
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", result.Iterations)
	}
}

func TestAgent_FallbackPath_ParsesToolCallsFromBody(t *testing.T) {
	tool := &fakeTool{name: "lookup", result: "42"}
	tr := &scriptedTransport{
		supportsTools: false,
		responses: []ChatResponse{
			{Message: Message{Role: RoleAssistant, Content: `<call tool="lookup">{}</call>`}},
			{Message: Message{Role: RoleAssistant, Content: "The answer is 42."}},
		},
	}
	a := &Agent{Transport: tr, Model: "m", Tools: []Tool{tool}}
	result, err := a.Run(context.Background(), "what?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "lookup" {
		t.Errorf("ToolCalls = %v", result.ToolCalls)
	}
}

func TestAgent_MaxIterations(t *testing.T) {
	// Transport keeps requesting the tool, never gives final answer.
	tool := &fakeTool{name: "loop", result: "keep going"}
	responses := make([]ChatResponse, 30)
	for i := range responses {
		responses[i] = ChatResponse{
			Message:   Message{Role: RoleAssistant},
			ToolCalls: []ToolCall{{ID: fmt.Sprintf("c%d", i), Name: "loop", Args: json.RawMessage(`{}`)}},
		}
	}
	tr := &scriptedTransport{supportsTools: true, responses: responses}
	a := &Agent{Transport: tr, Model: "m", Tools: []Tool{tool}, MaxIterations: 5}
	result, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stopped != StopReasonMaxIterations {
		t.Errorf("Stopped = %q, want max_iterations", result.Stopped)
	}
	if result.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5", result.Iterations)
	}
}

func TestAgent_ContextCancellation(t *testing.T) {
	tr := &scriptedTransport{
		supportsTools: true,
		responses: []ChatResponse{
			{Message: Message{Role: RoleAssistant, Content: "ok"}},
		},
	}
	a := &Agent{Transport: tr, Model: "m"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled
	_, err := a.Run(ctx, "hi")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestAgent_UnknownToolCall(t *testing.T) {
	tr := &scriptedTransport{
		supportsTools: true,
		responses: []ChatResponse{
			{
				Message:   Message{Role: RoleAssistant},
				ToolCalls: []ToolCall{{ID: "c1", Name: "nonexistent", Args: json.RawMessage(`{}`)}},
			},
			{Message: Message{Role: RoleAssistant, Content: "sorry"}},
		},
	}
	a := &Agent{Transport: tr, Model: "m"}
	result, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations = %d", result.Iterations)
	}
}

func TestAgent_EventCallbacks(t *testing.T) {
	tool := &fakeTool{name: "lookup", result: "42"}
	tr := &scriptedTransport{
		supportsTools: true,
		responses: []ChatResponse{
			{
				Message:   Message{Role: RoleAssistant},
				ToolCalls: []ToolCall{{ID: "c1", Name: "lookup", Args: json.RawMessage(`{}`)}},
			},
			{Message: Message{Role: RoleAssistant, Content: "done"}},
		},
	}
	var events []Event
	a := &Agent{
		Transport: tr, Model: "m", Tools: []Tool{tool},
		OnEvent: func(e Event) { events = append(events, e) },
	}
	_, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Expect: ToolStart, ToolResult, Final
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(events), events)
	}
	if events[0].Type != EventToolStart {
		t.Errorf("events[0] = %q", events[0].Type)
	}
	if events[1].Type != EventToolResult {
		t.Errorf("events[1] = %q", events[1].Type)
	}
	if events[2].Type != EventFinal {
		t.Errorf("events[2] = %q", events[2].Type)
	}
}
