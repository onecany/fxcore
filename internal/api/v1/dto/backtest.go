package dto

import "time"

// ========== 回测模块（API设计.md §10/§11 backtest 路由） ==========

// BacktestState 回测状态机。
const (
	BacktestCreated   = "created"
	BacktestRunning   = "running"
	BacktestPaused    = "paused"
	BacktestStopped   = "stopped"
	BacktestCompleted = "completed"
	BacktestFailed    = "failed"
	BacktestLiquidated = "liquidated"
)

// FillPolicy 成交策略。
type FillPolicy string

const (
	FillNextOpen FillPolicy = "next_open"
	FillBarVWAP  FillPolicy = "bar_vwap"
	FillMid      FillPolicy = "mid"
)

// BacktestConfig 回测配置（§10 BacktestConfig）。
type BacktestConfig struct {
	StrategyID    string     `json:"strategy_id,omitempty"`
	Symbols       []string   `json:"symbols,omitempty"`
	Timeframes    []string   `json:"timeframes,omitempty"`
	Cadence       int        `json:"cadence,omitempty"` // 扫描间隔（分钟）
	InitialBalance float64   `json:"initial_balance,omitempty"`
	Leverage      int        `json:"leverage,omitempty"`
	PromptVariant string     `json:"prompt_variant,omitempty"`
	PromptTemplate string    `json:"prompt_template,omitempty"`
	Temperature   float64    `json:"temperature,omitempty"`
	FeeBPS        float64    `json:"fee_bps,omitempty"`
	SlippageBPS   float64    `json:"slippage_bps,omitempty"`
	FillPolicy    FillPolicy `json:"fill_policy,omitempty"`
	StartTime     int64      `json:"start_time,omitempty"` // unix 秒
	EndTime       int64      `json:"end_time,omitempty"`
	AICache       bool       `json:"ai_cache,omitempty"`
	ReplayOnly    bool       `json:"replay_only,omitempty"`
}

// StartBacktestRequest 启动回测（POST /backtest/start）。
type StartBacktestRequest struct {
	Config BacktestConfig `json:"config" binding:"required"`
}

// BacktestControlRequest 暂停/恢复/停止/标签/删除（POST /backtest/{action}）。
type BacktestControlRequest struct {
	RunID string `json:"run_id" binding:"required,max=128"`
	Label string `json:"label,omitempty"` // 仅 label 用
}

// RunMetadata 回测运行元信息（§11 响应）。
type RunMetadata struct {
	RunID         string  `json:"run_id"` // 唯一（随机后缀）
	State         string  `json:"state"`
	Label         string  `json:"label,omitempty"`
	SymbolCount   int     `json:"symbol_count,omitempty"`
	ProgressPct   float64 `json:"progress_pct,omitempty"`
	EquityLast    float64 `json:"equity_last,omitempty"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct,omitempty"`
	Liquidated    bool    `json:"liquidated,omitempty"`
	LastError     string  `json:"last_error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// RunSummary 运行摘要（GET /backtest/runs 分页项）。
type RunSummary struct {
	RunMetadata
	Symbols  []string `json:"symbols,omitempty"`
	Timeframe string  `json:"timeframe,omitempty"`
}

// BacktestStatusPayload 状态载荷（GET /backtest/status）。
type BacktestStatusPayload struct {
	RunMetadata
	CurrentCycle int64 `json:"current_cycle,omitempty"`
	TotalCycles  int64 `json:"total_cycles,omitempty"`
}

// BacktestEquityPoint 权益点（GET /backtest/equity，§10 BacktestEquityPoint）。
type BacktestEquityPoint struct {
	Timestamp   int64   `json:"timestamp"`
	Equity      float64 `json:"equity"`
	Available   float64 `json:"available,omitempty"`
	PnL         float64 `json:"pnl,omitempty"`
	PnLPct      float64 `json:"pnl_pct,omitempty"`
	DrawdownPct float64 `json:"drawdown_pct,omitempty"`
	Cycle       int64   `json:"cycle,omitempty"`
}

// BacktestTradeEvent 回测成交事件（GET /backtest/trades，§10 BacktestTradeEvent）。
type BacktestTradeEvent struct {
	Timestamp      int64   `json:"timestamp"`
	Symbol         string  `json:"symbol"`
	Action         string  `json:"action"` // open_long | open_short | close_long | close_short | ...
	Side           string  `json:"side,omitempty"`
	Quantity       float64 `json:"quantity"`
	Price          float64 `json:"price"`
	Fee            float64 `json:"fee,omitempty"`
	Slippage       float64 `json:"slippage,omitempty"`
	OrderValue     float64 `json:"order_value,omitempty"`
	RealizedPnL    float64 `json:"realized_pnl,omitempty"`
	Leverage       int     `json:"leverage,omitempty"`
	Cycle          int64   `json:"cycle,omitempty"`
	PositionAfter  float64 `json:"position_after,omitempty"`
	LiquidationFlag bool   `json:"liquidation_flag,omitempty"`
	Note           string  `json:"note,omitempty"`
}

// BacktestMetrics 回测指标（GET /backtest/metrics，未就绪返回 202）。
type BacktestMetrics struct {
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate"`
	TotalPnL       float64 `json:"total_pnl"`
	ProfitFactor   float64 `json:"profit_factor,omitempty"`
	SharpeRatio    float64 `json:"sharpe_ratio,omitempty"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct,omitempty"`
	AvgWin         float64 `json:"avg_win,omitempty"`
	AvgLoss        float64 `json:"avg_loss,omitempty"`
	Liquidated     bool    `json:"liquidated,omitempty"`
	FinalEquity    float64 `json:"final_equity,omitempty"`
	ReturnPct      float64 `json:"return_pct,omitempty"`
}
