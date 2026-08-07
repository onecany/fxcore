package store

import (
	"time"

	"fxcore/internal/model"
)

// ========== Telegram 配置（单用户单例） ==========

// GetTelegramConfig 取当前绑定配置（返回副本）。
func (s *Store) GetTelegramConfig() (*model.TelegramConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.telegram == nil {
		return nil, false
	}
	cp := *s.telegram
	return &cp, true
}

// UpsertTelegramConfig 保存/更新配置（无则建，有则更，表内恒一条）。
// BotTokenEnc 已由 service 层加密；ChatID/Username 由 bot 绑定流程回填。
func (s *Store) UpsertTelegramConfig(cfg *model.TelegramConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if s.telegram == nil {
		cfg.ID = newID()
		cfg.CreatedAt = now
	} else {
		cfg.ID = s.telegram.ID
		cfg.CreatedAt = s.telegram.CreatedAt
		// 未显式变更的绑定字段保留旧值（绑定后仅换模型等场景）
		if cfg.ChatID == "" {
			cfg.ChatID = s.telegram.ChatID
		}
		if cfg.Username == "" {
			cfg.Username = s.telegram.Username
		}
		if cfg.BoundAt == nil {
			cfg.BoundAt = s.telegram.BoundAt
		}
	}
	cfg.UpdatedAt = now
	s.telegram = cfg
}

// DeleteTelegramConfig 解绑：清空单例（含加密 token）。
func (s *Store) DeleteTelegramConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telegram = nil
}
