package agentic

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeTool struct {
	name   string
	result string
}

func (t *fakeTool) Name() string            { return t.name }
func (t *fakeTool) Description() string     { return "test tool" }
func (t *fakeTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t *fakeTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	return t.result, nil
}

func TestToolInterface(t *testing.T) {
	var tool Tool = &fakeTool{name: "foo", result: "bar"}
	if tool.Name() != "foo" {
		t.Errorf("Name = %q, want %q", tool.Name(), "foo")
	}
	out, err := tool.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out != "bar" {
		t.Errorf("Invoke = %q, want %q", out, "bar")
	}
}
