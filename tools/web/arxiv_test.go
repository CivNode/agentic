package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchArxiv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/1234.5678</id>
    <title>A Paper Title</title>
    <summary>Paper abstract here.</summary>
    <published>2024-01-15T00:00:00Z</published>
    <author><name>Alice Doe</name></author>
    <author><name>Bob Roe</name></author>
  </entry>
</feed>`))
	}))
	defer srv.Close()

	tool := NewSearchArxiv(srv.URL)
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"q":"byzantine"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(out, "A Paper Title") || !strings.Contains(out, "Alice Doe") {
		t.Errorf("missing fields:\n%s", out)
	}
}
