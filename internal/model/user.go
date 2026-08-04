// Package model 定义全局共享的 GORM 实体（API设计.md 7.1 蓝图的 internal/model/）。
// 当前运行时使用 internal/store 的内存实现；实体已带 GORM Tag，
// v1/v2 共享本包，后续接入 SQLite/PostgreSQL 时可直接迁移。
package model

import "time"

// User 用户实体。PasswordHash / SignSecret 永不序列化。
type User struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Email        string    `gorm:"uniqueIndex;size:255" json:"email"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Avatar       string    `gorm:"size:512" json:"avatar,omitempty"`
	SignSecret   string    `gorm:"size:128" json:"-"` // HMAC-SHA256 请求签名 user_secret（文档 6.1）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
