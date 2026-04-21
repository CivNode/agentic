package agentic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// callTagRE matches <call tool="NAME">JSON_ARGS</call> blocks. The JSON must
// not contain the literal substring "</call>" which is a reasonable
// constraint for LLM output.
var callTagRE = regexp.MustCompile(`(?s)<call\s+tool="([a-z][a-z0-9_]*)">(.*?)</call>`)

// parseToolCalls extracts tool calls from an assistant message's body.
// Returns the parsed calls plus the remaining body text with call tags
// stripped. Returns an error if any tag contains malformed JSON.
func parseToolCalls(body string) ([]ToolCall, string, error) {
	matches := callTagRE.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil, body, nil
	}

	var calls []ToolCall
	var builder strings.Builder
	cursor := 0

	for _, m := range matches {
		// m[0], m[1] = full match start/end
		// m[2], m[3] = tool name start/end
		// m[4], m[5] = args start/end
		builder.WriteString(body[cursor:m[0]])

		name := body[m[2]:m[3]]
		argsRaw := strings.TrimSpace(body[m[4]:m[5]])

		var dummy interface{}
		if err := json.Unmarshal([]byte(argsRaw), &dummy); err != nil {
			return nil, "", fmt.Errorf("tool call %q: invalid JSON args: %w", name, err)
		}

		calls = append(calls, ToolCall{
			ID:   fmt.Sprintf("call_%d", len(calls)),
			Name: name,
			Args: json.RawMessage(argsRaw),
		})

		cursor = m[1]
	}
	builder.WriteString(body[cursor:])
	return calls, builder.String(), nil
}

// renderFallbackToolPrompt returns a system-prompt fragment describing the
// available tools for models that do not support native tool-calling.
// Injected into the agent's system prompt when the transport reports
// SupportsTools == false.
func renderFallbackToolPrompt(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("You have the following tools available. To invoke a tool, emit exactly one line:\n")
	b.WriteString(`  <call tool="TOOL_NAME">JSON_ARGUMENTS</call>` + "\n")
	b.WriteString("The JSON must match the schema shown for the tool. After each tool call, wait for the result before continuing. When you have enough information, give your final answer without any <call> tags.\n\n")
	b.WriteString("Tools:\n")
	for _, t := range tools {
		b.WriteString(fmt.Sprintf("  - %s — %s\n    schema: %s\n", t.Name(), t.Description(), string(t.Schema())))
	}
	return b.String()
}
