package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mockUpstream records every request it receives so tests can assert on the
// wire format, and serves a canned JSON body.
type mockUpstream struct {
	t          *testing.T
	status     int
	body       string
	reqs       []*http.Request
	reqBodies  []string
	authHeader []string
}

func (m *mockUpstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.reqs = append(m.reqs, r)
		m.authHeader = append(m.authHeader, r.Header.Get("Authorization"))
		b, _ := io.ReadAll(r.Body)
		m.reqBodies = append(m.reqBodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(m.status)
		fmt.Fprint(w, m.body)
	}
}

func newMockClient(t *testing.T, baseURL string, opts ...ClientOption) AIClient {
	t.Helper()
	return NewClient(append(opts,
		WithBaseURL(baseURL),
		WithAPIKey("test-key"),
		WithRetryWaitBase(time.Millisecond),
	)...)
}

type mockUpstreamWithServer struct {
	*mockUpstream
	srv *httptest.Server
}

func newTestServer(t *testing.T, up *mockUpstream) *mockUpstreamWithServer {
	t.Helper()
	srv := httptest.NewServer(up.handler())
	t.Cleanup(srv.Close)
	return &mockUpstreamWithServer{mockUpstream: up, srv: srv}
}

func TestCallWithMessagesBasic(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"hello world"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	got, err := c.CallWithMessages("be brief", "hi")
	if err != nil {
		t.Fatalf("CallWithMessages failed: %v", err)
	}
	if got != "hello world" {
		t.Errorf("content = %q", got)
	}
	if len(up.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(up.reqs))
	}
	if up.reqs[0].URL.Path != "/chat/completions" {
		t.Errorf("path = %s, want /chat/completions", up.reqs[0].URL.Path)
	}
	if up.authHeader[0] != "Bearer test-key" {
		t.Errorf("auth = %q, want Bearer test-key", up.authHeader[0])
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(up.reqBodies[0]), &body); err != nil {
		t.Fatalf("request body not JSON: %v", err)
	}
	if body["model"] != DefaultDeepSeekModel {
		t.Errorf("model = %v, want %s", body["model"], DefaultDeepSeekModel)
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	first := msgs[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("message[0].role = %v, want system (hoisted to front)", first["role"])
	}
	if body["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5 default", body["temperature"])
	}
	if body["max_tokens"] != float64(2000) {
		t.Errorf("max_tokens = %v, want 2000", body["max_tokens"])
	}
}

func TestOpenAIUsesMaxCompletionTokens(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"ok"}}]}`}
	ts := newTestServer(t, up)
	c := NewClient(WithBaseURL(ts.srv.URL), WithAPIKey("k"), WithProvider(ProviderOpenAI))
	if _, err := c.CallWithMessages("s", "u"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(up.reqBodies[0], "max_completion_tokens") {
		t.Error("openai request should use max_completion_tokens")
	}
	if strings.Contains(up.reqBodies[0], `"max_tokens"`) {
		t.Error("openai request must not use max_tokens")
	}
}

func TestWireFormatToolMessages(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"done"}}]}`}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	req, _ := NewRequestBuilder().
		AddSystemMessage("sys").
		AddMessage(Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "api_request", Arguments: `{"a":1}`}}}}).
		AddMessage(Message{Role: "tool", ToolCallID: "call_1", Content: `{"ok":true}`}).
		AddMessage(Message{Role: "assistant", Content: "answer", ReasoningContent: "thinking..."}).
		AddUserMessage("next").
		AddTool(Tool{Type: "function", Function: FunctionDef{Name: "api_request", Parameters: map[string]any{"type": "object"}}}).
		WithToolChoice("auto").
		Build()
	if _, err := c.CallWithRequest(req); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(up.reqBodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	msgs := body["messages"].([]any)
	// assistant tool_calls message must NOT carry a content field
	assistant := msgs[1].(map[string]any)
	if _, hasContent := assistant["content"]; hasContent {
		t.Error("assistant tool_calls message must omit content")
	}
	if assistant["tool_calls"] == nil {
		t.Error("assistant message should carry tool_calls")
	}
	// tool result message carries tool_call_id
	tool := msgs[2].(map[string]any)
	if tool["tool_call_id"] != "call_1" {
		t.Errorf("tool message tool_call_id = %v", tool["tool_call_id"])
	}
	// reasoning_content is echoed back
	thinking := msgs[3].(map[string]any)
	if thinking["reasoning_content"] != "thinking..." {
		t.Errorf("reasoning_content = %v", thinking["reasoning_content"])
	}
	// system hoisted to front
	if msgs[0].(map[string]any)["role"] != "system" {
		t.Error("system message not hoisted to front")
	}
	if body["tools"] == nil {
		t.Error("tools missing from body")
	}
	if body["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v", body["tool_choice"])
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"message":"rate limit"}}`)
			return
		}
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"recovered"}}]}`)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"), WithRetryWaitBase(time.Millisecond))
	got, err := c.CallWithMessages("s", "u")
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if got != "recovered" {
		t.Errorf("content = %q", got)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (1 initial + 1 retry)", calls)
	}
}

func TestNonRetryableStatusNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(500) // not in the retryable list
		fmt.Fprint(w, `{"error":{"message":"boom"}}`)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"), WithRetryWaitBase(time.Millisecond))
	if _, err := c.CallWithMessages("s", "u"); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (500 not retryable)", calls)
	}
}

func TestRetryExhaustionStopsAfterMaxRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"unavailable"}}`)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("k"), WithMaxRetries(2), WithRetryWaitBase(time.Millisecond))
	if _, err := c.CallWithMessages("s", "u"); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (1 initial + 2 retries)", calls)
	}
}

func TestEmptyAPIKeyFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"x"}}]}`)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL)) // no API key
	_, err := c.CallWithMessages("s", "u")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("error = %q, want API key mention", err)
	}
}

func TestTokenUsageCallbackFires(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"c"}}],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	prev := TokenUsageCallback
	defer func() { TokenUsageCallback = prev }()
	var got TokenUsage
	TokenUsageCallback = func(u TokenUsage) { got = u }

	if _, err := c.CallWithMessages("s", "u"); err != nil {
		t.Fatal(err)
	}
	if got.TotalTokens != 14 || got.PromptTokens != 11 || got.CompletionTokens != 3 {
		t.Errorf("usage = %+v", got)
	}
	if got.Provider != ProviderCustom {
		t.Errorf("usage.Provider = %q, want %q", got.Provider, ProviderCustom)
	}
	if got.Model != DefaultDeepSeekModel {
		t.Errorf("usage.Model = %q", got.Model)
	}
	if got.Channel() != "native" {
		t.Errorf("Channel() = %q", got.Channel())
	}
}

func TestNoChoicesErrors(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[]}`}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)
	if _, err := c.CallWithMessages("s", "u"); err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestUpstreamErrorSurfaced(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"error":{"message":"model overloaded"}}`}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)
	if _, err := c.CallWithMessages("s", "u"); err == nil || !strings.Contains(err.Error(), "model overloaded") {
		t.Errorf("err = %v, want upstream error text", err)
	}
}

func TestCallWithRequestFullToolCalls(t *testing.T) {
	body := `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_9","type":"function","function":{"name":"api_request","arguments":"{\"method\":\"GET\",\"path\":\"/api/x\"}"}}]}}]}`
	up := &mockUpstream{status: 200, body: body}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	resp, err := c.CallWithRequestFull(&Request{Messages: []Message{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_9" || tc.Type != "function" || tc.Function.Name != "api_request" {
		t.Errorf("tool call = %+v", tc)
	}
	if !strings.Contains(tc.Function.Arguments, "GET") {
		t.Errorf("arguments = %q", tc.Function.Arguments)
	}
}

func TestCallWithRequestFullToolCallsObjectArguments(t *testing.T) {
	// Some providers send arguments as an inline object instead of a string.
	body := `{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":{"method":"POST"}}}]}}]}`
	up := &mockUpstream{status: 200, body: body}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	resp, err := c.CallWithRequestFull(&Request{Messages: []Message{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 || !strings.Contains(resp.ToolCalls[0].Function.Arguments, "POST") {
		t.Errorf("resp = %+v", resp)
	}
}

func TestToolArgumentsParseableByCaller(t *testing.T) {
	// OpenAI's standard wire form: arguments is a JSON-encoded STRING.
	// The parsed Arguments value must be the raw object text (no outer
	// quotes), so upper layers can json.Unmarshal it directly into a struct
	// (telegram/agent/agent.go does exactly this).
	body := `{"choices":[{"message":{"tool_calls":[{"id":"c9","type":"function","function":{"name":"api_request","arguments":"{\"method\":\"GET\",\"path\":\"/api/x\"}"}}]}}]}`
	up := &mockUpstream{status: 200, body: body}
	ts := newTestServer(t, up)
	c := newMockClient(t, ts.srv.URL)

	resp, err := c.CallWithRequestFull(&Request{Messages: []Message{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(resp.ToolCalls))
	}
	args := resp.ToolCalls[0].Function.Arguments
	var decoded map[string]any
	if err := json.Unmarshal([]byte(args), &decoded); err != nil {
		t.Fatalf("arguments %q must be directly unmarshalable by callers: %v", args, err)
	}
	if decoded["method"] != "GET" || decoded["path"] != "/api/x" {
		t.Errorf("decoded = %v", decoded)
	}
}

func TestSetAPIKeyOverridesAndFullURL(t *testing.T) {
	c := NewClient(WithAPIKey("orig"), WithProvider(ProviderDeepSeek))
	cl := c.(*Client)

	cl.SetAPIKey("new-key", "https://custom.example.com/v1", "custom-model")
	if cl.APIKey != "new-key" || cl.BaseURL != "https://custom.example.com/v1" || cl.Model != "custom-model" {
		t.Errorf("after SetAPIKey: %+v", cl)
	}
	if cl.UseFullURL {
		t.Error("UseFullURL should be false without # suffix")
	}

	// "#" suffix marks the URL as the complete endpoint.
	cl.SetAPIKey("k", "https://full.example.com/v1/chat/completions#", "")
	if !cl.UseFullURL {
		t.Error("UseFullURL should be true for # suffix")
	}
	if cl.BaseURL != "https://full.example.com/v1/chat/completions" {
		t.Errorf("BaseURL = %q, want URL without #", cl.BaseURL)
	}
	if cl.Model != "custom-model" {
		t.Error("empty customModel must not clear existing model")
	}

	// Empty overrides keep current values, only the key changes.
	cl.SetAPIKey("only-key", "", "")
	if cl.BaseURL != "https://full.example.com/v1/chat/completions" {
		t.Errorf("BaseURL changed unexpectedly: %q", cl.BaseURL)
	}
}

func TestUseFullURLAvoidsPathSuffix(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"x"}}]}`}
	ts := newTestServer(t, up)
	c := NewClient(WithBaseURL(ts.srv.URL+"/v1/chat/completions#"), WithAPIKey("k"))
	if _, err := c.CallWithMessages("s", "u"); err != nil {
		t.Fatal(err)
	}
	if up.reqs[0].URL.Path != "/v1/chat/completions" {
		t.Errorf("path = %s, want exact full URL path", up.reqs[0].URL.Path)
	}
}

func TestSetTimeoutUpdatesHTTPClient(t *testing.T) {
	c := NewClient().(*Client)
	c.SetTimeout(7 * time.Second)
	if c.HTTPClient.Timeout != 7*time.Second {
		t.Errorf("HTTPClient.Timeout = %v", c.HTTPClient.Timeout)
	}
	if c.Cfg.Timeout != 7*time.Second {
		t.Errorf("Cfg.Timeout = %v", c.Cfg.Timeout)
	}
}

func TestRequestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient(WithBaseURL("http://127.0.0.1:1"), WithAPIKey("k"), WithRetryWaitBase(time.Millisecond))
	_, err := c.CallWithRequest(&Request{Messages: []Message{{Role: "user", Content: "u"}}, Ctx: ctx})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
