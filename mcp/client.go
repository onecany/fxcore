package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// AIClient is the unified model client interface (spec 2.1). Every provider
// client — including the payment-backed claw402 client — implements it.
type AIClient interface {
	// SetAPIKey configures credentials. customURL/customModel override the
	// provider defaults when non-empty; a customURL ending in "#" marks the
	// value as the complete endpoint (no path suffix is appended).
	SetAPIKey(apiKey, customURL, customModel string)
	// SetTimeout changes the HTTP client timeout.
	SetTimeout(d time.Duration)
	CallWithMessages(system, user string) (string, error)
	CallWithRequest(req *Request) (string, error)
	// CallWithRequestStream streams SSE chunks; onChunk receives the
	// cumulative text at each step.
	CallWithRequestStream(req *Request, onChunk func(string)) (string, error)
	// CallWithRequestFull returns text plus any tool calls.
	CallWithRequestFull(req *Request) (*LLMResponse, error)
}

// ClientEmbedder exposes the underlying *Client so upper layers can read
// Provider/Model (kernel/engine_analysis.go).
type ClientEmbedder interface {
	BaseClient() *Client
}

// HTTPError wraps a non-2xx upstream response. The message contains
// "status NNN" so the retry classifier matches the spec 2.5.2 patterns.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("mcp: status %d: %s", e.Status, e.Body)
}

// New returns a default client (ProviderCustom with the system-default
// deepseek endpoint/model as fallback). Used for fully custom endpoints.
func New() AIClient {
	return NewClient()
}

// NewClient builds a Client from options, filling defaults for every
// unset field.
func NewClient(opts ...ClientOption) AIClient {
	cfg := defaultConfig()
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}
	applyProviderDefaults(cfg)
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	if cfg.Logger == nil {
		cfg.Logger = NewNoopLogger()
	}
	if len(cfg.RetryableErrors) == 0 {
		cfg.RetryableErrors = defaultRetryablePatterns()
	}

	cl := &Client{
		Provider:   cfg.Provider,
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		UseFullURL: cfg.UseFullURL,
		MaxTokens:  cfg.MaxTokens,
		HTTPClient: cfg.HTTPClient,
		Log:        cfg.Logger,
		Cfg:        cfg,
	}
	cl.Hooks = defaultHooks{}
	return cl
}

// BaseClient implements ClientEmbedder.
func (c *Client) BaseClient() *Client { return c }

// SetAPIKey stores credentials and applies overrides. A customURL ending in
// "#" is treated as the complete endpoint (spec 2.5.8).
func (c *Client) SetAPIKey(apiKey, customURL, customModel string) {
	c.APIKey = apiKey
	if customURL != "" {
		if strings.HasSuffix(customURL, "#") {
			c.BaseURL = strings.TrimSuffix(customURL, "#")
			c.UseFullURL = true
		} else {
			c.BaseURL = customURL
		}
	}
	if customModel != "" {
		c.Model = customModel
	}
	if c.Cfg != nil {
		c.Cfg.APIKey = c.APIKey
		c.Cfg.BaseURL = c.BaseURL
		c.Cfg.Model = c.Model
		c.Cfg.UseFullURL = c.UseFullURL
	}
}

// SetTimeout updates the HTTP client timeout in place.
func (c *Client) SetTimeout(d time.Duration) {
	if c.Cfg != nil {
		c.Cfg.Timeout = d
	}
	if c.HTTPClient != nil {
		c.HTTPClient.Timeout = d
	}
}

// CallWithMessages is the convenience two-prompt entry point.
func (c *Client) CallWithMessages(system, user string) (string, error) {
	msgs := make([]Message, 0, 2)
	if system != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: user})
	return c.CallWithRequest(&Request{Messages: msgs})
}

// CallWithRequest returns the plain-text content of a model call.
func (c *Client) CallWithRequest(req *Request) (string, error) {
	body, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	text, err := c.Hooks.ParseMCPResponse(c, body)
	if err != nil {
		return "", err
	}
	return text, nil
}

// CallWithRequestFull returns text plus any tool calls.
func (c *Client) CallWithRequestFull(req *Request) (*LLMResponse, error) {
	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.Hooks.ParseMCPResponseFull(c, body)
}

// CallWithRequestStream performs an SSE streaming call. onChunk receives the
// cumulative text at each delta; the returned string is the full text.
// A 60s gap between data events cancels the connection (spec 2.5.7).
func (c *Client) CallWithRequestStream(req *Request, onChunk func(string)) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("mcp: API key is empty")
	}
	streamReq := *req
	streamReq.Stream = true
	streamReq = *maybeTruncate(c, &streamReq)

	body, err := c.Hooks.BuildMCPRequestBody(c, &streamReq)
	if err != nil {
		return "", fmt.Errorf("mcp: build stream body: %w", err)
	}
	ctx := streamReq.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Hooks.BuildUrl(c, &streamReq), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("mcp: build stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.Hooks.SetAuthHeader(c, httpReq)

	resp, err := c.Hooks.Call(c, httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", &HTTPError{Status: resp.StatusCode, Body: string(raw)}
	}

	var accumulated strings.Builder
	var usage TokenUsage
	idleTimeout := 60 * time.Second

	lines := make(chan string)
	errCh := make(chan error, 1)
	done := make(chan struct{})
	defer close(done) // unblocks the reader goroutine on every exit path
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-done:
				return
			}
		}
		select {
		case errCh <- scanner.Err():
		case <-done:
		}
	}()

	idle := time.NewTimer(idleTimeout)
	defer idle.Stop()

	for {
		select {
		case line := <-lines:
			idle.Reset(idleTimeout)
			handleSSELine(line, &accumulated, &usage, onChunk)
		case err := <-errCh:
			if err != nil {
				return accumulated.String(), fmt.Errorf("mcp: read stream: %w", err)
			}
			ReportStreamUsage(c, usage)
			return accumulated.String(), nil
		case <-idle.C:
			return accumulated.String(), fmt.Errorf("mcp: stream idle timeout (%s without data)", idleTimeout)
		}
	}
}

// handleSSELine parses one SSE data line, accumulating content deltas and
// capturing usage. Tool-call deltas in streaming mode are intentionally not
// aggregated — CallWithRequestStream is a text path.
func handleSSELine(line string, accumulated *strings.Builder, usage *TokenUsage, onChunk func(string)) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return
	}
	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta
		accumulated.WriteString(delta.Content)
		if onChunk != nil {
			onChunk(accumulated.String())
		}
	}
	if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
		usage.Provider = ""
		usage.Model = ""
		usage.PromptTokens = chunk.Usage.PromptTokens
		usage.CompletionTokens = chunk.Usage.CompletionTokens
		usage.TotalTokens = chunk.Usage.TotalTokens
	}
}

// doRequest executes the request with the retry policy (spec 2.5.1): at most
// MaxRetries retries, backoff = RetryWaitBase × attempt, only for errors
// matching the retryable patterns. An empty API key fails immediately.
func (c *Client) doRequest(req *Request) ([]byte, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("mcp: API key is empty")
	}
	req = maybeTruncate(c, req)
	maxAttempts := c.Cfg.MaxRetries + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		httpReq, err := c.Hooks.BuildRequest(c, req)
		if err != nil {
			return nil, err
		}
		resp, err := c.Hooks.Call(c, httpReq)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts && c.Hooks.IsRetryableError(c, err) {
				if !c.backoff(attempt, req.Ctx) {
					return nil, ctxErr(req.Ctx)
				}
				continue
			}
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxAttempts && c.Hooks.IsRetryableError(c, readErr) {
				if !c.backoff(attempt, req.Ctx) {
					return nil, ctxErr(req.Ctx)
				}
				continue
			}
			return nil, readErr
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		lastErr = &HTTPError{Status: resp.StatusCode, Body: string(body)}
		if attempt < maxAttempts && c.Hooks.IsRetryableError(c, lastErr) {
			if !c.backoff(attempt, req.Ctx) {
				return nil, ctxErr(req.Ctx)
			}
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

// backoff sleeps RetryWaitBase × attempt, aborting early when ctx is
// cancelled. Returns false when the caller should give up.
func (c *Client) backoff(attempt int, ctx context.Context) bool {
	wait := c.Cfg.RetryWaitBase * time.Duration(attempt)
	if ctx == nil {
		time.Sleep(wait)
		return true
	}
	select {
	case <-time.After(wait):
		return true
	case <-ctx.Done():
		return false
	}
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func defaultConfig() *Config {
	maxTokens := DefaultMaxTokens
	if v := os.Getenv("AI_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxTokens = n
		}
	}
	return &Config{
		Provider:        ProviderCustom,
		Temperature:     MCPClientTemperature,
		MaxTokens:       maxTokens,
		MaxRetries:      MaxRetryTimes,
		RetryWaitBase:   DefaultRetryWaitBase,
		Timeout:         DefaultTimeout,
		RetryableErrors: defaultRetryablePatterns(),
	}
}

func applyProviderDefaults(cfg *Config) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURLFor(cfg.Provider)
	}
	if cfg.Model == "" {
		cfg.Model = defaultModelFor(cfg.Provider)
	}
}

func defaultBaseURLFor(provider string) string {
	switch provider {
	case ProviderDeepSeek:
		return DefaultDeepSeekBaseURL
	case ProviderOpenAI:
		return DefaultOpenAIBaseURL
	case ProviderClaude:
		return DefaultClaudeBaseURL
	case ProviderQwen:
		return DefaultQwenBaseURL
	case ProviderGemini:
		return DefaultGeminiBaseURL
	case ProviderGrok:
		return DefaultGrokBaseURL
	case ProviderKimi:
		return DefaultKimiBaseURL
	case ProviderMiniMax:
		return DefaultMiniMaxBaseURL
	case ProviderClaw402:
		return "https://claw402.ai"
	default:
		// ProviderCustom has no dedicated default; the system default model
		// (deepseek) is the pragmatic fallback used across fxcore.
		return DefaultDeepSeekBaseURL
	}
}

func defaultModelFor(provider string) string {
	switch provider {
	case ProviderDeepSeek:
		return DefaultDeepSeekModel
	case ProviderOpenAI:
		return DefaultOpenAIModel
	case ProviderClaude:
		return DefaultClaudeModel
	case ProviderQwen:
		return DefaultQwenModel
	case ProviderGemini:
		return DefaultGeminiModel
	case ProviderGrok:
		return DefaultGrokModel
	case ProviderKimi:
		return DefaultKimiModel
	case ProviderMiniMax:
		return DefaultMiniMaxModel
	default:
		return DefaultDeepSeekModel
	}
}
