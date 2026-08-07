package model

import "time"

// EquitySnapshot 权益快照实体（API设计.md §15 trader_equity_snapshots 表）。
// 交易循环每周期先存权益（与 AI 无关，AI 失败也有快照），
// 供 equity-history 曲线与回撤计算。
type EquitySnapshot struct {
	ID             string    `gorm:"primaryKey;size:36" json:"id"`
	TraderID       string    `gorm:"size:36;index" json:"trader_id"`
	Timestamp      time.Time `gorm:"index" json:"timestamp"`
	TotalEquity    float64   `json:"total_equity"`
	Balance        float64   `json:"balance"`
	UnrealizedPnL  float64   `json:"unrealized_pnl,omitempty"`
	PositionCount  int       `json:"position_count,omitempty"`
	MarginUsedPct  float64   `json:"margin_used_pct,omitempty"`
}
