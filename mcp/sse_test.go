package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallWithRequestStreamCumulative(t *testing.T) {
	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":5,\"total_tokens\":14}}\n\n" +
		"data: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"))
	var chunks []string
	got, err := c.CallWithRequestStream(&Request{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, func(text string) { chunks = append(chunks, text) })
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}
	if got != "Hello world" {
		t.Errorf("full text = %q", got)
	}
	wantChunks := []string{"Hel", "Hello", "Hello world"}
	if len(chunks) != len(wantChunks) {
		t.Fatalf("chunks = %v, want %v", chunks, wantChunks)
	}
	for i := range wantChunks {
		if chunks[i] != wantChunks[i] {
			t.Errorf("chunk[%d] = %q, want %q (cumulative)", i, chunks[i], wantChunks[i])
		}
	}
}

func TestCallWithRequestStreamReportsUsage(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":4,\"total_tokens\":14}}\n\n" +
		"data: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	}))
	defer srv.Close()

	prev := TokenUsageCallback
	defer func() { TokenUsageCallback = prev }()
	var got TokenUsage
	TokenUsageCallback = func(u TokenUsage) { got = u }

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"), WithProvider(ProviderDeepSeek))
	if _, err := c.CallWithRequestStream(&Request{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if got.TotalTokens != 14 {
		t.Errorf("usage = %+v", got)
	}
	if got.Provider != ProviderDeepSeek || got.Model != DefaultDeepSeekModel {
		t.Errorf("usage provider/model = %s/%s", got.Provider, got.Model)
	}
}

func TestCallWithRequestStreamHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"message":"slow down"}}`)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"), WithMaxRetries(0))
	_, err := c.CallWithRequestStream(&Request{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %v, want status 429 mention", err)
	}
}

func TestParseSSEStream(t *testing.T) {
	data := "data: {\"a\":1}\n\n" +
		"data: {\"b\":2}\n\n" +
		"data: [DONE]\n\n" +
		"data: {\"c\":3}\n\n" // after DONE must be ignored
	var events []string
	err := ParseSSEStream([]byte(data), func(e []byte) {
		events = append(events, string(e))
	})
	if err != nil {
		t.Fatalf("ParseSSEStream: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v, want 2 (stop at [DONE])", events)
	}
	if events[0] != `{"a":1}` || events[1] != `{"b":2}` {
		t.Errorf("events = %v", events)
	}
}

func TestParseSSEStreamNilCallback(t *testing.T) {
	if err := ParseSSEStream([]byte("data: x\n\n"), nil); err != nil {
		t.Errorf("nil callback should not error: %v", err)
	}
}

func TestReportStreamUsageNoCallback(t *testing.T) {
	prev := TokenUsageCallback
	defer func() { TokenUsageCallback = prev }()
	TokenUsageCallback = nil
	// must not panic
	ReportStreamUsage(&Client{}, TokenUsage{TotalTokens: 5})
}
