# agentic

A reliable, LLM-agnostic tool-use agent runtime in Go. Works with Ollama, any OpenAI-compatible API, and Anthropic. MIT licensed.

## Quickstart

Install and run the example researcher against your local Ollama:

```bash
go install github.com/CivNode/agentic/examples/hello@latest
hello   # answers "what time is it?" using a tool
```

Or build your own agent in ten lines:

```go
package main

import (
	"context"
	"fmt"

	"github.com/CivNode/agentic"
	"github.com/CivNode/agentic/tools/web"
	"github.com/CivNode/agentic/transport/ollama"
)

func main() {
	agent := &agentic.Agent{
		Transport:    ollama.New("http://localhost:11434"),
		Model:        "qwen3.5:27b",
		Tools:        []agentic.Tool{web.NewSearchWikipedia("")},
		SystemPrompt: "You are a research assistant.",
	}
	result, _ := agent.Run(context.Background(), "What was the Byzantine iconoclasm?")
	fmt.Println(result.FinalMessage)
}
```

## Features

- **LLM-agnostic.** Three built-in transports: Ollama, OpenAI-compatible (OpenAI, Z.ai, Groq, Mistral, Together, xAI), and Anthropic. Add your own by implementing one interface.
- **Two tool-call paths.** Native (llama3.1+, qwen2.5+, mistral-nemo/small, command-r, claude, gpt-4) or prompt-based fallback for models that were not trained for tool-use.
- **Generic web tools.** `tools/web` includes `search_web`, `fetch_url`, `fetch_pdf`, `search_wikipedia`, `search_arxiv`, `extract_citations`. Import only what you need.
- **Safe defaults.** 10 MB fetch cap, content-type gating, schema validation of tool arguments, iteration cap with context cancellation.
- **Small.** Core is under 400 lines. No HTTP framework, no deep dependencies.

## Interfaces

```go
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage
    Invoke(ctx context.Context, args json.RawMessage) (string, error)
}

type LLMTransport interface {
    SupportsTools(ctx context.Context, model string) (bool, error)
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type Agent struct {
    Transport     LLMTransport
    Model         string
    Tools         []Tool
    SystemPrompt  string
    MaxIterations int           // default 20
    OnEvent       func(Event)   // streaming callback
}

func (a *Agent) Run(ctx context.Context, userPrompt string) (RunResult, error)
```

## Model reliability

Measured by [`internal/modelmatrix`](internal/modelmatrix/). The matrix runs a one-tool-call task against each model and records whether the tool was called, whether a final answer was produced, and how long it took.

See [RELIABILITY.md](RELIABILITY.md) for the current table.

**Rule of thumb.** Any model on this list with "Yes / Yes" on a single Ollama machine handles CivNode-style research agents reliably. Smaller models (4B and under) generally do not.

## Examples

- [examples/hello](examples/hello/) — minimal agent with one tool.
- [examples/researcher](examples/researcher/) — full research agent with web search, URL fetch, Wikipedia, arXiv, citation extraction.
- [examples/shell-assistant](examples/shell-assistant/) — agent with side effects (runs shell commands). Demonstrates tool-execution pattern.

## Transports

### Ollama

```go
import "github.com/CivNode/agentic/transport/ollama"
tr := ollama.New("http://localhost:11434")
```

Probes a hard-coded list of known tool-capable Ollama models. Unknown models fall back to the parser — works but less reliable.

### OpenAI-compatible

```go
import "github.com/CivNode/agentic/transport/openaicompat"
tr := openaicompat.New("https://api.openai.com/v1", "sk-...")
```

Also works with Z.ai, Groq, Mistral, Together, xAI, and any provider that speaks the OpenAI chat-completions dialect.

### Anthropic

```go
import "github.com/CivNode/agentic/transport/anthropic"
tr := anthropic.New("https://api.anthropic.com", "sk-ant-...")
```

## Implementing your own tool

```go
type myTool struct{}

func (*myTool) Name() string        { return "my_tool" }
func (*myTool) Description() string { return "what it does" }
func (*myTool) Schema() json.RawMessage {
    return json.RawMessage(`{
        "type":"object",
        "properties":{"x":{"type":"string"}},
        "required":["x"]
    }`)
}
func (*myTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
    var in struct{ X string `json:"x"` }
    if err := json.Unmarshal(args, &in); err != nil {
        return "", err
    }
    return "result for " + in.X, nil
}
```

## Safety

- The `fetch_url` and `fetch_pdf` tools cap responses at 10 MB. Binary content types are rejected.
- The agent validates every tool call's JSON arguments against the tool's JSON Schema before invoking.
- `MaxIterations` bounds the loop. Context cancellation stops the agent between iterations.
- SSRF, rate limiting, and caching are **not** in the package — they belong in your calling code where you can tune them for your environment. See CivNode's `internal/researchagent` for a full production wrapping.

## Testing

```bash
go test ./...                              # unit tests
go test -tags=integration ./tests/...      # requires Ollama
```

## License

MIT. Use it however you want.

## Related

- `github.com/CivNode/agentic` was built for CivNode's research agent. See [civnode.com](https://civnode.com).
- MCP (Model Context Protocol) interop is on the Phase 2 roadmap. Track the issue tracker.
