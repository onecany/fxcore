package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fxcore/mcp"
)

func TestAllProvidersRegistered(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		model   string
	}{
		{mcp.ProviderDeepSeek, mcp.DefaultDeepSeekBaseURL, mcp.DefaultDeepSeekModel},
		{mcp.ProviderOpenAI, mcp.DefaultOpenAIBaseURL, mcp.DefaultOpenAIModel},
		{mcp.ProviderClaude, mcp.DefaultClaudeBaseURL, mcp.DefaultClaudeModel},
		{mcp.ProviderQwen, mcp.DefaultQwenBaseURL, mcp.DefaultQwenModel},
		{mcp.ProviderGemini, mcp.DefaultGeminiBaseURL, mcp.DefaultGeminiModel},
		{mcp.ProviderGrok, mcp.DefaultGrokBaseURL, mcp.DefaultGrokModel},
		{mcp.ProviderKimi, mcp.DefaultKimiBaseURL, mcp.DefaultKimiModel},
		{mcp.ProviderMiniMax, mcp.DefaultMiniMaxBaseURL, mcp.DefaultMiniMaxModel},
	}
	for _, tc := range cases {
		c := mcp.NewAIClientByProvider(tc.name)
		if c == nil {
			t.Errorf("%s: not registered", tc.name)
			continue
		}
		cl := c.(mcp.ClientEmbedder).BaseClient()
		if cl.Provider != tc.name {
			t.Errorf("%s: Provider = %q", tc.name, cl.Provider)
		}
		if cl.BaseURL != tc.baseURL {
			t.Errorf("%s: BaseURL = %q, want %q", tc.name, cl.BaseURL, tc.baseURL)
		}
		if cl.Model != tc.model {
			t.Errorf("%s: Model = %q, want %q", tc.name, cl.Model, tc.model)
		}
	}
}

func TestProviderSetAPIKeyOverrides(t *testing.T) {
	c := mcp.NewAIClientByProvider(mcp.ProviderDeepSeek)
	if c == nil {
		t.Fatal("deepseek not registered")
	}
	c.SetAPIKey("sk-test-key-1234", "https://proxy.example.com/v1", "custom-v4")
	cl := c.(mcp.ClientEmbedder).BaseClient()
	if cl.APIKey != "sk-test-key-1234" {
		t.Errorf("APIKey = %q", cl.APIKey)
	}
	if cl.BaseURL != "https://proxy.example.com/v1" {
		t.Errorf("BaseURL = %q", cl.BaseURL)
	}
	if cl.Model != "custom-v4" {
		t.Errorf("Model = %q", cl.Model)
	}
	// Empty overrides keep defaults but the key still updates.
	c.SetAPIKey("only-key", "", "")
	if cl.BaseURL != "https://proxy.example.com/v1" || cl.Model != "custom-v4" {
		t.Error("empty overrides must not reset URL/model")
	}
}

func TestProviderOptionsForwarded(t *testing.T) {
	c := mcp.NewAIClientByProvider(mcp.ProviderDeepSeek, mcp.WithMaxTokens(321))
	cl := c.(mcp.ClientEmbedder).BaseClient()
	if cl.MaxTokens != 321 {
		t.Errorf("MaxTokens = %d, want 321", cl.MaxTokens)
	}
}

func TestKimiForcesTemperatureOne(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer srv.Close()

	c := mcp.NewAIClientByProvider(mcp.ProviderKimi, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("k"), mcp.WithTemperature(0.3))
	if _, err := c.CallWithMessages("s", "u"); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["temperature"] != 1.0 {
		t.Errorf("kimi temperature = %v, want 1.0 (K2.5 only accepts 1.0)", parsed["temperature"])
	}
}

func TestClaudeAuthAndEndpoint(t *testing.T) {
	var authKey, authVersion, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authKey = r.Header.Get("x-api-key")
		authVersion = r.Header.Get("anthropic-version")
		path = r.URL.Path
		w.WriteHeader(200)
		fmt.Fprint(w, `{"content":[{"type":"text","text":"hi"}]}`)
	}))
	defer srv.Close()

	c := mcp.NewAIClientByProvider(mcp.ProviderClaude, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("sk-ant-xyz"))
	if _, err := c.CallWithMessages("sys", "u"); err != nil {
		t.Fatal(err)
	}
	if authKey != "sk-ant-xyz" {
		t.Errorf("x-api-key = %q", authKey)
	}
	if authVersion != "2023-06-01" {
		t.Errorf("anthropic-version = %q", authVersion)
	}
	if path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", path)
	}
}

func TestClaudeWireFormatSystemAndTools(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(200)
		fmt.Fprint(w, `{"content":[{"type":"text","text":"ok"}]}`)
	}))
	defer srv.Close()

	c := mcp.NewAIClientByProvider(mcp.ProviderClaude, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("k"))
	req, _ := mcp.NewRequestBuilder().
		AddSystemMessage("system instructions").
		AddUserMessage("do it").
		AddTool(mcp.Tool{Type: "function", Function: mcp.FunctionDef{Name: "api_request", Description: "Call API", Parameters: map[string]any{"type": "object"}}}).
		WithToolChoice("auto").
		WithMaxTokens(1500).
		Build()
	if _, err := c.CallWithRequest(req); err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["system"] != "system instructions" {
		t.Errorf("system = %v (want top-level)", parsed["system"])
	}
	if parsed["max_tokens"] != float64(1500) {
		t.Errorf("max_tokens = %v", parsed["max_tokens"])
	}
	msgs := parsed["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1 (system role stripped)", len(msgs))
	}
	if msgs[0].(map[string]any)["role"] != "user" {
		t.Errorf("message role = %v", msgs[0].(map[string]any)["role"])
	}
	tools := parsed["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["name"] != "api_request" || tool["input_schema"] == nil {
		t.Errorf("tool = %v", tool)
	}
	tc := parsed["tool_choice"].(map[string]any)
	if tc["type"] != "auto" {
		t.Errorf("tool_choice = %v, want object with type=auto", tc)
	}
}

func TestClaudeToolUseAndToolResult(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(200)
		fmt.Fprint(w, `{"content":[{"type":"text","text":"done"}]}`)
	}))
	defer srv.Close()

	c := mcp.NewAIClientByProvider(mcp.ProviderClaude, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("k"))
	req, _ := mcp.NewRequestBuilder().
		AddUserMessage("run tools").
		AddMessage(mcp.Message{Role: "assistant", ToolCalls: []mcp.ToolCall{{ID: "tu_1", Type: "function", Function: mcp.ToolCallFunction{Name: "api_request", Arguments: `{"method":"GET"}`}}}}).
		AddMessage(mcp.Message{Role: "tool", ToolCallID: "tu_1", Content: `{"ok":true}`}).
		AddUserMessage("continue").
		Build()
	if _, err := c.CallWithRequest(req); err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	msgs := parsed["messages"].([]any)
	// [0] user, [1] assistant tool_use block, [2] user tool_result block, [3] user
	assistant := msgs[1].(map[string]any)
	blocks := assistant["content"].([]any)
	tu := blocks[0].(map[string]any)
	if tu["type"] != "tool_use" || tu["id"] != "tu_1" || tu["name"] != "api_request" {
		t.Errorf("tool_use block = %v", tu)
	}
	if tu["input"] == nil {
		t.Error("tool_use input must be an object")
	}
	if _, isObj := tu["input"].(map[string]any); !isObj {
		t.Errorf("tool_use input = %T, want object", tu["input"])
	}
	toolResult := msgs[2].(map[string]any)
	if toolResult["role"] != "user" {
		t.Errorf("tool result role = %v, want user", toolResult["role"])
	}
	tr := toolResult["content"].([]any)[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "tu_1" {
		t.Errorf("tool_result block = %v", tr)
	}
}

func TestClaudeResponseParsingTextAndToolUse(t *testing.T) {
	body := `{"content":[
		{"type":"text","text":"first"},
		{"type":"tool_use","id":"tu_9","name":"api_request","input":{"method":"POST","path":"/api/x"}}
	],"usage":{"input_tokens":50,"output_tokens":7}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	prev := mcp.TokenUsageCallback
	defer func() { mcp.TokenUsageCallback = prev }()
	var usage mcp.TokenUsage
	mcp.TokenUsageCallback = func(u mcp.TokenUsage) { usage = u }

	c := mcp.NewAIClientByProvider(mcp.ProviderClaude, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("k"))
	resp, err := c.CallWithRequestFull(&mcp.Request{Messages: []mcp.Message{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "first" {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "tu_9" || tc.Function.Name != "api_request" {
		t.Errorf("tool call = %+v", tc)
	}
	if !strings.Contains(tc.Function.Arguments, "POST") {
		t.Errorf("arguments = %q", tc.Function.Arguments)
	}
	if usage.TotalTokens != 57 {
		t.Errorf("usage total = %d, want 57", usage.TotalTokens)
	}
}

func TestClaudeResponseErrorBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`)
	}))
	defer srv.Close()

	c := mcp.NewAIClientByProvider(mcp.ProviderClaude, mcp.WithBaseURL(srv.URL), mcp.WithAPIKey("k"))
	_, err := c.CallWithMessages("s", "u")
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("err = %v, want error block message", err)
	}
}
