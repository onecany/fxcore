package mcp

import (
	"net/http"
	"testing"
	"time"
)

// ── spec 1.3: endpoint & model constants ────────────────────────────────────

func TestProviderBaseURLsAndModels(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		model   string
	}{
		{ProviderDeepSeek, DefaultDeepSeekBaseURL, DefaultDeepSeekModel},
		{ProviderOpenAI, DefaultOpenAIBaseURL, DefaultOpenAIModel},
		{ProviderClaude, DefaultClaudeBaseURL, DefaultClaudeModel},
		{ProviderQwen, DefaultQwenBaseURL, DefaultQwenModel},
		{ProviderGemini, DefaultGeminiBaseURL, DefaultGeminiModel},
		{ProviderGrok, DefaultGrokBaseURL, DefaultGrokModel},
		{ProviderKimi, DefaultKimiBaseURL, DefaultKimiModel},
		{ProviderMiniMax, DefaultMiniMaxBaseURL, DefaultMiniMaxModel},
	}
	want := map[string][2]string{
		ProviderDeepSeek: {"https://api.deepseek.com", "deepseek-v4-flash"},
		ProviderOpenAI:   {"https://api.openai.com/v1", "gpt-5.4"},
		ProviderClaude:   {"https://api.anthropic.com/v1", "claude-opus-4-6"},
		ProviderQwen:     {"https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen3-max"},
		ProviderGemini:   {"https://generativelanguage.googleapis.com/v1beta/openai", "gemini-3.1-pro"},
		ProviderGrok:     {"https://api.x.ai/v1", "grok-3-latest"},
		ProviderKimi:     {"https://api.moonshot.ai/v1", "moonshot-v1-auto"},
		ProviderMiniMax:  {"https://api.minimax.io/v1", "MiniMax-M2.7"},
	}
	for _, tc := range cases {
		w := want[tc.name]
		if tc.baseURL != w[0] {
			t.Errorf("%s base URL = %q, want %q", tc.name, tc.baseURL, w[0])
		}
		if tc.model != w[1] {
			t.Errorf("%s model = %q, want %q", tc.name, tc.model, w[1])
		}
	}
}

func TestProviderConstants(t *testing.T) {
	if ProviderClaw402 != "claw402" {
		t.Errorf("ProviderClaw402 = %q", ProviderClaw402)
	}
	if ProviderCustom != "custom" {
		t.Errorf("ProviderCustom = %q", ProviderCustom)
	}
	if DefaultTimeout != 120*time.Second {
		t.Errorf("DefaultTimeout = %v", DefaultTimeout)
	}
	if MaxRetryTimes != 3 {
		t.Errorf("MaxRetryTimes = %d", MaxRetryTimes)
	}
	if MCPClientTemperature != 0.5 {
		t.Errorf("MCPClientTemperature = %v", MCPClientTemperature)
	}
}

// ── spec 1.1: New / NewClient defaults ──────────────────────────────────────

func TestNewReturnsUsableClient(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	cl, ok := c.(*Client)
	if !ok {
		t.Fatalf("New() returned %T, want *Client", c)
	}
	if cl.Cfg == nil {
		t.Fatal("Cfg is nil")
	}
	if cl.Hooks == nil {
		t.Fatal("Hooks is nil")
	}
	if cl.Log == nil {
		t.Fatal("Log is nil")
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cl := NewClient().(*Client)
	cfg := cl.Cfg
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("default Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.MaxRetries != MaxRetryTimes {
		t.Errorf("default MaxRetries = %d, want %d", cfg.MaxRetries, MaxRetryTimes)
	}
	if cfg.RetryWaitBase != 2*time.Second {
		t.Errorf("default RetryWaitBase = %v, want 2s", cfg.RetryWaitBase)
	}
	if cfg.Temperature != MCPClientTemperature {
		t.Errorf("default Temperature = %v, want %v", cfg.Temperature, MCPClientTemperature)
	}
	if cfg.MaxTokens != 2000 {
		t.Errorf("default MaxTokens = %d, want 2000 (AI_MAX_TOKENS unset)", cfg.MaxTokens)
	}
	if cfg.MaxContext != 0 {
		t.Errorf("default MaxContext = %d, want 0", cfg.MaxContext)
	}
	if cfg.HTTPClient == nil {
		t.Error("default HTTPClient is nil")
	}
	if cl.MaxTokens != cfg.MaxTokens {
		t.Errorf("Client.MaxTokens = %d, want %d", cl.MaxTokens, cfg.MaxTokens)
	}
}

func TestAI_MAX_TOKENSEnvIsHonored(t *testing.T) {
	t.Setenv("AI_MAX_TOKENS", "777")
	cl := NewClient().(*Client)
	if cl.Cfg.MaxTokens != 777 {
		t.Errorf("MaxTokens = %d, want 777 from AI_MAX_TOKENS", cl.Cfg.MaxTokens)
	}
}

func TestOptionsOverrideConfig(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	cl := NewClient(
		WithAPIKey("secret-key"),
		WithBaseURL("https://example.com/v1"),
		WithModel("test-model"),
		WithProvider("testprov"),
		WithMaxTokens(100),
		WithMaxContext(4096),
		WithTemperature(0.9),
		WithTimeout(30*time.Second),
		WithMaxRetries(5),
		WithRetryWaitBase(time.Second),
		WithUseFullURL(true),
		WithHTTPClient(hc),
	).(*Client)

	if cl.Cfg.APIKey != "secret-key" {
		t.Errorf("APIKey = %q", cl.Cfg.APIKey)
	}
	if cl.Cfg.BaseURL != "https://example.com/v1" {
		t.Errorf("BaseURL = %q", cl.Cfg.BaseURL)
	}
	if cl.Cfg.Model != "test-model" {
		t.Errorf("Model = %q", cl.Cfg.Model)
	}
	if cl.Cfg.Provider != "testprov" {
		t.Errorf("Provider = %q", cl.Cfg.Provider)
	}
	if cl.Cfg.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d", cl.Cfg.MaxTokens)
	}
	if cl.Cfg.MaxContext != 4096 {
		t.Errorf("MaxContext = %d", cl.Cfg.MaxContext)
	}
	if cl.Cfg.Temperature != 0.9 {
		t.Errorf("Temperature = %v", cl.Cfg.Temperature)
	}
	if cl.Cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", cl.Cfg.Timeout)
	}
	if cl.Cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d", cl.Cfg.MaxRetries)
	}
	if cl.Cfg.RetryWaitBase != time.Second {
		t.Errorf("RetryWaitBase = %v", cl.Cfg.RetryWaitBase)
	}
	if !cl.Cfg.UseFullURL {
		t.Error("UseFullURL = false, want true")
	}
	if cl.Cfg.HTTPClient != hc {
		t.Error("HTTPClient option not applied")
	}
	// Client fields mirror config for the fields upper layers read directly.
	if cl.Provider != "testprov" || cl.Model != "test-model" || cl.MaxTokens != 100 {
		t.Error("Client fields do not mirror Config")
	}
}

func TestConfigHooksOptionsCompile(t *testing.T) {
	// These options exist for API compatibility; exercising them must not panic.
	_ = NewClient(
		WithDeepSeekConfig(DeepSeekConfig{}),
		WithQwenConfig(QwenConfig{}),
		WithMiniMaxConfig(MiniMaxConfig{}),
	)
}

// ── TokenUsage.Channel (used by config.go / telemetry) ─────────────────────

func TestTokenUsageChannel(t *testing.T) {
	if got := (TokenUsage{Provider: ProviderClaw402}).Channel(); got != "claw402" {
		t.Errorf("claw402 Channel() = %q, want \"claw402\"", got)
	}
	if got := (TokenUsage{Provider: ProviderDeepSeek}).Channel(); got != "native" {
		t.Errorf("deepseek Channel() = %q, want \"native\"", got)
	}
}

// ── NewNoopLogger ───────────────────────────────────────────────────────────

func TestNewNoopLoggerDoesNotPanic(t *testing.T) {
	l := NewNoopLogger()
	l.Debugf("debug %d", 1)
	l.Infof("info")
	l.Warnf("warn %s", "x")
	l.Errorf("error %v", nil)
}
