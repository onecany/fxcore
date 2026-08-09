package model

import (
	"encoding/json"
	"time"
)

// Trader 交易员实体（状态机：idle→running→paused→running→stopped，见 trader_svc.go）。
// 嵌套结构体（ModelConfig/RiskConfig/Schedule/Metrics）用 GORM serializer:json 落库。
type Trader struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	Name        string     `gorm:"size:64;index" json:"name"`
	Exchange    string     `gorm:"size:32;index" json:"exchange"`
	ModelConfig ModelConfig `gorm:"serializer:json;type:text" json:"model_config"`
	StrategyID  string     `gorm:"size:36;index" json:"strategy_id"`
	RiskConfig  RiskConfig `gorm:"serializer:json;type:text" json:"risk_config"`
	Schedule    *Schedule  `gorm:"serializer:json;type:text" json:"schedule,omitempty"`
	Status      string     `gorm:"size:16;index" json:"status"`
	Metrics     Metrics    `gorm:"serializer:json;type:text" json:"metrics"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ModelConfig 引用已保存的 AI 模型（model_id 对应 model.AIModel.ID）。
type ModelConfig struct {
	Provider   string          `json:"provider"`
	ModelID    string          `json:"model_id"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// RiskConfig 风控参数。
type RiskConfig struct {
	MaxPositionSize float64 `json:"max_position_size"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	MaxDailyLoss    float64 `json:"max_daily_loss"`
}

// Schedule 定时调度。
type Schedule struct {
	Interval    int           `json:"interval"`
	ActiveHours []ActiveHours `json:"active_hours"`
}

// ActiveHours 活跃时段。
type ActiveHours struct {
	Start string `json:"start"` // "HH:MM"
	End   string `json:"end"`   // "HH:MM"
}

// Metrics 交易员绩效指标。
type Metrics struct {
	TotalPnL   float64 `json:"total_pnl"`
	WinRate    float64 `json:"win_rate"`
	TradeCount int64   `json:"trade_count"`
	DailyPnL   float64 `json:"daily_pnl"`
}

// TraderStatus 常量。
const (
	StatusIdle    = "idle"
	StatusRunning = "running"
	StatusPaused  = "paused"
	StatusStopped = "stopped"
	StatusError   = "error"
)

// Position 持仓/平仓记录（API设计.md §15 positions 表）。
// 保留早期字段（Size/PnL/OpenedAt/ClosedAt）兼容现有 dashboard/trader_svc 逻辑，
// 新增 §10/§15 契约字段：quantity/leverage/status/entry_time/exit_time/
// realized_pnl/fee/close_reason 等。PositionBuilder 是 position 表唯一写者。
type Position struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	TraderID      string     `gorm:"size:36;index" json:"trader_id"`
	ExchangeID    string     `gorm:"size:36;index" json:"exchange_id,omitempty"`
	Symbol        string     `gorm:"size:32;index" json:"symbol"`
	Side          string     `gorm:"size:8" json:"side"` // long | short
	Size          float64    `json:"size"` // 兼容旧字段（=quantity）
	EntryPrice    float64    `json:"entry_price"`
	PnL           float64    `json:"pnl"` // 兼容旧字段（平仓后=realized_pnl）
	OpenedAt      time.Time  `json:"opened_at"`
	ClosedAt      *time.Time `gorm:"index" json:"closed_at,omitempty"`
	// ---- §10/§15 扩展字段 ----
	EntryQuantity float64    `json:"entry_quantity,omitempty"` // 开仓数量
	Quantity      float64    `json:"quantity,omitempty"`       // 当前数量（减仓后变小）
	MarkPrice     float64    `json:"mark_price,omitempty"`
	UnrealizedPnL float64    `json:"unrealized_pnl,omitempty"`
	Leverage      int        `json:"leverage,omitempty"`
	Status        string     `json:"status,omitempty"` // OPEN | CLOSED
	EntryTime     int64      `json:"entry_time,omitempty"` // unix 秒（§10 契约）
	ExitTime      int64      `json:"exit_time,omitempty"`
	ExitPrice     float64    `json:"exit_price,omitempty"`
	RealizedPnL   float64    `json:"realized_pnl,omitempty"`
	Fee           float64    `json:"fee,omitempty"`
	CloseReason   string     `json:"close_reason,omitempty"` // tp | sl | manual | risk | liquidated
	Source        string     `json:"source,omitempty"`       // engine | manual
}

// Position 状态常量（OPEN/CLOSED，区别于 Trader 状态机）。
const (
	PositionOpen   = "OPEN"
	PositionClosed = "CLOSED"
)
