package model

import "time"

// AIModel AI 模型实体。APIKeyEnc 为 RSA-OAEP 加密后的 base64，永不明文落库。
type AIModel struct {
	ID           string          `gorm:"primaryKey;size:36" json:"id"`
	UserID       string          `gorm:"size:36;index" json:"user_id"` // 属主用户（多用户隔离）
	Name         string     `gorm:"size:64" json:"name"`       // 用户自定义别名
	Provider     string     `gorm:"size:32;index" json:"provider"`
	ModelName    string     `gorm:"size:128" json:"model_name"` // 实际模型 ID
	APIKeyEnc    string     `gorm:"size:2048" json:"-"`         // RSA 密文
	APIKeyPrefix string     `gorm:"size:32" json:"api_key_prefix"` // 脱敏 sk-****abcd
	Status       string     `gorm:"size:16" json:"status"`      // active | inactive | error
	Config       string     `gorm:"type:text" json:"config"`    // JSON 字符串
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	LastTestLatencyMS int64      `gorm:"-" json:"-"` // 最近一次测试延迟（毫秒），供仪表盘健康度聚合
	RotatedAt         *time.Time `json:"rotated_at,omitempty"` // 最近一次 Key 轮换时间（文档 6.2）
	DeletedAt    *time.Time `gorm:"index" json:"-"`       // 软删除
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
