package provider

import (
	"fxcore/mcp"
)

// kimiClient pins the request temperature to 1.0 — the only value Kimi K2.5
// accepts — while delegating everything else to the OpenAI-compatible wire
// format.
type kimiClient struct {
	*mcp.Client
}

// kimiHooks overrides request-body construction, forcing temperature 1.0;
// all other hook methods delegate to the embedded default hooks.
type kimiHooks struct {
	mcp.ClientHooks
}

func init() {
	mcp.RegisterProvider(mcp.ProviderKimi, func(opts ...mcp.ClientOption) mcp.AIClient {
		c := mcp.NewClient(append(opts, mcp.WithProvider(mcp.ProviderKimi))...)
		cl := c.(*mcp.Client)
		cl.Hooks = kimiHooks{ClientHooks: cl.Hooks}
		return &kimiClient{Client: cl}
	})
}

func (c *kimiClient) SetAPIKey(apiKey, customURL, customModel string) {
	c.Client.SetAPIKey(apiKey, customURL, customModel)
	c.Log.Debugf("mcp: kimi API key configured: %s", maskKey(apiKey))
}

func (h kimiHooks) BuildRequestBodyFromRequest(client *mcp.Client, req *mcp.Request) ([]byte, error) {
	return h.withFixedTemperature(client, req, false)
}

func (h kimiHooks) BuildMCPRequestBody(client *mcp.Client, req *mcp.Request) ([]byte, error) {
	return h.withFixedTemperature(client, req, true)
}

func (h kimiHooks) withFixedTemperature(client *mcp.Client, req *mcp.Request, stream bool) ([]byte, error) {
	cp := *req
	one := 1.0
	cp.Temperature = &one
	return h.ClientHooks.BuildRequestBodyFromRequest(client, &cp)
}
