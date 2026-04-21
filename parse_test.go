package agentic

import (
	"testing"
)

func TestParseToolCalls_Simple(t *testing.T) {
	input := `I'll search for that. <call tool="search_web">{"q":"byzantine"}</call>`
	calls, _, err := parseToolCalls(input)
	if err != nil {
		t.Fatalf("parseToolCalls: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].Name != "search_web" {
		t.Errorf("name = %q, want %q", calls[0].Name, "search_web")
	}
	if string(calls[0].Args) != `{"q":"byzantine"}` {
		t.Errorf("args = %q, want %q", string(calls[0].Args), `{"q":"byzantine"}`)
	}
}

func TestParseToolCalls_Multiple(t *testing.T) {
	input := `<call tool="a">{"x":1}</call> and <call tool="b">{"y":2}</call>`
	calls, _, _ := parseToolCalls(input)
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0].Name != "a" || calls[1].Name != "b" {
		t.Errorf("names = %v", []string{calls[0].Name, calls[1].Name})
	}
}

func TestParseToolCalls_MalformedJSON(t *testing.T) {
	input := `<call tool="search_web">{not valid json</call>`
	_, _, err := parseToolCalls(input)
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestParseToolCalls_None(t *testing.T) {
	input := `Plain assistant reply, no tool calls here.`
	calls, remaining, err := parseToolCalls(input)
	if err != nil {
		t.Fatalf("parseToolCalls: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("got %d calls, want 0", len(calls))
	}
	if remaining != input {
		t.Errorf("remaining = %q, want original input", remaining)
	}
}

func TestParseToolCalls_RemainingText(t *testing.T) {
	input := `Before <call tool="a">{}</call> after.`
	_, remaining, _ := parseToolCalls(input)
	// remaining strips the tool calls; trimmed leftover is the assistant's prose
	if remaining != "Before  after." {
		t.Errorf("remaining = %q", remaining)
	}
}

func TestRenderFallbackToolPrompt(t *testing.T) {
	tools := []Tool{&fakeTool{name: "search_web"}}
	prompt := renderFallbackToolPrompt(tools)
	if !containsAll(prompt, "search_web", "<call tool=", "JSON") {
		t.Errorf("prompt missing key elements:\n%s", prompt)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
