package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBuildPayloadOpenAICompat(t *testing.T) {
	m := &Model{Provider: "deepseek", ModelName: "deepseek-chat", APIKey: "sk-test"}
	body, headers, err := buildPayload(m, ChatRequest{
		Messages:    []Message{{Role: "system", Content: "you are a trader"}},
		Temperature: 0.5,
		MaxTokens:   1024,
	})
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if headers["Authorization"] != "Bearer sk-test" {
		t.Errorf("auth header: %v", headers)
	}
	s := string(body)
	for _, want := range []string{`"model":"deepseek-chat"`, `"role":"system"`, `"max_tokens":1024`, `"temperature":0.5`} {
		if !strings.Contains(s, want) {
			t.Errorf("payload missing %s: %s", want, s)
		}
	}
}

func TestBuildPayloadClaude(t *testing.T) {
	m := &Model{Provider: "claude", ModelName: "claude-sonnet-4-5", APIKey: "sk-ant"}
	body, headers, err := buildPayload(m, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 100})
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if headers["x-api-key"] != "sk-ant" || headers["anthropic-version"] == "" {
		t.Errorf("claude headers: %v", headers)
	}
	if !strings.Contains(string(body), `"max_tokens":100`) {
		t.Errorf("claude payload missing max_tokens: %s", body)
	}
}

func TestParseContent(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		raw      string
		want     string
	}{
		{"openai", "gpt", `{"choices":[{"message":{"content":"hello"}}]}`, "hello"},
		{"claude", "claude", `{"content":[{"type":"text","text":"hello claude"}]}`, "hello claude"},
		{"gemini", "gemini", `{"candidates":[{"content":{"parts":[{"text":"hi gem"}]}}]}`, "hi gem"},
		{"openai-error", "gpt", `{"error":{"message":"rate limited"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseContent(tc.provider, []byte(tc.raw))
			if tc.want == "" {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseContent: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// roundTripFunc mock transport（测试用）。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockClient(handler roundTripFunc, timeout time.Duration) *Client {
	return NewWithTransport(handler, timeout)
}

func TestChatHappyPath(t *testing.T) {
	c := mockClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing auth header")
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"decision ok"}}]}`)),
		}, nil
	}, 5*time.Second)

	res, err := c.Chat(context.Background(), &Model{Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk-test"}, ChatRequest{})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if res.Content != "decision ok" {
		t.Errorf("content: %q", res.Content)
	}
	if res.LatencyMS < 0 {
		t.Errorf("latency negative")
	}
}

func TestChatTimeoutClassification(t *testing.T) {
	c := mockClient(func(r *http.Request) (*http.Response, error) {
		time.Sleep(2 * time.Second)
		return nil, context.DeadlineExceeded
	}, 100*time.Millisecond)

	_, err := c.Chat(context.Background(), &Model{Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk"}, ChatRequest{})
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestChatStatusError(t *testing.T) {
	c := mockClient(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key"}}`)),
		}, nil
	}, 5*time.Second)

	_, err := c.Chat(context.Background(), &Model{Provider: "gpt", ModelName: "gpt-4o", APIKey: "bad"}, ChatRequest{})
	if !errors.Is(err, ErrProvider) {
		t.Errorf("expected ErrProvider, got %v", err)
	}
}

func TestValidateBaseURL(t *testing.T) {
	if err := validateBaseURL("http://192.168.1.1/v1"); err == nil {
		t.Errorf("http scheme should be rejected")
	}
	if err := validateBaseURL("https://127.0.0.1/v1"); err == nil {
		t.Errorf("loopback should be rejected")
	}
	if err := validateBaseURL("https://10.0.0.5/v1"); err == nil {
		t.Errorf("private should be rejected")
	}
	if err := validateBaseURL("not-a-url"); err == nil {
		t.Errorf("malformed should be rejected")
	}
	// 合法 https 域名不应在 URL 校验层被拒（拨号层还有第二道防线）
	if err := validateBaseURL("https://api.openai.com/v1"); err != nil {
		t.Errorf("public https should pass: %v", err)
	}
}

func TestCustomProviderRequiresBaseURL(t *testing.T) {
	c := New(time.Second)
	_, err := c.Chat(context.Background(), &Model{Provider: "custom", ModelName: "m", APIKey: "k"}, ChatRequest{})
	if !errors.Is(err, ErrProvider) {
		t.Errorf("expected ErrProvider for missing base_url, got %v", err)
	}
}
