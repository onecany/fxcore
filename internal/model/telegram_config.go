package model

import "time"

// TelegramConfig Telegram 绑定配置（API设计.md §15 telegram_configs 表）。
// BotTokenEnc RSA-OAEP 加密存储；单用户部署，表内恒一条记录（store 按单例处理）。
type TelegramConfig struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	BotTokenEnc string    `gorm:"size:4096" json:"-"` // 加密存储
	ChatID     string     `gorm:"size:64" json:"chat_id,omitempty"`
	Username   string     `gorm:"size:64" json:"username,omitempty"`
	BoundAt    *time.Time `json:"bound_at,omitempty"` // 绑定时间（保存 token 后 /start 建立会话）
	ModelID    string     `gorm:"size:36" json:"model_id,omitempty"`
	Language   string     `gorm:"size:8" json:"language,omitempty"` // zh | en | id
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
