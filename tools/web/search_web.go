// Package web provides generic web tools for agentic agents: search, fetch,
// wikipedia, arxiv, and citation extraction. Each tool is optional — users
// import only what they need.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SearchWebBrave is a Tool that searches the web via the Brave Search API.
// Free tier: 2000 queries/month, no credit card required.
// Sign up at https://api.search.brave.com/
type SearchWebBrave struct {
	BaseURL string       // default "https://api.search.brave.com/res/v1"
	APIKey  string       // X-Subscription-Token header value
	Client  *http.Client // defaults to http.DefaultClient
	Count   int          // result count (default 10, max 20)
}

// NewSearchWebBrave constructs the tool. Pass "" for baseURL to use the
// default Brave API endpoint.
func NewSearchWebBrave(baseURL, apiKey string) *SearchWebBrave {
	if baseURL == "" {
		baseURL = "https://api.search.brave.com/res/v1"
	}
	return &SearchWebBrave{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Client:  http.DefaultClient,
		Count:   10,
	}
}

func (*SearchWebBrave) Name() string { return "search_web" }
func (*SearchWebBrave) Description() string {
	return "Search the web for recent information on a topic. Returns a ranked list of results with titles, URLs, and snippets."
}
func (*SearchWebBrave) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"q":{"type":"string","description":"search query"}},
		"required":["q"]
	}`)
}

type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func (s *SearchWebBrave) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Q string `json:"q"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if in.Q == "" {
		return "", fmt.Errorf("q is required")
	}
	count := s.Count
	if count <= 0 {
		count = 10
	}
	u := fmt.Sprintf("%s/web/search?q=%s&count=%d", s.BaseURL, url.QueryEscape(in.Q), count)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", s.APIKey)

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("brave: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("brave: status %d", resp.StatusCode)
	}
	var out braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(out.Web.Results) == 0 {
		return "No results found.", nil
	}
	var b strings.Builder
	for i, r := range out.Web.Results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}
	return b.String(), nil
}
