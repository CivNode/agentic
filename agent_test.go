package agentic

import (
	"context"
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
