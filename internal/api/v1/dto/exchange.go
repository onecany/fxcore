package dto

import "time"

// ========== 交易所模块（API设计.md §10/§11） ==========

// CreateExchangeRequest 创建/更新交易所请求（Partial 语义用指针字段）。
// 凭据字段后端 RSA-OAEP 加密后落库；返回时脱敏（api_key_prefix 风格）。
type CreateExchangeRequest struct {
	ExchangeType              string `json:"exchange_type" binding:"required,max=32"`
	AccountName               string `json:"account_name" binding:"required,max=64"`
	Enabled                   *bool  `json:"enabled"`
	Testnet                   *bool  `json:"testnet"`
	APIKey                    string `json:"api_key"`                    // CEX
	SecretKey                 string `json:"secret_key"`                 // CEX
	Passphrase                string `json:"passphrase"`                 // okx/gate/kucoin
	HyperliquidWalletAddr     string `json:"hyperliquid_wallet_addr"`    // hyperliquid
	HyperliquidPrivateKey     string `json:"hyperliquid_private_key"`    // hyperliquid 64 hex 私钥种子
	AsterUser                 string `json:"aster_user"`                 // aster
	AsterSigner               string `json:"aster_signer"`               // aster
	AsterPrivateKey           string `json:"aster_private_key"`          // aster
	LighterWalletAddr         string `json:"lighter_wallet_addr"`        // lighter
	LighterPrivateKey         string `json:"lighter_private_key"`        // lighter
	LighterAPIKeyPrivateKey   string `json:"lighter_api_key_private_key"` // lighter
	LighterAPIKeyIndex        int    `json:"lighter_api_key_index"`
}

// UpdateExchangeRequest 部分更新（Partial<CreateExchangeRequest>）。
type UpdateExchangeRequest struct {
	ExchangeType              *string `json:"exchange_type"`
	AccountName               *string `json:"account_name"`
	Enabled                   *bool   `json:"enabled"`
	Testnet                   *bool   `json:"testnet"`
	APIKey                    *string `json:"api_key"`
	SecretKey                 *string `json:"secret_key"`
	Passphrase                *string `json:"passphrase"`
	HyperliquidWalletAddr     *string `json:"hyperliquid_wallet_addr"`
	HyperliquidPrivateKey     *string `json:"hyperliquid_private_key"`
	AsterUser                 *string `json:"aster_user"`
	AsterSigner               *string `json:"aster_signer"`
	AsterPrivateKey           *string `json:"aster_private_key"`
	LighterWalletAddr         *string `json:"lighter_wallet_addr"`
	LighterPrivateKey         *string `json:"lighter_private_key"`
	LighterAPIKeyPrivateKey   *string `json:"lighter_api_key_private_key"`
	LighterAPIKeyIndex        *int    `json:"lighter_api_key_index"`
}

// ExchangeDTO 交易所响应（凭据脱敏：仅返回前缀掩码与公开字段）。
type ExchangeDTO struct {
	ID                        string    `json:"id"`
	ExchangeType              string    `json:"exchange_type"`
	AccountName               string    `json:"account_name"`
	Enabled                   bool      `json:"enabled"`
	Testnet                   bool      `json:"testnet,omitempty"`
	APIKeyPrefix              string    `json:"api_key_prefix,omitempty"` // 脱敏，如 ab****cd
	HyperliquidWalletAddr     string    `json:"hyperliquid_wallet_addr,omitempty"`
	AsterUser                 string    `json:"aster_user,omitempty"`
	AsterSigner               string    `json:"aster_signer,omitempty"`
	LighterWalletAddr         string    `json:"lighter_wallet_addr,omitempty"`
	LighterAPIKeyIndex        int       `json:"lighter_api_key_index,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// ExchangeBalanceDTO 交易所账户余额（调各所账户 API 实时查询；失败降级 balance=null + error）。
type ExchangeBalanceDTO struct {
	ExchangeType string  `json:"exchange_type"`
	AccountName  string  `json:"account_name"`
	Balance      *float64 `json:"balance"` // null = 查询失败
	Currency     string  `json:"currency"`
	Error        string  `json:"error,omitempty"`
}
