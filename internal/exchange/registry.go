package exchange

import (
	"fmt"
	"sync"
)

// constructor 适配器构造器。
type constructor func(creds Credentials) (Adapter, error)

var (
	regMu    sync.RWMutex
	registry = map[string]constructor{} // 各适配器 init() 自注册（依赖序提交友好）
)

// Register 注册适配器构造器（测试/第三方扩展用）。
func Register(exType string, c constructor) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[exType] = c
}

// newFromRegistry 按类型构造适配器。
func newFromRegistry(creds Credentials) (Adapter, error) {
	regMu.RLock()
	c, ok := registry[creds.ExchangeType]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: exchange type %q has no adapter", ErrNotImplemented, creds.ExchangeType)
	}
	return c(creds)
}
