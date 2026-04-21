// Command researcher is an agentic demonstration that researches a topic
// using web search, URL fetch, Wikipedia, and arXiv.
//
// Usage:
//
//	BRAVE_API_KEY=... go run ./examples/researcher "byzantine iconoclasm"
//
// Requires Ollama at localhost:11434 with a tool-capable model (set
// AGENTIC_MODEL) and a Brave Search API key.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/tools/web"
	"github.com/CivNode/agentic/transport/ollama"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: researcher \"topic\"")
		os.Exit(2)
	}
	topic := os.Args[1]

	braveKey := os.Getenv("BRAVE_API_KEY")
	if braveKey == "" {
		fmt.Fprintln(os.Stderr, "BRAVE_API_KEY env required")
		os.Exit(2)
	}
	model := os.Getenv("AGENTIC_MODEL")
	if model == "" {
		model = "qwen3.5:27b"
	}

	agent := &agentic.Agent{
		Transport: ollama.New("http://localhost:11434"),
		Model:     model,
		Tools: []agentic.Tool{
			web.NewSearchWebBrave("", braveKey),
			web.NewFetchURL(),
			web.NewFetchPDF(),
			web.NewSearchWikipedia(""),
			web.NewSearchArxiv(""),
			web.NewExtractCitations(),
		},
		SystemPrompt: `You are a research assistant. Research the user's topic using the available tools. Start with Wikipedia for an overview, then search the web and arXiv for depth. Cite specific URLs in your final answer. Do not fabricate sources.`,
		OnEvent: func(e agentic.Event) {
			switch e.Type {
			case agentic.EventToolStart:
				fmt.Fprintf(os.Stderr, "[tool: %s %s]\n", e.ToolName, string(e.ToolArgs))
			case agentic.EventError:
				fmt.Fprintf(os.Stderr, "[error: %v]\n", e.Err)
			}
		},
	}
	result, err := agent.Run(context.Background(), "Research topic: "+topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result.FinalMessage)
	fmt.Fprintf(os.Stderr, "\n--- Stats ---\niterations: %d\ntokens: in=%d out=%d\n",
		result.Iterations, result.Usage.InputTokens, result.Usage.OutputTokens)
}
