package agentic

import (
	"context"
	"testing"
)

type fakeTransport struct {
	supportsTools bool
	chatResult    ChatResponse
	chatErr       error
}

func (t *fakeTransport) SupportsTools(ctx context.Context, model string) (bool, error) {
	return t.supportsTools, nil
}
func (t *fakeTransport) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return t.chatResult, t.chatErr
}

func TestTransportInterface(t *testing.T) {
	var tr LLMTransport = &fakeTransport{supportsTools: true}
	ok, _ := tr.SupportsTools(context.Background(), "any-model")
	if !ok {
		t.Errorf("SupportsTools = false, want true")
	}
}

func TestMessageRoles(t *testing.T) {
	m := Message{Role: RoleUser, Content: "hi"}
	if m.Role != "user" {
		t.Errorf("RoleUser = %q, want %q", m.Role, "user")
	}
}
