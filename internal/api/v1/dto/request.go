// Package dto 定义 /api/v1 的请求/响应结构体。
//
// 按 data/API设计.md 第 3、9 节要求：JSON Tag 一律 snake_case，
// 字段语义与 web/src/api/v1/types/contract.ts 一一对应。
// （camelCase/snake_case 差异是文档自身矛盾，见契约文件头注释。）
package dto

import "encoding/json"

// ========== 通用查询 ==========

// ListQuery 分页 + 状态 + 字段过滤的通用查询参数。
type ListQuery struct {
	Page   int    `form:"page" json:"page"`
	Size   int    `form:"size" json:"size"`
	Status string `form:"status" json:"status"`
	Fields string `form:"fields" json:"fields"` // 例: ?fields=id,name,status
	Symbol string `form:"symbol" json:"symbol"`
}

// Normalized 返回规范化后的分页参数（1<=page<=100000, 1<=size<=100）。
// L2：page 必须钳制上限——(page-1)*size 在 page=MaxInt64 时整数溢出，
// 切片越界直接 panic（已实证），且 (100000-1)*100 在 32/64 位下均安全。
func (q *ListQuery) Normalized() (page, size int) {
	page = q.Page
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	size = q.Size
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

// ========== 认证模块 ==========

// LoginRequest 登录请求。
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=128"`
}

// RegisterRequest 注册请求。
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Nickname string `json:"nickname,omitempty" binding:"max=64"`
}

// RefreshRequest 刷新令牌请求。
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ForgotPasswordRequest 忘记密码请求（发送重置邮件）。
// 响应不做存在性区分（防用户枚举）：无论邮箱是否注册都返回通用成功文案。
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
}

// ResetPasswordRequest 重置密码请求。
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required,max=128"` // 邮件重置链接中的一次性令牌
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// ========== AI 模型模块 ==========

// CreateModelRequest 创建/更新 AI 模型。api_key 后端 RSA 加密后落库。
// api_key 无 binding:required——Update 语义是「留空保留原 Key」（service Update 的 `if in.APIKey != ""`），
// 创建必填校验在 service Create 层做（同一 DTO 双用途，binding 层无法区分）。
type CreateModelRequest struct {
	Name      string          `json:"name" binding:"required,max=64"`
	Provider  string          `json:"provider" binding:"required,max=32"`
	ModelName string          `json:"model_name" binding:"required,max=128"`
	APIKey    string          `json:"api_key" binding:"max=512"`
	Config    json.RawMessage `json:"config"` // 温度、TopP 等
}

// TestModelRequest 连通测试请求（可选；空请求体时使用已存 Key）。
type TestModelRequest struct {
	APIKey    string `json:"api_key"`
	ModelName string `json:"model_name"`
	Provider  string `json:"provider"`
}

// ClosePositionRequest 平仓请求（可选；缺省 pnl=0）。
type ClosePositionRequest struct {
	PnL *float64 `json:"pnl"`
}

// ========== 交易员模块 ==========

// ModelConfig 引用已保存的 AI 模型。
type ModelConfig struct {
	Provider   string          `json:"provider" binding:"required"`
	ModelID    string          `json:"model_id" binding:"required"` // 引用 internal/model 中的 AI 模型 ID
	Parameters json.RawMessage `json:"parameters"`
}

// RiskConfig 风控参数。
type RiskConfig struct {
	MaxPositionSize float64 `json:"max_position_size" binding:"required"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	MaxDailyLoss    float64 `json:"max_daily_loss"`
}

// Schedule 定时调度。
type Schedule struct {
	Interval    int           `json:"interval"` // 轮询间隔（秒）
	ActiveHours []ActiveHours `json:"active_hours"`
}

// MaxActiveHours 活跃时段数量上限（S5：防超大数组入库）。
const MaxActiveHours = 24

// ActiveHours 活跃时段。
type ActiveHours struct {
	Start string `json:"start"` // "HH:MM"
	End   string `json:"end"`   // "HH:MM"
}

// CreateTraderRequest 创建交易员请求。
type CreateTraderRequest struct {
	Name        string      `json:"name" binding:"required,max=64"`
	Exchange    string      `json:"exchange" binding:"required,max=32"`
	ModelConfig ModelConfig `json:"model_config" binding:"required"`
	StrategyID  string      `json:"strategy_id" binding:"required,max=64"`
	RiskConfig  RiskConfig  `json:"risk_config" binding:"required"`
	Schedule    *Schedule   `json:"schedule"`
}

// UpdateTraderRequest PATCH 部分更新（Partial<CreateTraderRequest>）。
// 指针字段区分「未传」与「置空」。
type UpdateTraderRequest struct {
	Name        *string      `json:"name"`
	Exchange    *string      `json:"exchange"`
	ModelConfig *ModelConfig `json:"model_config"`
	StrategyID  *string      `json:"strategy_id"`
	RiskConfig  *RiskConfig  `json:"risk_config"`
	Schedule    *Schedule    `json:"schedule"`
}
