package mcp

import "sync"

var (
	registryMu       sync.RWMutex
	providerRegistry = map[string]func(...ClientOption) AIClient{}
)

// RegisterProvider registers a factory under name. Blank names and nil
// factories are ignored. Registrations are idempotent per name and safe for
// concurrent use.
func RegisterProvider(name string, factory func(...ClientOption) AIClient) {
	if name == "" || factory == nil {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	providerRegistry[name] = factory
}

// NewAIClientByProvider creates a client through the registry. It returns
// nil when name is not registered; callers fall back to New().
func NewAIClientByProvider(name string, opts ...ClientOption) AIClient {
	registryMu.RLock()
	factory, ok := providerRegistry[name]
	registryMu.RUnlock()
	if !ok {
		return nil
	}
	return factory(opts...)
}
