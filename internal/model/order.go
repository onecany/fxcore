package model

import "time"

// Order 订单实体（API设计.md §15 orders 表）。
// ExchangeOrderID / ExchangeTradeID 唯一约束是交易所侧幂等的基石：
// OrderSync 拉取 24h 成交去重时依赖该约束防重复落库。
type Order struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	TraderID        string    `gorm:"size:36;index" json:"trader_id"`
	ExchangeID      string    `gorm:"size:36;index" json:"exchange_id"`
	ExchangeOrderID string    `gorm:"uniqueIndex;size:128" json:"exchange_order_id,omitempty"`
	ClientOrderID   string    `gorm:"size:128" json:"client_order_id,omitempty"` // 幂等键：traderID+cycle+symbol+action
	Symbol          string    `gorm:"size:32;index" json:"symbol"`
	Side            string    `gorm:"size:8" json:"side"`             // buy | sell
	PositionSide    string    `gorm:"size:8" json:"position_side,omitempty"` // long | short（期货双向）
	Type            string    `gorm:"size:16" json:"type"`            // limit | market | conditional
	TimeInForce     string    `gorm:"size:16" json:"time_in_force,omitempty"`
	Quantity        float64   `json:"quantity"`
	Price           float64   `json:"price,omitempty"`
	StopPrice       float64   `json:"stop_price,omitempty"`
	Status          string    `gorm:"size:16;index" json:"status"` // NEW | PARTIALLY_FILLED | FILLED | CANCELED | REJECTED
	FilledQuantity  float64   `json:"filled_quantity,omitempty"`
	AvgFillPrice    float64   `json:"avg_fill_price,omitempty"`
	Commission      float64   `json:"commission,omitempty"`
	CommissionAsset string    `gorm:"size:16" json:"commission_asset,omitempty"`
	Leverage        int       `json:"leverage,omitempty"`
	ReduceOnly      bool      `json:"reduce_only,omitempty"`
	ClosePosition   bool      `json:"close_position,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
