package dto

import "time"

// ========== 辩论模块（API设计.md §10/§11 debate 路由） ==========

// DebateStatus 辩论状态。
const (
	DebatePending    = "pending"
	DebateRunning    = "running"
	DebateVoting     = "voting"
	DebateCompleted  = "completed"
	DebateCancelled  = "cancelled"
)

// DebatePersonality 固定 5 人格（GET /debates/personalities）。
const (
	PersonalityBull        = "bull"
	PersonalityBear        = "bear"
	PersonalityAnalyst     = "analyst"
	PersonalityContrarian  = "contrarian"
	PersonalityRiskManager = "risk_manager"
)

// Personality 人格描述项。
type Personality struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// CreateDebateRequest 创建辩论（participants≥2，max_rounds≤5，§11）。
type CreateDebateRequest struct {
	Name        string   `json:"name" binding:"required,max=64"`
	StrategyID  string   `json:"strategy_id" binding:"required,max=36"`
	Symbol      string   `json:"symbol" binding:"required,max=32"`
	Participants []string `json:"participants" binding:"required,min=2,max=5"` // AI model IDs
	MaxRounds   int      `json:"max_rounds" binding:"min=1,max=5"`
	IntervalMinutes int   `json:"interval_minutes" binding:"omitempty,min=1"` // 缺省 5（引擎兜底）
	PromptVariant string `json:"prompt_variant,omitempty"`
	AutoExecute  bool     `json:"auto_execute,omitempty"`
	TraderID     string   `json:"trader_id,omitempty"`
}

// DebateExecuteRequest 共识执行（仅 open_long/open_short，§11）。
type DebateExecuteRequest struct {
	TraderID string `json:"trader_id" binding:"required,max=36"`
}

// DebateSessionDTO 辩论会话（§10 DebateSession）。
type DebateSessionDTO struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	StrategyID      string    `json:"strategy_id"`
	Status          string    `json:"status"`
	Symbol          string    `json:"symbol"`
	MaxRounds       int       `json:"max_rounds"`
	CurrentRound    int       `json:"current_round"`
	IntervalMinutes int       `json:"interval_minutes"`
	PromptVariant   string    `json:"prompt_variant,omitempty"`
	AutoExecute     bool      `json:"auto_execute,omitempty"`
	TraderID        string    `json:"trader_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DebateParticipantDTO 参与者（§10 DebateParticipant）。
type DebateParticipantDTO struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	AIModelID   string `json:"ai_model_id"`
	AIModelName string `json:"ai_model_name"`
	Provider    string `json:"provider"`
	Personality string `json:"personality"`
	Color       string `json:"color"`
	SpeakOrder  int    `json:"speak_order"`
}

// DebateMessageDTO 辩论消息（SSE message 事件载荷）。
type DebateMessageDTO struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Round     int    `json:"round"`
	ParticipantID string `json:"participant_id,omitempty"`
	Personality  string `json:"personality,omitempty"`
	AIModelName  string `json:"ai_model_name,omitempty"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// DebateVoteDTO 投票（§10 DebateVote）。
type DebateVoteDTO struct {
	SessionID     string  `json:"session_id"`
	AIModelID     string  `json:"ai_model_id"`
	Action        string  `json:"action"` // DecisionActionType
	Symbol        string  `json:"symbol"`
	Confidence    float64 `json:"confidence"`
	Leverage      int     `json:"leverage"`
	PositionPct   float64 `json:"position_pct"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
	Reasoning     string  `json:"reasoning,omitempty"`
}

// SessionWithDetailsDTO 会话详情（GET /debates/{id}，含消息/投票）。
type SessionWithDetailsDTO struct {
	DebateSessionDTO
	Participants []DebateParticipantDTO `json:"participants"`
	Messages     []DebateMessageDTO     `json:"messages"`
	Votes        []DebateVoteDTO        `json:"votes"`
}
