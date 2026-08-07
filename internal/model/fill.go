package model

import "time"

// Fill 成交记录实体（API设计.md §15 fills 表）。
// ExchangeTradeID 唯一约束配合 OrderSync 去重；IsMaker 按交易所口径
// （okx ExecType==M / bybit 字段 / hyperliquid 恒 false）由适配器填充。
type Fill struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	TraderID        string    `gorm:"size:36;index" json:"trader_id"`
	OrderID         string    `gorm:"size:36;index" json:"order_id"`
	ExchangeOrderID string    `gorm:"size:128" json:"exchange_order_id,omitempty"`
	ExchangeTradeID string    `gorm:"uniqueIndex;size:128" json:"exchange_trade_id,omitempty"`
	Symbol          string    `gorm:"size:32;index" json:"symbol"`
	Side            string    `gorm:"size:8" json:"side"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	QuoteQuantity   float64   `json:"quote_quantity,omitempty"`
	Commission      float64   `json:"commission,omitempty"`
	CommissionAsset string    `gorm:"size:16" json:"commission_asset,omitempty"`
	RealizedPnL     float64   `json:"realized_pnl,omitempty"`
	IsMaker         bool      `json:"is_maker,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
