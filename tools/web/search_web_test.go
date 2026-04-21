package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchWeb_Brave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "testkey" {
			t.Errorf("missing X-Subscription-Token")
		}
		q := r.URL.Query().Get("q")
		if q != "byzantine iconoclasm" {
			t.Errorf("q = %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"web": map[string]interface{}{
				"results": []map[string]string{
					{"title": "A", "url": "https://a.example/", "description": "desc"},
					{"title": "B", "url": "https://b.example/", "description": "desc2"},
				},
			},
		})
	}))
	defer srv.Close()

	tool := NewSearchWebBrave(srv.URL, "testkey")
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"q":"byzantine iconoclasm"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !contains(out, "a.example") || !contains(out, "b.example") {
		t.Errorf("output missing URLs:\n%s", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
