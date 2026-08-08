package service

import (
	"fxcore/internal/exchange"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// CredentialsResolver 实现 trader engine 的 CredentialsResolver 接口（组装层注入）。
// 从 store 取启用的交易所账户 + RSA 解密凭据。
type CredentialsResolver struct {
	store *store.Store
	km    *crypto.KeyManager
}

// NewCredentialsResolver 构造。
func NewCredentialsResolver(s *store.Store, km *crypto.KeyManager) *CredentialsResolver {
	return &CredentialsResolver{store: s, km: km}
}

// Resolve 按交易所类型取凭据（优先启用中的账户）。
func (r *CredentialsResolver) Resolve(exchangeType string) (*exchange.Credentials, bool) {
	for _, e := range r.store.ListExchanges() {
		if e.ExchangeType != exchangeType || !e.Enabled {
			continue
		}
		creds := &exchange.Credentials{
			ExchangeType: exchangeType,
			WalletAddr:   e.HyperliquidWalletAddr,
			Testnet:      e.Testnet,
		}
		var ok = true
		decrypt := func(enc string) string {
			if enc == "" {
				return ""
			}
			plain, err := r.km.Decrypt(enc)
			if err != nil {
				ok = false
				return ""
			}
			return string(plain)
		}
		creds.APIKey = decrypt(e.APIKeyEnc)
		creds.SecretKey = decrypt(e.SecretKeyEnc)
		creds.Passphrase = decrypt(e.PassphraseEnc)
		creds.PrivateKey = decrypt(e.AsterPrivateKeyEnc)
		creds.APIKeyPrivateKey = decrypt(e.LighterAPIKeyPrivateKeyEnc)
		if creds.PrivateKey == "" {
			creds.PrivateKey = decrypt(e.LighterPrivateKeyEnc)
		}
		creds.APIKeyIndex = e.LighterAPIKeyIndex
		if !ok {
			continue // 解密失败跳过该账户
		}
		return creds, true
	}
	return nil, false
}
