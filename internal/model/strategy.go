package model

import (
	"encoding/json"
	"time"
)

// Strategy 策略实体（API设计.md §15 strategies 表）。
// Config 为 StrategyConfig JSON（dto.StrategyConfig，见 dto/strategy.go），
// 实体层不展开，避免 GORM 对嵌套结构的隐式迁移负担；DTO 层负责解析校验。
type Strategy struct {
	ID          string          `gorm:"primaryKey;size:36" json:"id"`
	UserID      string          `gorm:"size:36;index" json:"user_id"`
	Name        string          `gorm:"size:64" json:"name"`
	Description string          `gorm:"size:512" json:"description,omitempty"`
	IsActive    bool            `gorm:"index" json:"is_active"` // 当前生效策略（唯一）
	IsDefault   bool            `json:"is_default"`
	IsPublic    bool            `json:"is_public"`
	Config      json.RawMessage `gorm:"type:text;serializer:json" json:"config"` // StrategyConfig JSON
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
