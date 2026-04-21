package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchURL_HTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h1>Title</h1><p>Body paragraph.</p></body></html>"))
	}))
	defer srv.Close()

	tool := NewFetchURL()
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Body paragraph") {
		t.Errorf("output missing expected text:\n%s", out)
	}
}

func TestFetchURL_RejectsUnsupportedContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("fake png bytes"))
	}))
	defer srv.Close()

	tool := NewFetchURL()
	_, err := tool.Invoke(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if err == nil {
		t.Fatal("expected error on unsupported content type")
	}
}

func TestFetchURL_TruncatesOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// 11MB of bytes — over the 10MB cap.
		big := strings.Repeat("a", 11*1024*1024)
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()

	tool := NewFetchURL()
	_, err := tool.Invoke(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if err == nil {
		t.Fatal("expected error on oversized response")
	}
}

func TestFetchURL_StripsHTMLToText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><script>alert(1)</script><style>body{color:red}</style></head><body><nav>Nav</nav><main><h1>Real Title</h1><p>Prose.</p></main></body></html>`))
	}))
	defer srv.Close()

	tool := NewFetchURL()
	out, _ := tool.Invoke(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if strings.Contains(out, "alert(1)") {
		t.Errorf("script content leaked:\n%s", out)
	}
	if strings.Contains(out, "color:red") {
		t.Errorf("style content leaked:\n%s", out)
	}
	if !strings.Contains(out, "Real Title") {
		t.Errorf("missing actual content:\n%s", out)
	}
}
