package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchWikipedia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pages":[
			{"title":"Byzantine Iconoclasm","description":"iconoclastic period","excerpt":"The Byzantine Iconoclasm refers to...","key":"Byzantine_Iconoclasm"}
		]}`))
	}))
	defer srv.Close()

	tool := NewSearchWikipedia(srv.URL)
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"q":"byzantine"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(out, "Byzantine Iconoclasm") {
		t.Errorf("missing title:\n%s", out)
	}
}
