package model

import (
	"encoding/json"
	"time"
)

// Trader 交易员实体（状态机：idle→running→paused→running→stopped，见 trader_svc.go）。
type Trader struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Exchange    string          `json:"exchange"`
	ModelConfig ModelConfig     `json:"model_config"`
	StrategyID  string          `json:"strategy_id"`
	RiskConfig  RiskConfig      `json:"risk_config"`
	Schedule    *Schedule       `json:"schedule,omitempty"`
	Status      string          `json:"status"`
	Metrics     Metrics         `json:"metrics"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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

// Position 持仓/平仓记录。
type Position struct {
	ID         string     `json:"id"`
	Symbol     string     `json:"symbol"`
	Side       string     `json:"side"` // long | short
	Size       float64    `json:"size"`
	EntryPrice float64    `json:"entry_price"`
	PnL        float64    `json:"pnl"`
	TraderID   string     `json:"trader_id"`
	OpenedAt   time.Time  `json:"opened_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
}
