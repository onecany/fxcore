package model

import (
	"encoding/json"
	"time"
)

// BacktestRun 回测运行实体（API设计.md §15 backtest_runs 表）。
// RunID 为唯一键（随机后缀）；状态机见 dto.BacktestState。
type BacktestRun struct {
	RunID           string    `gorm:"primaryKey;size:128" json:"run_id"`
	UserID          string    `gorm:"size:36;index" json:"user_id"`
	ConfigJSON      string    `gorm:"type:text" json:"-"` // BacktestConfig JSON
	State           string    `gorm:"size:16;index" json:"state"`
	Label           string    `gorm:"size:128" json:"label,omitempty"`
	SymbolCount     int       `json:"symbol_count,omitempty"`
	ProgressPct     float64   `json:"progress_pct,omitempty"`
	EquityLast      float64   `json:"equity_last,omitempty"`
	MaxDrawdownPct  float64   `json:"max_drawdown_pct,omitempty"`
	Liquidated      bool      `json:"liquidated,omitempty"`
	PromptTemplate  string    `gorm:"type:text" json:"-"` // 回放时用的提示词模板
	AIProvider      string    `gorm:"size:32" json:"ai_provider,omitempty"`
	AIModel         string    `gorm:"size:128" json:"ai_model,omitempty"`
	LastError       string    `gorm:"size:512" json:"last_error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// BacktestEquity 权益点（§15 backtest_equities）。
type BacktestEquity struct {
	ID        string  `gorm:"primaryKey;size:36" json:"-"`
	RunID     string  `gorm:"size:128;index" json:"run_id"`
	Timestamp int64   `gorm:"index" json:"timestamp"`
	Equity    float64 `json:"equity"`
	Available float64 `json:"available,omitempty"`
	PnL       float64 `json:"pnl,omitempty"`
	PnLPct    float64 `json:"pnl_pct,omitempty"`
	DrawdownPct float64 `json:"drawdown_pct,omitempty"`
	Cycle     int64   `json:"cycle,omitempty"`
}

// BacktestTrade 回测成交（§15 backtest_trades）。
type BacktestTrade struct {
	ID          string  `gorm:"primaryKey;size:36" json:"-"`
	RunID       string  `gorm:"size:128;index" json:"run_id"`
	Timestamp   int64   `gorm:"index" json:"timestamp"`
	Symbol      string  `gorm:"size:32" json:"symbol"`
	Action      string  `gorm:"size:16" json:"action"`
	Side        string  `gorm:"size:8" json:"side,omitempty"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price"`
	Fee         float64 `json:"fee,omitempty"`
	Slippage    float64 `json:"slippage,omitempty"`
	OrderValue  float64 `json:"order_value,omitempty"`
	RealizedPnL float64 `json:"realized_pnl,omitempty"`
	Leverage    int     `json:"leverage,omitempty"`
	Cycle       int64   `json:"cycle,omitempty"`
	PositionAfter float64 `json:"position_after,omitempty"`
	Liquidation bool    `json:"liquidation,omitempty"`
	Note        string  `gorm:"size:256" json:"note,omitempty"`
}

// BacktestDecision 回测决策（§15 backtest_decisions）。
type BacktestDecision struct {
	ID        string          `gorm:"primaryKey;size:36" json:"-"`
	RunID     string          `gorm:"size:128;index" json:"run_id"`
	Cycle     int64           `gorm:"index" json:"cycle"`
	Payload   json.RawMessage `gorm:"type:text" json:"payload"` // DecisionRecord JSON
}

// BacktestCheckpoint 回测检查点（§15 backtest_checkpoints，断点续跑）。
type BacktestCheckpoint struct {
	RunID   string          `gorm:"primaryKey;size:128" json:"run_id"`
	Payload json.RawMessage `gorm:"type:text" json:"payload"` // 持仓/余额快照 JSON
}
