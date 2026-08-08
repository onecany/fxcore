package service

import (
	"encoding/json"

	"fxcore/internal/llm"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// LLMModelProvider 实现 llm.ModelProvider 接口（组装层注入，llm 包不感知存储）。
// 从 store 取模型配置 + RSA 解密 API Key；custom provider 的 base_url 从 Config JSON 解析。
type LLMModelProvider struct {
	store *store.Store
	km    *crypto.KeyManager
}

// NewLLMModelProvider 构造。
func NewLLMModelProvider(s *store.Store, km *crypto.KeyManager) *LLMModelProvider {
	return &LLMModelProvider{store: s, km: km}
}

// GetModel 按模型 ID 取配置（含解密后的 API Key）。
func (p *LLMModelProvider) GetModel(id string) (*llm.Model, bool) {
	m, ok := p.store.GetModel(id)
	if !ok {
		return nil, false
	}
	plain, err := p.km.Decrypt(m.APIKeyEnc)
	if err != nil {
		return nil, false
	}
	cfg := &llm.Model{
		Provider:  m.Provider,
		ModelName: m.ModelName,
		APIKey:    string(plain),
	}
	if m.Provider == "custom" && m.Config != "" {
		var c struct {
			BaseURL string `json:"base_url"`
		}
		if json.Unmarshal([]byte(m.Config), &c) == nil {
			cfg.BaseURL = c.BaseURL
		}
	}
	return cfg, true
}
