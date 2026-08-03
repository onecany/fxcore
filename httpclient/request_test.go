package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewJSONRequestSetsContentTypeAndHeaders(t *testing.T) {
	req, err := NewJSONRequest(context.Background(), http.MethodPost, "https://example.com/test", map[string]string{"hello": "world"}, http.Header{
		"X-Test": []string{"value"},
	})
	if err != nil {
		t.Fatalf("NewJSONRequest() error = %v", err)
	}

	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := req.Header.Get("X-Test"); got != "value" {
		t.Fatalf("X-Test = %q, want value", got)
	}
}

func TestDoJSONSendsEncodedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("X-Test"); got != "value" {
			t.Fatalf("X-Test = %q, want value", got)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["hello"] != "world" {
			t.Fatalf("payload = %#v, want hello=world", payload)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	resp, err := DoJSON(context.Background(), server.Client(), http.MethodPost, server.URL, map[string]string{"hello": "world"}, http.Header{
		"X-Test": []string{"value"},
	})
	if err != nil {
		t.Fatalf("DoJSON() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}
