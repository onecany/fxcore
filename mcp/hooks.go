package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"fxcore/httpclient"
	"strings"
)

// defaultHooks is the OpenAI-compatible wire format shared by every provider
// except Claude (which overrides it with the Anthropic Messages format).
type defaultHooks struct{}

// Call executes the HTTP request through the client's configured transport.
func (defaultHooks) Call(client *Client, req *httpclient.Request) (*httpclient.Response, error) {
	return client.HTTPClient.Do(req)
}

// BuildUrl returns the chat completions endpoint. UseFullURL clients use the
// configured BaseURL verbatim (SetAPIKey URLs ending in "#" set this flag).
func (defaultHooks) BuildUrl(client *Client, _ *Request) string {
	if client.UseFullURL {
		return client.BaseURL
	}
	return strings.TrimRight(client.BaseURL, "/") + "/chat/completions"
}

// SetAuthHeader applies the standard Bearer authentication.
func (defaultHooks) SetAuthHeader(client *Client, req *httpclient.Request) {
	if client.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+client.APIKey)
	}
}

// MarshalRequestBody is the default JSON encoder.
func (defaultHooks) MarshalRequestBody(_ *Client, payload any) ([]byte, error) {
	return json.Marshal(payload)
}

// BuildRequestBodyFromRequest builds a non-streaming request body.
func (h defaultHooks) BuildRequestBodyFromRequest(client *Client, req *Request) ([]byte, error) {
	return buildOpenAIRequestBody(client, req, false)
}

// BuildMCPRequestBody builds a streaming request body (stream=true).
func (h defaultHooks) BuildMCPRequestBody(client *Client, req *Request) ([]byte, error) {
	return buildOpenAIRequestBody(client, req, true)
}

// BuildRequest assembles the full HTTP request: URL, body, auth headers.
func (h defaultHooks) BuildRequest(client *Client, req *Request) (*httpclient.Request, error) {
	body, err := client.Hooks.BuildRequestBodyFromRequest(client, req)
	if err != nil {
		return nil, fmt.Errorf("mcp: build request body: %w", err)
	}
	ctx := req.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := httpclient.NewRequest(ctx, httpclient.MethodPost, client.Hooks.BuildUrl(client, req), bytes.NewReader(body), nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client.Hooks.SetAuthHeader(client, httpReq)
	return httpReq, nil
}

// ParseMCPResponse extracts the plain-text content from a response body.
func (h defaultHooks) ParseMCPResponse(client *Client, body []byte) (string, error) {
	resp, err := h.ParseMCPResponseFull(client, body)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ParseMCPResponseFull parses choices[0] plus usage reporting (spec 2.5.5).
func (defaultHooks) ParseMCPResponseFull(client *Client, body []byte) (*LLMResponse, error) {
	var parsed openAIWireResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("mcp: parse response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("mcp: upstream error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("mcp: no choices in response")
	}

	msg := parsed.Choices[0].Message
	resp := &LLMResponse{
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
	}
	for _, tc := range msg.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: parseToolArguments(tc.Function.Arguments),
			},
		})
	}

	if parsed.Usage != nil && parsed.Usage.TotalTokens > 0 {
		ReportStreamUsage(client, TokenUsage{
			Provider:         client.Provider,
			Model:            client.Model,
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		})
	}
	return resp, nil
}

// IsRetryableError matches error text against the configured retryable
// patterns (spec 2.5.2). HTTP status errors carry "status NNN" in their
// text, so 429/502/503/520/524 match while other statuses do not.
func (defaultHooks) IsRetryableError(client *Client, err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, pattern := range client.Cfg.RetryableErrors {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// defaultRetryablePatterns returns the retryable error substrings (spec 2.5.2).
func defaultRetryablePatterns() []string {
	return []string{
		"EOF",
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
		"stream error",
		"INTERNAL_ERROR",
		"status 429",
		"rate_limit_error",
		"upstream_empty_output",
		"status 502",
		"status 503",
		"status 520",
		"status 524",
	}
}

// parseToolArguments normalizes a tool-call arguments payload to raw JSON
// object text that callers can json.Unmarshal directly. OpenAI sends
// arguments as a JSON-encoded string ("{...}"); some providers send an
// inline object. Both become "{...}".
func parseToolArguments(raw json.RawMessage) string {
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	return string(raw)
}

// ── OpenAI-compatible request body ──────────────────────────────────────────

// openAIWireResponse mirrors the OpenAI chat completions JSON envelope.
type openAIWireResponse struct {
	Choices []struct {
		Message struct {
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			ToolCalls        []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// openAIToolCall keeps arguments as raw JSON: providers may send either a
// JSON string or an inline object.
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// buildOpenAIRequestBody constructs the messages payload (spec 2.5.3).
// System messages are hoisted to the front; optional parameters are written
// only when non-nil; assistant tool_calls omit content; tool results carry
// tool_call_id; reasoning_content is echoed back for thinking models.
func buildOpenAIRequestBody(client *Client, req *Request, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = client.Model
	}
	body := map[string]any{
		"model":       model,
		"messages":    buildWireMessages(req.Messages),
		"temperature": effectiveTemperature(client, req),
		"stream":      stream || req.Stream,
	}

	maxTokens := client.Cfg.MaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if maxTokens > 0 {
		if client.Provider == ProviderOpenAI {
			body["max_completion_tokens"] = maxTokens
		} else {
			body["max_tokens"] = maxTokens
		}
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.FrequencyPenalty != nil {
		body["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		body["presence_penalty"] = *req.PresencePenalty
	}
	if len(req.Stop) > 0 {
		body["stop"] = req.Stop
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
		if req.ToolChoice != "" {
			body["tool_choice"] = req.ToolChoice
		}
	}
	return json.Marshal(body)
}

// buildWireMessages converts messages to the OpenAI wire shape, hoisting
// system messages to the front while preserving the relative order of the
// remaining messages.
func buildWireMessages(msgs []Message) []map[string]any {
	var system, rest []Message
	for _, m := range msgs {
		if m.Role == "system" {
			system = append(system, m)
		} else {
			rest = append(rest, m)
		}
	}
	ordered := append(system, rest...)

	out := make([]map[string]any, 0, len(ordered))
	for _, m := range ordered {
		item := map[string]any{"role": m.Role}
		switch {
		case m.Role == "tool":
			item["tool_call_id"] = m.ToolCallID
			item["content"] = m.Content
		case len(m.ToolCalls) > 0:
			tcs := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			item["tool_calls"] = tcs
			if m.Content != "" {
				item["content"] = m.Content
			}
		default:
			item["content"] = m.Content
		}
		if m.ReasoningContent != "" {
			item["reasoning_content"] = m.ReasoningContent
		}
		out = append(out, item)
	}
	return out
}

func effectiveTemperature(client *Client, req *Request) float64 {
	if req.Temperature != nil {
		return *req.Temperature
	}
	return client.Cfg.Temperature
}
