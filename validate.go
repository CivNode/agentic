package agentic

import (
	"encoding/json"
	"fmt"
)

// validateArgs does minimal validation of tool arguments against the tool's
// JSON Schema. It checks two things:
//   - args is a JSON object (when schema.type == "object")
//   - all required fields are present
//
// Full JSON Schema validation would need a third-party library. This is
// intentionally minimal — most LLM failures are "forgot the required field"
// or "sent a string instead of an object", and both are caught here.
func validateArgs(schemaRaw, argsRaw json.RawMessage) error {
	var schema struct {
		Type     string   `json:"type"`
		Required []string `json:"required"`
	}
	if len(schemaRaw) > 0 {
		if err := json.Unmarshal(schemaRaw, &schema); err != nil {
			// Malformed schema — skip validation, don't block tool calls.
			return nil
		}
	}
	if schema.Type == "object" || len(schema.Required) > 0 {
		var args map[string]interface{}
		if err := json.Unmarshal(argsRaw, &args); err != nil {
			return fmt.Errorf("args must be a JSON object: %w", err)
		}
		for _, field := range schema.Required {
			if _, ok := args[field]; !ok {
				return fmt.Errorf("missing required field %q", field)
			}
		}
	}
	return nil
}
