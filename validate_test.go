package agentic

import (
	"encoding/json"
	"testing"
)

func TestValidateArgs_RequiredFields(t *testing.T) {
	schema := json.RawMessage(`{
		"type":"object",
		"properties":{"q":{"type":"string"}},
		"required":["q"]
	}`)
	if err := validateArgs(schema, json.RawMessage(`{"q":"hi"}`)); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	if err := validateArgs(schema, json.RawMessage(`{}`)); err == nil {
		t.Error("missing required field accepted")
	}
}

func TestValidateArgs_NonObjectArgs(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	if err := validateArgs(schema, json.RawMessage(`"string not object"`)); err == nil {
		t.Error("non-object args accepted")
	}
}

func TestValidateArgs_EmptySchema(t *testing.T) {
	// No schema = anything goes.
	if err := validateArgs(json.RawMessage(`{}`), json.RawMessage(`{"anything":1}`)); err != nil {
		t.Errorf("empty schema should accept anything: %v", err)
	}
}
