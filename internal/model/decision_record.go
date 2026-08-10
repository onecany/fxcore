package model

import (
	"encoding/json"
	"time"
)

// DecisionActionType 决策动作六值（API设计.md §16 输出契约）。
const (
	ActionOpenLong   = "open_long"
	ActionOpenShort  = "open_short"
	ActionCloseLong  = "close_long"
	ActionCloseShort = "close_short"
	ActionHold       = "hold"
	ActionWait       = "wait"
)

// DecisionAction 单条决策动作（§10 DecisionAction）。
type DecisionAction struct {
	Action      string  `json:"action"`
	Symbol      string  `json:"symbol"`
	Quantity    float64 `json:"quantity,omitempty"`
	Leverage    float64 `json:"leverage,omitempty"`
	Price       float64 `json:"price,omitempty"`
	StopLoss    float64 `json:"stop_loss,omitempty"`
	TakeProfit  float64 `json:"take_profit,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
	Reasoning   string  `json:"reasoning,omitempty"`
	OrderID     string  `json:"order_id,omitempty"`
	Success     *bool   `json:"success,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// DecisionRecord 决策记录实体（API设计.md §15 decision_records 表）。
// 每交易员每周期一条：完整输入输出留痕（system/input prompt、cot、原始响应、
// 解析后的决策数组、执行结果），供回测 trace 与审计。
type DecisionRecord struct {
	ID                  string            `gorm:"primaryKey;size:36" json:"id"`
	TraderID            string            `gorm:"size:36;index" json:"trader_id"`
	CycleNumber         int64             `gorm:"index" json:"cycle_number"`
	Timestamp           time.Time         `gorm:"index" json:"timestamp"`
	SystemPrompt        string            `gorm:"type:text" json:"system_prompt,omitempty"`
	InputPrompt         string            `gorm:"type:text" json:"input_prompt,omitempty"`
	CotTrace            string            `gorm:"type:text" json:"cot_trace,omitempty"`
	DecisionJSON        string            `gorm:"type:text" json:"decision_json,omitempty"`
	RawResponse         string            `gorm:"type:text" json:"raw_response,omitempty"`
	CandidateCoins      json.RawMessage   `gorm:"type:text;serializer:json" json:"candidate_coins,omitempty"`
	Decisions           []DecisionAction  `gorm:"-" json:"decisions"`
	ExecutionLog        json.RawMessage   `gorm:"type:text;serializer:json" json:"execution_log,omitempty"`
	Success             bool              `json:"success"`
	ErrorMessage        string            `gorm:"size:512" json:"error_message,omitempty"`
	AIRequestDurationMS int64             `json:"ai_request_duration_ms,omitempty"`
}
