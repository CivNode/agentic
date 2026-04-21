package web

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SearchArxiv hits the arxiv.org export API.
// Default base URL: http://export.arxiv.org/api
type SearchArxiv struct {
	BaseURL string
	Client  *http.Client
}

func NewSearchArxiv(baseURL string) *SearchArxiv {
	if baseURL == "" {
		baseURL = "http://export.arxiv.org/api"
	}
	return &SearchArxiv{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  http.DefaultClient,
	}
}

func (*SearchArxiv) Name() string { return "search_arxiv" }
func (*SearchArxiv) Description() string {
	return "Search arXiv for academic papers. Returns titles, abstracts, authors, publication dates, and URLs."
}
func (*SearchArxiv) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"q":{"type":"string"}},
		"required":["q"]
	}`)
}

type arxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []arxivAuthor `xml:"author"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

func (s *SearchArxiv) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Q string `json:"q"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/query?search_query=all:%s&max_results=5", s.BaseURL, url.QueryEscape(in.Q))
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "agentic/0.1")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("arxiv: status %d", resp.StatusCode)
	}
	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return "", err
	}
	if len(feed.Entries) == 0 {
		return "No arXiv results.", nil
	}
	var b strings.Builder
	for i, e := range feed.Entries {
		authors := make([]string, len(e.Authors))
		for j, a := range e.Authors {
			authors[j] = a.Name
		}
		fmt.Fprintf(&b, "%d. %s\n   Authors: %s\n   Published: %s\n   URL: %s\n   Abstract: %s\n\n",
			i+1,
			strings.TrimSpace(e.Title),
			strings.Join(authors, ", "),
			e.Published[:10],
			e.ID,
			strings.TrimSpace(e.Summary),
		)
	}
	return b.String(), nil
}
