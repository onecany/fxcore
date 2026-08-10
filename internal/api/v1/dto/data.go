package dto

import "time"

// ========== 数据模块（API设计.md §11 数据路由） ==========

// StatusDTO 交易员运行状态（GET /status）。
type StatusDTO struct {
	IsRunning bool   `json:"is_running"`
	TraderID  string `json:"trader_id"`
}

// AccountInfoDTO 账户信息（GET /account）。
type AccountInfoDTO struct {
	TotalEquity    float64 `json:"total_equity"`
	AvailableBalance float64 `json:"available_balance"`
	UnrealizedPnL  float64 `json:"unrealized_pnl,omitempty"`
	PositionCount  int     `json:"position_count,omitempty"`
	MarginUsedPct  float64 `json:"margin_used_pct,omitempty"`
	Currency       string  `json:"currency"` // USDT
}

// StatisticsDTO 交易统计（GET /statistics，§10 Statistics）。
type StatisticsDTO struct {
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate"` // 0~1
	TotalPnL       float64 `json:"total_pnl"`
	ProfitFactor   float64 `json:"profit_factor,omitempty"`
	SharpeRatio    float64 `json:"sharpe_ratio,omitempty"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct,omitempty"`
	AvgWin         float64 `json:"avg_win,omitempty"`
	AvgLoss        float64 `json:"avg_loss,omitempty"`
}

// TradeEventDTO 成交事件（GET /trades）。
type TradeEventDTO struct {
	ID          string    `json:"id"`
	TraderID    string    `json:"trader_id"`
	OrderID     string    `json:"order_id,omitempty"`
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"` // buy | sell
	Quantity    float64   `json:"quantity"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee,omitempty"`
	FeeAsset    string    `json:"fee_asset,omitempty"`
	RealizedPnL float64   `json:"realized_pnl,omitempty"`
	IsMaker     bool      `json:"is_maker,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// OrderDTO 订单（GET /orders，§15 orders 表形状）。
type OrderDTO struct {
	ID              string    `json:"id"`
	TraderID        string    `json:"trader_id"`
	ExchangeID      string    `json:"exchange_id,omitempty"`
	ExchangeOrderID string    `json:"exchange_order_id,omitempty"`
	ClientOrderID   string    `json:"client_order_id,omitempty"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	PositionSide    string    `json:"position_side,omitempty"`
	Type            string    `json:"type"`
	TimeInForce     string    `json:"time_in_force,omitempty"`
	Quantity        float64   `json:"quantity"`
	Price           float64   `json:"price,omitempty"`
	StopPrice       float64   `json:"stop_price,omitempty"`
	Status          string    `json:"status"`
	FilledQuantity  float64   `json:"filled_quantity,omitempty"`
	AvgFillPrice    float64   `json:"avg_fill_price,omitempty"`
	Commission      float64   `json:"commission,omitempty"`
	CommissionAsset string    `json:"commission_asset,omitempty"`
	Leverage        int       `json:"leverage,omitempty"`
	ReduceOnly      bool      `json:"reduce_only,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// FillDTO 成交明细（GET /orders/{id}/fills，§15 fills 表形状）。
type FillDTO struct {
	ID              string    `json:"id"`
	TraderID        string    `json:"trader_id"`
	OrderID         string    `json:"order_id"`
	ExchangeTradeID string    `json:"exchange_trade_id,omitempty"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	QuoteQuantity   float64   `json:"quote_quantity,omitempty"`
	Commission      float64   `json:"commission,omitempty"`
	CommissionAsset string    `json:"commission_asset,omitempty"`
	RealizedPnL     float64   `json:"realized_pnl,omitempty"`
	IsMaker         bool      `json:"is_maker,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// KlineDTO 行情 K 线（GET /klines，数据源链统一 OHLCV）。
type KlineDTO struct {
	Timestamp int64   `json:"timestamp"` // unix 秒
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	OI        float64 `json:"oi,omitempty"`        // 可选：未平仓量
	Funding   float64 `json:"funding,omitempty"`   // 可选：资金费率
}

// EquityPointDTO 权益点（GET /equity-history，§10 BacktestEquityPoint 同构）。
type EquityPointDTO struct {
	Timestamp      int64   `json:"timestamp"` // unix 秒
	Equity         float64 `json:"equity"`
	Balance        float64 `json:"balance,omitempty"`
	PnL            float64 `json:"pnl,omitempty"`
	PnLPct         float64 `json:"pnl_pct,omitempty"`
	DrawdownPct    float64 `json:"drawdown_pct,omitempty"`
	MarginUsedPct  float64 `json:"margin_used_pct,omitempty"`  // 保证金占用（引擎实时）
	PositionCount  int     `json:"position_count,omitempty"`
}

// EquityBatchRequest 批量权益查询（POST /equity-history-batch）。
type EquityBatchRequest struct {
	TraderIDs []string `json:"trader_ids" binding:"required,min=1,max=50"`
}

// TraderConfigDTO 脱敏公开配置（GET /traders/{id}/public-config）。
type TraderConfigDTO struct {
	Name           string `json:"name"`
	Exchange       string `json:"exchange"`
	StrategyID     string `json:"strategy_id,omitempty"`
	ScanIntervalMinutes int `json:"scan_interval_minutes,omitempty"`
	IsCrossMargin  bool   `json:"is_cross_margin,omitempty"`
	BTCEthLeverage int    `json:"btc_eth_leverage,omitempty"`
	AltcoinLeverage int   `json:"altcoin_leverage,omitempty"`
	ShowInCompetition bool `json:"show_in_competition,omitempty"`
}
