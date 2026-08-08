package model

import (
	"encoding/json"
	"time"
)

// DebateSession 辩论会话实体（API设计.md §15 debate_sessions）。
// 状态机：pending→running→voting→completed | cancelled（§10 DebateStatus）。
type DebateSession struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	Name            string    `gorm:"size:64" json:"name"`
	StrategyID      string    `gorm:"size:36;index" json:"strategy_id"`
	Status          string    `gorm:"size:16;index" json:"status"`
	Symbol          string    `gorm:"size:32" json:"symbol"`
	MaxRounds       int       `json:"max_rounds"`
	CurrentRound    int       `json:"current_round"`
	IntervalMinutes int       `json:"interval_minutes"`
	PromptVariant   string    `gorm:"size:32" json:"prompt_variant,omitempty"`
	AutoExecute     bool      `json:"auto_execute,omitempty"`
	TraderID        string    `gorm:"size:36" json:"trader_id,omitempty"`
	Consensus       string    `gorm:"type:text" json:"consensus,omitempty"` // 共识投票 JSON
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DebateParticipant 辩论参与者（§15 debate_participants）。
type DebateParticipant struct {
	ID          string `gorm:"primaryKey;size:36" json:"id"`
	SessionID   string `gorm:"size:36;index" json:"session_id"`
	AIModelID   string `gorm:"size:36" json:"ai_model_id"`
	AIModelName string `gorm:"size:128" json:"ai_model_name"`
	Provider    string `gorm:"size:32" json:"provider"`
	Personality string `gorm:"size:16" json:"personality"`
	Color       string `gorm:"size:16" json:"color"`
	SpeakOrder  int    `json:"speak_order"`
}

// DebateMessage 辩论消息（§15 debate_messages）。
type DebateMessage struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	SessionID     string    `gorm:"size:36;index" json:"session_id"`
	Round         int       `json:"round"`
	ParticipantID string    `gorm:"size:36" json:"participant_id,omitempty"`
	Personality   string    `gorm:"size:16" json:"personality,omitempty"`
	AIModelName   string    `gorm:"size:128" json:"ai_model_name,omitempty"`
	Content       string    `gorm:"type:text" json:"content"`
	Timestamp     int64     `json:"timestamp"`
	CreatedAt     time.Time `json:"created_at"`
}

// DebateVote 辩论投票（§15 debate_votes）。
type DebateVote struct {
	ID            string          `gorm:"primaryKey;size:36" json:"id"`
	SessionID     string          `gorm:"size:36;index" json:"session_id"`
	AIModelID     string          `gorm:"size:36" json:"ai_model_id"`
	Action        string          `gorm:"size:16" json:"action"` // DecisionActionType
	Symbol        string          `gorm:"size:32" json:"symbol"`
	Confidence    float64         `json:"confidence"`
	Leverage      int             `json:"leverage"`
	PositionPct   float64         `json:"position_pct"`
	StopLossPct   float64         `json:"stop_loss_pct"`
	TakeProfitPct float64         `json:"take_profit_pct"`
	Reasoning     string          `gorm:"type:text" json:"reasoning,omitempty"`
	Extra         json.RawMessage `gorm:"type:text" json:"extra,omitempty"`
}
