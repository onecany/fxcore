package dto

// ========== Telegram 模块（API设计.md §11 telegram 路由） ==========

// SaveTelegramRequest 保存并绑定（POST /telegram：保存后 /start 绑定）。
type SaveTelegramRequest struct {
	BotToken string `json:"bot_token" binding:"required,max=512"`
	ModelID  string `json:"model_id" binding:"required,max=36"`
	ChatID   string `json:"chat_id,omitempty"`
}

// SetTelegramModelRequest 仅换模型（POST /telegram/model）。
type SetTelegramModelRequest struct {
	ModelID string `json:"model_id" binding:"required,max=36"`
}

// TelegramConfigDTO 配置响应（bot_token 脱敏，不返回原文）。
type TelegramConfigDTO struct {
	BotTokenPrefix string `json:"bot_token_prefix,omitempty"` // 脱敏，如 12345****
	ChatID         string `json:"chat_id,omitempty"`
	Username       string `json:"username,omitempty"`
	BoundAt        int64  `json:"bound_at,omitempty"` // unix 秒；0=未绑定
	ModelID        string `json:"model_id,omitempty"`
	Language       string `json:"language,omitempty"`
}
