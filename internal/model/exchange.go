package model

import "time"

// ExchangeType 支持的交易所类型（API设计.md §10 扩展 10 家）。
// CEX: binance/bybit/okx/bitget/gate/kucoin/indodax；DEX: hyperliquid/aster/lighter。
const (
	ExchangeBinance    = "binance"
	ExchangeBybit      = "bybit"
	ExchangeOKX        = "okx"
	ExchangeBitget     = "bitget"
	ExchangeGate       = "gate"
	ExchangeKuCoin     = "kucoin"
	ExchangeIndodax    = "indodax"
	ExchangeHyperliquid = "hyperliquid"
	ExchangeAster      = "aster"
	ExchangeLighter    = "lighter"
)

// Exchange 交易所账户实体（API设计.md §15 exchanges 表）。
// 凭据列一律 RSA-OAEP 加密后落库（ENC:v1: 前缀），json:"-" 保证永不出现在响应。
// 各交易所必填字段：
//   CEX: api_key + secret_key（okx/gate/kucoin 额外 passphrase）
//   hyperliquid: hyperliquid_wallet_addr
//   aster: aster_user + aster_signer + aster_private_key
//   lighter: lighter_wallet_addr + lighter_private_key + lighter_api_key_private_key + lighter_api_key_index
type Exchange struct {
	ID                        string     `gorm:"primaryKey;size:36" json:"id"`
	UserID                    string     `gorm:"size:36;index" json:"user_id"`
	ExchangeType              string     `gorm:"size:32;index" json:"exchange_type"`
	AccountName               string     `gorm:"size:64" json:"account_name"`
	Enabled                   bool       `json:"enabled"`
	Testnet                   bool       `json:"testnet,omitempty"`
	APIKeyEnc                 string     `gorm:"size:4096" json:"-"`
	SecretKeyEnc              string     `gorm:"size:4096" json:"-"`
	PassphraseEnc             string     `gorm:"size:4096" json:"-"`
	HyperliquidWalletAddr     string     `gorm:"size:128" json:"hyperliquid_wallet_addr,omitempty"`
	AsterUser                 string     `gorm:"size:128" json:"aster_user,omitempty"`
	AsterSigner               string     `gorm:"size:128" json:"aster_signer,omitempty"`
	AsterPrivateKeyEnc        string     `gorm:"size:4096" json:"-"`
	LighterWalletAddr         string     `gorm:"size:128" json:"lighter_wallet_addr,omitempty"`
	LighterPrivateKeyEnc      string     `gorm:"size:4096" json:"-"`
	LighterAPIKeyPrivateKeyEnc string    `gorm:"size:4096" json:"-"`
	LighterAPIKeyIndex        int        `json:"lighter_api_key_index,omitempty"`
	DeletedAt                 *time.Time `gorm:"index" json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}
