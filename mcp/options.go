package mcp

import (
	"time"

	"fxcore/httpclient"
)

// ClientOption mutates the Config before a Client is built.
type ClientOption func(*Config)

// DeepSeekConfig carries DeepSeek-specific overrides.
type DeepSeekConfig struct {
	// EnableThinking turns on the model's thinking mode if supported.
	EnableThinking bool
}

// QwenConfig carries Qwen (DashScope compatible-mode) overrides.
type QwenConfig struct {
	// EnableThinking turns on qwen reasoning mode.
	EnableThinking bool
}

// MiniMaxConfig carries MiniMax overrides.
type MiniMaxConfig struct {
	// GroupID is required by some MiniMax accounts for billing.
	GroupID string
}

func WithLogger(l Logger) ClientOption {
	return func(c *Config) { c.Logger = l }
}

func WithHTTPClient(hc *httpclient.Client) ClientOption {
	return func(c *Config) { c.HTTPClient = hc }
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Config) { c.Timeout = d }
}

func WithMaxRetries(n int) ClientOption {
	return func(c *Config) { c.MaxRetries = n }
}

func WithRetryWaitBase(d time.Duration) ClientOption {
	return func(c *Config) { c.RetryWaitBase = d }
}

func WithMaxTokens(n int) ClientOption {
	return func(c *Config) { c.MaxTokens = n }
}

func WithMaxContext(n int) ClientOption {
	return func(c *Config) { c.MaxContext = n }
}

func WithTemperature(t float64) ClientOption {
	return func(c *Config) { c.Temperature = t }
}

func WithAPIKey(k string) ClientOption {
	return func(c *Config) { c.APIKey = k }
}

func WithBaseURL(u string) ClientOption {
	return func(c *Config) { c.BaseURL = u }
}

func WithModel(m string) ClientOption {
	return func(c *Config) { c.Model = m }
}

func WithProvider(p string) ClientOption {
	return func(c *Config) { c.Provider = p }
}

func WithUseFullURL(b bool) ClientOption {
	return func(c *Config) { c.UseFullURL = b }
}

func WithDeepSeekConfig(cfg DeepSeekConfig) ClientOption {
	return func(c *Config) { c.DeepSeek = cfg }
}

func WithQwenConfig(cfg QwenConfig) ClientOption {
	return func(c *Config) { c.Qwen = cfg }
}

func WithMiniMaxConfig(cfg MiniMaxConfig) ClientOption {
	return func(c *Config) { c.MiniMax = cfg }
}
