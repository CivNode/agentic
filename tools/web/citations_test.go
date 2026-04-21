package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractCitations_InlineReferences(t *testing.T) {
	input := `See Smith (2023) for background. Also [1] and Jones et al., 2020.`
	tool := NewExtractCitations()
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"text":`+mustJSON(input)+`}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(out, "Smith") || !strings.Contains(out, "2023") {
		t.Errorf("missing Smith citation:\n%s", out)
	}
	if !strings.Contains(out, "Jones") {
		t.Errorf("missing Jones citation:\n%s", out)
	}
}

func mustJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
