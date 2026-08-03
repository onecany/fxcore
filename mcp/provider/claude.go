package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"fxcore/httpclient"
	"fxcore/mcp"
)

// claudeClient implements the Anthropic Messages wire format (spec 2.7) on
// top of the shared *Client framework: same retry/timeout/logging, different
// request body, auth header and response parsing.
type claudeClient struct {
	*mcp.Client
}

// claudeHooks overrides the Anthropic-specific surface; Call, BuildRequest,
// MarshalRequestBody and IsRetryableError delegate to the embedded default
// hooks.
type claudeHooks struct {
	mcp.ClientHooks
}

func init() {
	mcp.RegisterProvider(mcp.ProviderClaude, func(opts ...mcp.ClientOption) mcp.AIClient {
		c := mcp.NewClient(append(opts, mcp.WithProvider(mcp.ProviderClaude))...)
		cl := c.(*mcp.Client)
		cl.Hooks = claudeHooks{ClientHooks: cl.Hooks}
		return &claudeClient{Client: cl}
	})
}

func (c *claudeClient) SetAPIKey(apiKey, customURL, customModel string) {
	c.Client.SetAPIKey(apiKey, customURL, customModel)
	c.Log.Debugf("mcp: claude API key configured: %s", maskKey(apiKey))
}

// ── wire format ─────────────────────────────────────────────────────────────

func (h claudeHooks) BuildUrl(client *mcp.Client, _ *mcp.Request) string {
	if client.UseFullURL {
		return client.BaseURL
	}
	return strings.TrimRight(client.BaseURL, "/") + "/v1/messages"
}

func (h claudeHooks) SetAuthHeader(client *mcp.Client, req *httpclient.Request) {
	if client.APIKey != "" {
		req.Header.Set("x-api-key", client.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func (h claudeHooks) BuildRequestBodyFromRequest(client *mcp.Client, req *mcp.Request) ([]byte, error) {
	return buildClaudeBody(client, req, false)
}

func (h claudeHooks) BuildMCPRequestBody(client *mcp.Client, req *mcp.Request) ([]byte, error) {
	return buildClaudeBody(client, req, true)
}

// buildClaudeBody renders the Anthropic Messages payload: system prompt at
// the top level, system roles stripped from messages, tools as
// name/description/input_schema, tool_choice as an object.
func buildClaudeBody(client *mcp.Client, req *mcp.Request, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = client.Model
	}

	var system strings.Builder
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			if system.Len() > 0 {
				system.WriteString("\n")
			}
			system.WriteString(m.Content)
			continue
		}
		msgs = append(msgs, claudeMessage(m))
	}

	body := map[string]any{
		"model":       model,
		"messages":    msgs,
		"max_tokens":  effectiveMaxTokens(client, req),
		"stream":      stream || req.Stream,
		"temperature": claudeTemperature(client, req),
	}
	if system.Len() > 0 {
		body["system"] = system.String()
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		body["stop_sequences"] = req.Stop
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"name":         t.Function.Name,
				"description":  t.Function.Description,
				"input_schema": t.Function.Parameters,
			})
		}
		body["tools"] = tools
		if req.ToolChoice != "" {
			body["tool_choice"] = map[string]any{"type": req.ToolChoice}
		}
	}
	return json.Marshal(body)
}

func effectiveMaxTokens(client *mcp.Client, req *mcp.Request) int {
	if req.MaxTokens != nil {
		return *req.MaxTokens
	}
	return client.Cfg.MaxTokens
}

func claudeTemperature(client *mcp.Client, req *mcp.Request) float64 {
	if req.Temperature != nil {
		return *req.Temperature
	}
	return client.Cfg.Temperature
}

// claudeMessage converts a message to the Anthropic shape:
//   - assistant tool calls → content blocks of type tool_use (input as object)
//   - tool results      → role=user + content block type tool_result
//   - everything else   → role/content string pair
func claudeMessage(m mcp.Message) map[string]any {
	switch {
	case len(m.ToolCalls) > 0:
		content := make([]map[string]any, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			var input map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]any{"raw": tc.Function.Arguments}
			}
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}
		return map[string]any{"role": "assistant", "content": content}
	case m.Role == "tool":
		return map[string]any{
			"role": "user",
			"content": []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallID,
				"content":     m.Content,
			}},
		}
	default:
		return map[string]any{"role": m.Role, "content": m.Content}
	}
}

// ── response parsing ────────────────────────────────────────────────────────

// claudeWireResponse mirrors the Anthropic Messages JSON envelope.
type claudeWireResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h claudeHooks) ParseMCPResponse(client *mcp.Client, body []byte) (string, error) {
	resp, err := h.ParseMCPResponseFull(client, body)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (h claudeHooks) ParseMCPResponseFull(client *mcp.Client, body []byte) (*mcp.LLMResponse, error) {
	var parsed claudeWireResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("mcp: parse claude response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("mcp: claude error (%s): %s", parsed.Error.Type, parsed.Error.Message)
	}
	resp := &mcp.LLMResponse{}
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			resp.Content += block.Text
		case "tool_use":
			var args string
			if len(block.Input) > 0 {
				args = string(block.Input)
			}
			resp.ToolCalls = append(resp.ToolCalls, mcp.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: mcp.ToolCallFunction{
					Name:      block.Name,
					Arguments: args,
				},
			})
		}
	}
	if parsed.Usage != nil && parsed.Usage.InputTokens+parsed.Usage.OutputTokens > 0 {
		mcp.ReportStreamUsage(client, mcp.TokenUsage{
			Provider:         client.Provider,
			Model:            client.Model,
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		})
	}
	return resp, nil
}
