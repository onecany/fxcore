// Package llm 统一 AI 客户端（API设计.md §16 输出契约的执行端）。
// 支持 deepseek/qwen/gpt（OpenAI 兼容）、claude（Anthropic 格式）、
// gemini（generateContent）、custom（OpenAI 兼容 + 自定义 base_url，SSRF 校验）。
// 超时归 ErrTimeout（映射 1302），其余失败归 ErrProvider（映射 1301）。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fxcore/httpclient"
)

// 错误分类：调用方据此映射业务错误码。
var (
	ErrTimeout  = errors.New("llm: timeout")
	ErrProvider = errors.New("llm: provider error")
)

// Message 对话消息。
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Model 模型配置（由组装层从 store 解密组装，llm 包不感知存储）。
type Model struct {
	Provider  string // deepseek | qwen | claude | gpt | gemini | custom
	ModelName string
	APIKey    string
	BaseURL   string // custom provider 的 base_url；其余为空用内置端点
}

// ModelProvider 模型配置访问接口（用户偏好：消费者定义接口、组装层注入）。
type ModelProvider interface {
	// GetModel 按模型 ID 取配置（含解密后的 API Key）。不存在返回 false。
	GetModel(id string) (*Model, bool)
}

// ChatRequest 请求参数。
type ChatRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// ChatResult 响应。
type ChatResult struct {
	Content   string // 模型文本输出
	LatencyMS int64
}

// Client 统一客户端。
type Client struct {
	http *http.Client
}

// New 构造。timeout 为单次调用的总超时（AI 决策建议 ≥60s）。
func New(timeout time.Duration) *Client {
	tr := httpclient.DefaultTransport().Clone()
	return NewWithTransport(tr, timeout)
}

// NewWithTransport 注入自定义 transport（测试/代理场景）。
func NewWithTransport(tr http.RoundTripper, timeout time.Duration) *Client {
	return &Client{
		http: &http.Client{
			Timeout:   timeout,
			Transport: tr,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 禁重定向（防 API Key 泄往攻击 URL）
			},
		},
	}
}

// Chat 调用对话补全。ctx 超时或客户端超时归 ErrTimeout。
func (c *Client) Chat(ctx context.Context, m *Model, req ChatRequest) (*ChatResult, error) {
	if m == nil || m.Provider == "" {
		return nil, fmt.Errorf("%w: missing provider", ErrProvider)
	}
	endpoint, err := buildEndpoint(m)
	if err != nil {
		return nil, err
	}
	body, headers, err := buildPayload(m, req)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrProvider, err)
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("User-Agent", "fxcore/1.0")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		if isTimeoutErr(err) || ctx.Err() == context.DeadlineExceeded {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MiB 上限
	latency := time.Since(start).Milliseconds()

	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("%w: status %d: %s", ErrProvider, resp.StatusCode, truncate(string(raw), 300))
	}
	content, err := parseContent(m.Provider, raw)
	if err != nil {
		return nil, err
	}
	return &ChatResult{Content: content, LatencyMS: latency}, nil
}

// buildEndpoint 构造端点 URL。
func buildEndpoint(m *Model) (string, error) {
	switch m.Provider {
	case "deepseek":
		return "https://api.deepseek.com/chat/completions", nil
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", nil
	case "gpt":
		return "https://api.openai.com/v1/chat/completions", nil
	case "claude":
		return "https://api.anthropic.com/v1/messages", nil
	case "gemini":
		return fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", url.PathEscape(m.ModelName)), nil
	case "custom":
		if m.BaseURL == "" {
			return "", fmt.Errorf("%w: custom provider requires base_url", ErrProvider)
		}
		if err := validateBaseURL(m.BaseURL); err != nil {
			return "", err // SSRF 防护
		}
		return strings.TrimRight(m.BaseURL, "/") + "/chat/completions", nil
	default:
		return "", fmt.Errorf("%w: unsupported provider %q", ErrProvider, m.Provider)
	}
}

// buildPayload 构造请求体与鉴权头。
func buildPayload(m *Model, req ChatRequest) ([]byte, map[string]string, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	if m.Provider == "claude" {
		headers["x-api-key"] = m.APIKey
		headers["anthropic-version"] = "2023-06-01"
		payload := map[string]any{
			"model":      m.ModelName,
			"max_tokens": req.MaxTokens,
			"messages":   req.Messages,
		}
		if req.Temperature != 0 {
			payload["temperature"] = req.Temperature
		}
		body, err := json.Marshal(payload)
		return body, headers, err
	}
	if m.Provider == "gemini" {
		headers["x-goog-api-key"] = m.APIKey
		contents := make([]map[string]any, 0, len(req.Messages))
		for _, msg := range req.Messages {
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []map[string]string{{"text": msg.Content}},
			})
		}
		payload := map[string]any{"contents": contents}
		if req.Temperature != 0 {
			payload["generationConfig"] = map[string]any{"temperature": req.Temperature}
		}
		body, err := json.Marshal(payload)
		return body, headers, err
	}
	// OpenAI 兼容（deepseek/qwen/gpt/custom）
	headers["Authorization"] = "Bearer " + m.APIKey
	payload := map[string]any{
		"model":    m.ModelName,
		"messages": req.Messages,
	}
	if req.Temperature != 0 {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	body, err := json.Marshal(payload)
	return body, headers, err
}

// parseContent 按 provider 解析文本输出。
func parseContent(provider string, raw []byte) (string, error) {
	switch provider {
	case "claude":
		var r struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", fmt.Errorf("%w: parse claude response: %v", ErrProvider, err)
		}
		if r.Error != nil {
			return "", fmt.Errorf("%w: %s", ErrProvider, r.Error.Message)
		}
		var sb strings.Builder
		for _, part := range r.Content {
			if part.Type == "text" {
				sb.WriteString(part.Text)
			}
		}
		return sb.String(), nil
	case "gemini":
		var r struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", fmt.Errorf("%w: parse gemini response: %v", ErrProvider, err)
		}
		if r.Error != nil {
			return "", fmt.Errorf("%w: %s", ErrProvider, r.Error.Message)
		}
		if len(r.Candidates) == 0 {
			return "", fmt.Errorf("%w: empty candidates", ErrProvider)
		}
		var sb strings.Builder
		for _, p := range r.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
		return sb.String(), nil
	default: // OpenAI 兼容
		var r struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return "", fmt.Errorf("%w: parse openai response: %v", ErrProvider, err)
		}
		if r.Error != nil {
			return "", fmt.Errorf("%w: %s", ErrProvider, r.Error.Message)
		}
		if len(r.Choices) == 0 {
			return "", fmt.Errorf("%w: empty choices", ErrProvider)
		}
		return r.Choices[0].Message.Content, nil
	}
}

// validateBaseURL custom provider SSRF 校验：https-only + 解析后 IP 拒绝私网。
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: invalid base_url: %v", ErrProvider, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: custom base_url must use https", ErrProvider)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: custom base_url missing host", ErrProvider)
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("%w: resolve base_url host: %v", ErrProvider, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("%w: base_url resolves to blocked network (%s)", ErrProvider, ip)
		}
	}
	return nil
}

func isTimeoutErr(err error) bool {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
