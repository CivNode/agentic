package web

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExtractCitations pulls structured citation mentions out of a block of
// text. Looks for:
//   - "Author (Year)" style: Smith (2023)
//   - "Author, Year" style: Jones, 2020
//   - "Author et al., Year" style
//   - "[N]" numbered references
//
// Returns them as a line-per-citation plain-text block.
type ExtractCitations struct{}

func NewExtractCitations() *ExtractCitations { return &ExtractCitations{} }

func (*ExtractCitations) Name() string { return "extract_citations" }
func (*ExtractCitations) Description() string {
	return "Extract citation mentions (author-year references) from a block of text. Use after fetching an article or paper to pull the bibliography."
}
func (*ExtractCitations) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"text":{"type":"string"}},
		"required":["text"]
	}`)
}

var (
	authorYearParen = regexp.MustCompile(`([A-Z][a-zA-Z\-]+(?:\s+(?:et\s+al\.?|and\s+[A-Z][a-zA-Z\-]+))?)\s*\((\d{4})\)`)
	authorYearComma = regexp.MustCompile(`([A-Z][a-zA-Z\-]+(?:\s+et\s+al\.?)?),\s*(\d{4})`)
	bracketNumRef   = regexp.MustCompile(`\[(\d+)\]`)
)

func (e *ExtractCitations) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	found := map[string]bool{}
	var cites []string

	for _, m := range authorYearParen.FindAllStringSubmatch(in.Text, -1) {
		key := m[1] + "|" + m[2]
		if !found[key] {
			found[key] = true
			cites = append(cites, fmt.Sprintf("- %s (%s)", strings.TrimSpace(m[1]), m[2]))
		}
	}
	for _, m := range authorYearComma.FindAllStringSubmatch(in.Text, -1) {
		key := m[1] + "|" + m[2]
		if !found[key] {
			found[key] = true
			cites = append(cites, fmt.Sprintf("- %s, %s", strings.TrimSpace(m[1]), m[2]))
		}
	}
	for _, m := range bracketNumRef.FindAllStringSubmatch(in.Text, -1) {
		key := "num|" + m[1]
		if !found[key] {
			found[key] = true
			cites = append(cites, fmt.Sprintf("- Reference [%s]", m[1]))
		}
	}
	if len(cites) == 0 {
		return "No citations found in the provided text.", nil
	}
	return strings.Join(cites, "\n"), nil
}
