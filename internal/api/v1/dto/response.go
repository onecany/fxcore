package dto

import (
	"encoding/json"
	"time"
)

// ========== 通用响应信封 ==========

// ApiResponse 统一响应信封（与 contract.ts ApiResponse 对齐）。
type ApiResponse[T any] struct {
	Code      int    `json:"code"`      // 业务码，见 internal/middleware/error.go 矩阵
	Message   string `json:"message"`   // 人类可读消息
	Data      T      `json:"data"`      // 业务数据
	Timestamp int64  `json:"timestamp"` // unix 毫秒
	RequestID string `json:"request_id"`
}

// OK 构造成功响应。
func OK[T any](data T, requestID string) ApiResponse[T] {
	return ApiResponse[T]{Code: 0, Message: "success", Data: data, Timestamp: time.Now().UnixMilli(), RequestID: requestID}
}

// ErrorResponse 统一错误信封（swagger 引用用；实际由 middleware.WriteError 输出）。
type ErrorResponse struct {
	Code      int            `json:"code"`      // 业务错误码（1001-1500 矩阵）
	Message   string         `json:"message"`   // 可读错误信息
	Data      map[string]any `json:"data"`      // 1001 时字段级错误映射，其余为 null
	Timestamp int64          `json:"timestamp"` // Unix 毫秒
	RequestID string         `json:"request_id"`// 链路追踪 ID
}

// PaginatedData 分页响应（列表端点）。
type PaginatedData[T any] struct {
	Items      []T       `json:"items"`
	Pagination Paginator `json:"pagination"`
}

// ========== swagger 引用类型（swag 泛型嵌套解析限制，用具体类型替代） ==========

// AIModelDTOList 模型列表响应。
type AIModelDTOList []AIModelDTO

// ProviderOptionList 提供商列表响应。
type ProviderOptionList []ProviderOption

// PositionDTOList 持仓列表响应。
type PositionDTOList []PositionDTO

// TraderPage 交易员分页响应（结构与 PaginatedData[TraderDTO] 一致）。
type TraderPage struct {
	Items      []TraderDTO `json:"items"`
	Pagination Paginator   `json:"pagination"`
}

// Paginator 分页信息。
type Paginator struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ========== 认证模块 ==========

// UserDTO 用户信息。
type UserDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar,omitempty"`
}

// LoginResponse 登录响应。sign_secret / refresh_token 为扩展字段
// （请求签名 user_secret 与 Refresh 轮换闭环所需，见 data/API设计.md 复盘）。
type LoginResponse struct {
	User         UserDTO `json:"user"`
	AccessToken  string  `json:"access_token"`
	SignSecret   string  `json:"sign_secret,omitempty"`
	RefreshToken string  `json:"refresh_token,omitempty"`
}

// RefreshResponse 刷新响应（含轮换后的新 refresh token 与签名密钥）。
// sign_secret：reload 后用户 secret 丢失，刷新必须重新下发，
// 否则会话"半恢复"（读通写挂，见复盘 S3）。
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	SignSecret   string `json:"sign_secret,omitempty"`
}

// ForgotPasswordResponse 忘记密码响应。
// 防枚举：邮箱不存在时同样返回 200 + 通用 message（无 dev_link）。
// dev_link 仅 SMTP 未配置的 dev 模式返回（无邮件服务器时的联调便利）。
type ForgotPasswordResponse struct {
	Message string `json:"message"`
	DevLink string `json:"dev_link,omitempty"`
}

// ResetPasswordResponse 重置密码响应。
type ResetPasswordResponse struct {
	Message string `json:"message"`
}

// ========== AI 模型模块 ==========

// AIModelDTO 模型列表项（api_key 永不出现在响应中）。
type AIModelDTO struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Provider     string          `json:"provider"`
	ModelName    string          `json:"model_name"`
	APIKeyPrefix string          `json:"api_key_prefix"` // 脱敏，如 sk-****abcd
	Status       string          `json:"status"`         // active | inactive | error
	Config       json.RawMessage `json:"config"`
	CreatedAt    time.Time       `json:"created_at"`
	LastTestAt   *time.Time      `json:"last_test_at,omitempty"`
	RotatedAt    *time.Time      `json:"rotated_at,omitempty"`
}

// TestResult 连通测试结果。
type TestResult struct {
	Success bool   `json:"success"`
	Latency int64  `json:"latency"` // 毫秒
	Error   string `json:"error,omitempty"`
}

// ProviderOption 提供商下拉选项。
type ProviderOption struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
}

// ========== 交易员模块 ==========

// MetricsDTO 交易员绩效指标。
type MetricsDTO struct {
	TotalPnL   float64 `json:"total_pnl"`
	WinRate    float64 `json:"win_rate"` // 0~1
	TradeCount int64   `json:"trade_count"`
	DailyPnL   float64 `json:"daily_pnl"`
}

// TraderDTO 交易员响应（对应 TraderResponse）。
type TraderDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Exchange    string          `json:"exchange"`
	ModelConfig ModelConfig     `json:"model_config"`
	StrategyID  string          `json:"strategy_id"`
	RiskConfig  RiskConfig      `json:"risk_config"`
	Schedule    *Schedule       `json:"schedule,omitempty"`
	Status      string          `json:"status"` // idle | running | paused | stopped | error
	Metrics     MetricsDTO      `json:"metrics"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ========== 持仓模块 ==========

// PositionDTO 持仓/平仓记录（§10/§15 全字段，对齐 model.Position）。
type PositionDTO struct {
	ID             string     `json:"id"`
	TraderID       string     `json:"trader_id"`
	ExchangeID     string     `json:"exchange_id,omitempty"`
	Symbol         string     `json:"symbol"`
	Side           string     `json:"side"` // long | short
	Size           float64    `json:"size"`
	EntryPrice     float64    `json:"entry_price"`
	PnL            float64    `json:"pnl"`
	OpenedAt       time.Time  `json:"opened_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	EntryQuantity  float64    `json:"entry_quantity,omitempty"`
	Quantity       float64    `json:"quantity,omitempty"`
	MarkPrice      float64    `json:"mark_price,omitempty"`
	UnrealizedPnL  float64    `json:"unrealized_pn_l,omitempty"`
	Leverage       int        `json:"leverage,omitempty"`
	Status         string     `json:"status,omitempty"` // OPEN | CLOSED
	EntryTime      int64      `json:"entry_time,omitempty"`
	ExitTime       int64      `json:"exit_time,omitempty"`
	ExitPrice      float64    `json:"exit_price,omitempty"`
	RealizedPnL    float64    `json:"realized_pn_l,omitempty"`
	Fee            float64    `json:"fee,omitempty"`
	CloseReason    string     `json:"close_reason,omitempty"`
	Source         string     `json:"source,omitempty"`
}

// ========== 仪表盘模块 ==========

// AccountSummary 账户汇总。
type AccountSummary struct {
	TotalBalance float64 `json:"total_balance"`
	TotalPnL     float64 `json:"total_pnl"`
	DailyPnL     float64 `json:"daily_pnl"`
}

// SystemHealth 系统健康度。
type SystemHealth struct {
	Status           string `json:"status"` // healthy | degraded
	AILatency        int64  `json:"ai_latency"`        // 毫秒，-1 表示不可用
	ExchangeLatency  int64  `json:"exchange_latency"`  // 毫秒，-1 表示不可用
}

// DashboardSummary 仪表盘 3 合 1 聚合响应。
type DashboardSummary struct {
	Account         AccountSummary `json:"account"`
	ActiveTraders   []TraderDTO    `json:"active_traders"`    // 运行中的前 5 个
	RecentPositions []PositionDTO  `json:"recent_positions"`  // 最新 5 条
	SystemHealth    SystemHealth   `json:"system_health"`
}

// ========== 字段过滤工具 ==========

// FilterFields 按逗号分隔的字段白名单裁剪 JSON 结构（仅处理顶层字段）。
// fields 为空时原样返回。用于 ?fields=id,name,status 这类查询。
func FilterFields(v any, fields []string) any {
	if len(fields) == 0 {
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return v
	}
	want := make(map[string]bool, len(fields))
	for _, f := range fields {
		want[f] = true
	}
	for k := range m {
		if !want[k] {
			delete(m, k)
		}
	}
	return m
}
