package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SearchWikipedia hits the MediaWiki REST search endpoint.
// Default base URL: https://en.wikipedia.org/w/rest.php/v1
type SearchWikipedia struct {
	BaseURL string
	Client  *http.Client
}

func NewSearchWikipedia(baseURL string) *SearchWikipedia {
	if baseURL == "" {
		baseURL = "https://en.wikipedia.org/w/rest.php/v1"
	}
	return &SearchWikipedia{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  http.DefaultClient,
	}
}

func (*SearchWikipedia) Name() string { return "search_wikipedia" }
func (*SearchWikipedia) Description() string {
	return "Search Wikipedia for factual overviews of topics, people, places, and events. Returns titles, summaries, and page keys."
}
func (*SearchWikipedia) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"q":{"type":"string"}},
		"required":["q"]
	}`)
}

func (s *SearchWikipedia) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Q string `json:"q"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/search/page?q=%s&limit=5", s.BaseURL, url.QueryEscape(in.Q))
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "agentic/0.1")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("wikipedia: status %d", resp.StatusCode)
	}
	var out struct {
		Pages []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Excerpt     string `json:"excerpt"`
			Key         string `json:"key"`
		} `json:"pages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Pages) == 0 {
		return "No Wikipedia results.", nil
	}
	var b strings.Builder
	for i, p := range out.Pages {
		fmt.Fprintf(&b, "%d. %s — %s\n   https://en.wikipedia.org/wiki/%s\n   %s\n\n", i+1, p.Title, p.Description, p.Key, stripHTML(p.Excerpt))
	}
	return b.String(), nil
}
