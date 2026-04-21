package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ledongthuc/pdf"
)

// FetchPDF downloads a PDF, extracts its text content, and returns it.
// Enforces the same 10 MB cap as FetchURL.
type FetchPDF struct {
	Client *http.Client
}

func NewFetchPDF() *FetchPDF { return &FetchPDF{Client: http.DefaultClient} }

func (*FetchPDF) Name() string { return "fetch_pdf" }
func (*FetchPDF) Description() string {
	return "Download a PDF and return its text content. Use for academic papers and PDF reports. Up to 10 MB."
}
func (*FetchPDF) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{"url":{"type":"string"}},
		"required":["url"]
	}`)
}

func (f *FetchPDF) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
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
	if ct != "application/pdf" {
		return "", fmt.Errorf("not a PDF (content-type %q)", ct)
	}
	limited := io.LimitReader(resp.Body, MaxFetchBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if len(buf) > MaxFetchBytes {
		return "", fmt.Errorf("response exceeds %d byte cap", MaxFetchBytes)
	}
	reader, err := pdf.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return "", fmt.Errorf("parse pdf: %w", err)
	}
	var text strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
		text.WriteString("\n\n")
	}
	return text.String(), nil
}
