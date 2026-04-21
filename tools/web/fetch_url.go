package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// MaxFetchBytes is the hard cap on response bodies the fetcher will read.
const MaxFetchBytes = 10 * 1024 * 1024

var allowedContentTypes = map[string]bool{
	"text/html":             true,
	"text/plain":            true,
	"application/xhtml+xml": true,
	"application/json":      true,
}

// FetchURL is a Tool that fetches a URL, validates its Content-Type, and
// returns its text content (HTML stripped to plain text).
type FetchURL struct {
	Client *http.Client
}

// NewFetchURL constructs the tool with the default HTTP client.
func NewFetchURL() *FetchURL {
	return &FetchURL{Client: http.DefaultClient}
}

func (*FetchURL) Name() string { return "fetch_url" }
func (*FetchURL) Description() string {
	return "Fetch a URL and return its text content. Supports HTML, plain text, XHTML, and JSON. PDFs should use fetch_pdf instead. Rejects binary content types and responses over 10 MB."
}
func (*FetchURL) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"url":{"type":"string","description":"absolute URL to fetch"}},
		"required":["url"]
	}`)
}

func (f *FetchURL) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if in.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", in.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "agentic/0.1 (+https://github.com/CivNode/agentic)")

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]))
	if !allowedContentTypes[ct] {
		return "", fmt.Errorf("unsupported content type %q (use fetch_pdf for PDFs)", ct)
	}

	// Read with size cap; error if exceeded.
	limited := io.LimitReader(resp.Body, MaxFetchBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if len(buf) > MaxFetchBytes {
		return "", fmt.Errorf("response exceeds %d byte cap", MaxFetchBytes)
	}

	body := string(buf)
	if ct == "text/html" || ct == "application/xhtml+xml" {
		body = stripHTML(body)
	}
	return body, nil
}

var (
	scriptRE = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRE  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRE    = regexp.MustCompile(`<[^>]+>`)
	wsRE     = regexp.MustCompile(`[\s\n]+`)
)

// stripHTML removes scripts, styles, and tags, then collapses whitespace.
// Not a full HTML parser — good enough for feeding to an LLM.
func stripHTML(s string) string {
	s = scriptRE.ReplaceAllString(s, "")
	s = styleRE.ReplaceAllString(s, "")
	s = tagRE.ReplaceAllString(s, " ")
	s = wsRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
