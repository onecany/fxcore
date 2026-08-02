// Package provider registers the built-in LLM provider clients. Each file
// in this package registers one provider via init(); the shared
// OpenAI-compatible implementation lives here.
package provider

import "fxcore/mcp"

// openAICompatClient is the shared implementation for providers whose wire
// format is OpenAI chat completions: deepseek, openai, qwen, gemini, grok,
// minimax (and kimi, which pins temperature separately).
type openAICompatClient struct {
	*mcp.Client
	name string
}

// SetAPIKey stores the key (logging a masked form) and applies custom
// URL/model overrides.
func (c *openAICompatClient) SetAPIKey(apiKey, customURL, customModel string) {
	c.Client.SetAPIKey(apiKey, customURL, customModel)
	c.Log.Debugf("mcp: %s API key configured: %s", c.name, maskKey(apiKey))
}

// maskKey shows only the first and last 4 characters.
func maskKey(k string) string {
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "***" + k[len(k)-4:]
}

// registerCompat registers an OpenAI-compatible provider under name with the
// default base URL and model for that provider.
func registerCompat(name string) {
	mcp.RegisterProvider(name, func(opts ...mcp.ClientOption) mcp.AIClient {
		c := mcp.NewClient(append(opts, mcp.WithProvider(name))...)
		cl := c.(*mcp.Client)
		return &openAICompatClient{Client: cl, name: name}
	})
}
