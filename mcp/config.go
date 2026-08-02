package mcp

import (
	"net/http"
	"time"
)

// Provider identifiers. These are external contracts used by the upper
// layers (api/trader/kernel/telegram/store) and must not change.
const (
	ProviderDeepSeek = "deepseek"
	ProviderOpenAI   = "openai"
	ProviderClaude   = "claude"
	ProviderQwen     = "qwen"
	ProviderGemini   = "gemini"
	ProviderGrok     = "grok"
	ProviderKimi     = "kimi"
	ProviderMiniMax  = "minimax"
	ProviderClaw402  = "claw402"
	ProviderCustom   = "custom"
)

// Default base URLs (external facts, see spec 1.3).
const (
	DefaultDeepSeekBaseURL = "https://api.deepseek.com"
	DefaultOpenAIBaseURL   = "https://api.openai.com/v1"
	DefaultClaudeBaseURL   = "https://api.anthropic.com/v1"
	DefaultQwenBaseURL     = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	DefaultGeminiBaseURL   = "https://generativelanguage.googleapis.com/v1beta/openai"
	DefaultGrokBaseURL     = "https://api.x.ai/v1"
	DefaultKimiBaseURL     = "https://api.moonshot.ai/v1"
	DefaultMiniMaxBaseURL  = "https://api.minimax.io/v1"
)

// Default model names (external facts, see spec 1.3).
const (
	DefaultDeepSeekModel = "deepseek-v4-flash"
	DefaultOpenAIModel   = "gpt-5.4"
	DefaultClaudeModel   = "claude-opus-4-6"
	DefaultQwenModel     = "qwen3-max"
	DefaultGeminiModel   = "gemini-3.1-pro"
	DefaultGrokModel     = "grok-3-latest"
	DefaultKimiModel     = "moonshot-v1-auto"
	DefaultMiniMaxModel  = "MiniMax-M2.7"
)

// Timeouts and retry defaults.
const (
	DefaultTimeout       = 120 * time.Second
	MaxRetryTimes        = 3
	DefaultRetryWaitBase = 2 * time.Second
	MCPClientTemperature = 0.5
)

// DefaultMaxTokens is the fallback token cap when AI_MAX_TOKENS is unset.
const DefaultMaxTokens = 2000

// Config carries the resolved client configuration. Options mutate it before
// the Client is constructed.
type Config struct {
	Provider        string
	APIKey          string
	BaseURL         string
	Model           string
	MaxTokens       int
	MaxContext      int
	Temperature     float64
	UseFullURL      bool
	MaxRetries      int
	RetryWaitBase   time.Duration
	RetryableErrors []string
	Timeout         time.Duration
	Logger          Logger
	HTTPClient      *http.Client

	// Provider-specific extension configs (set via With*Config options).
	DeepSeek DeepSeekConfig
	Qwen     QwenConfig
	MiniMax  MiniMaxConfig
}

// Client is the concrete AI client. Upper layers read Provider and Model
// directly (e.g. kernel/engine_analysis.go), so they are exported fields.
type Client struct {
	Provider   string
	APIKey     string
	BaseURL    string
	Model      string
	UseFullURL bool
	MaxTokens  int
	HTTPClient *http.Client
	Log        Logger
	Cfg        *Config
	Hooks      ClientHooks
}

// ClientHooks is the provider polymorphism seam (spec 2.2): a provider type
// overrides request-body construction, URL building, auth headers, response
// parsing and retry classification while sharing the retry/timeout/logging
// framework of *Client.
type ClientHooks interface {
	Call(client *Client, req *http.Request) (*http.Response, error)
	BuildMCPRequestBody(client *Client, req *Request) ([]byte, error)
	BuildUrl(client *Client, req *Request) string
	BuildRequest(client *Client, req *Request) (*http.Request, error)
	SetAuthHeader(client *Client, req *http.Request)
	MarshalRequestBody(client *Client, payload any) ([]byte, error)
	BuildRequestBodyFromRequest(client *Client, req *Request) ([]byte, error)
	ParseMCPResponse(client *Client, body []byte) (string, error)
	ParseMCPResponseFull(client *Client, body []byte) (*LLMResponse, error)
	IsRetryableError(client *Client, err error) bool
}

// TokenUsageCallback is invoked with usage statistics after each call that
// reports a total token count greater than zero. Set by config/config.go.
var TokenUsageCallback func(TokenUsage)
