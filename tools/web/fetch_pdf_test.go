package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPDF_RejectsNonPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html/>"))
	}))
	defer srv.Close()

	tool := NewFetchPDF()
	_, err := tool.Invoke(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if err == nil {
		t.Fatal("expected error on non-PDF content type")
	}
}
