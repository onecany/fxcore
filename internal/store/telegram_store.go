package store

import (
	"time"

	"gorm.io/gorm"

	"fxcore/internal/model"
)

// ========== Telegram 配置（单用户单例） ==========

// GetTelegramConfig 取当前绑定配置（返回副本）。
// DB 路径：单例行（首条）。
func (s *Store) GetTelegramConfig(userID string) (*model.TelegramConfig, bool) {
	if s.db != nil {
		var cfg model.TelegramConfig
		if err := s.db.Where("user_id = ?", userID).Order("created_at ASC").First(&cfg).Error; err != nil {
			return nil, false
		}
		return &cfg, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.telegram[userID]
	if !ok {
		return nil, false
	}
	cp := *cfg
	return &cp, true
}

// UpsertTelegramConfig 保存/更新配置（无则建，有则更，表内恒一条）。
// BotTokenEnc 已由 service 层加密；ChatID/Username 由 bot 绑定流程回填。
// DB 路径：应用层锁串行化（防 mysql 并发双行）+ 事务读单例 → 合并绑定字段 → Save；
// 成功后同步内存镜像。
func (s *Store) UpsertTelegramConfig(userID string, cfg *model.TelegramConfig) {
	now := time.Now().UTC()
	if s.db != nil {
		s.mu.Lock() // 每用户单例语义：串行化整个 upsert
		defer s.mu.Unlock()
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var cur model.TelegramConfig
			exists := tx.Where("user_id = ?", userID).Order("created_at ASC").First(&cur).Error == nil
			if !exists {
				cfg.ID = newID()
				cfg.CreatedAt = now
			} else {
				cfg.ID = cur.ID
				cfg.CreatedAt = cur.CreatedAt
				if cfg.ChatID == "" {
					cfg.ChatID = cur.ChatID
				}
				if cfg.Username == "" {
					cfg.Username = cur.Username
				}
				if cfg.BoundAt == nil {
					cfg.BoundAt = cur.BoundAt
				}
			}
			cfg.UserID = userID
			cfg.UpdatedAt = now
			return tx.Save(cfg).Error
		})
		if err != nil {
			// 事务失败不更新内存镜像（保持 DB 一致）
			return
		}
		cfg.UserID = userID
		s.telegram[userID] = cfg // 已持有 s.mu（见函数头），不再重复加锁
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.telegram[userID]
	if !ok {
		cfg.ID = newID()
		cfg.CreatedAt = now
	} else {
		cfg.ID = cur.ID
		cfg.CreatedAt = cur.CreatedAt
		// 未显式变更的绑定字段保留旧值（绑定后仅换模型等场景）
		if cfg.ChatID == "" {
			cfg.ChatID = cur.ChatID
		}
		if cfg.Username == "" {
			cfg.Username = cur.Username
		}
		if cfg.BoundAt == nil {
			cfg.BoundAt = cur.BoundAt
		}
	}
	cfg.UserID = userID
	cfg.UpdatedAt = now
	s.telegram[userID] = cfg
}

// DeleteTelegramConfig 解绑：清空单例（含加密 token）。
// DB 路径：物理删除全部行（单例语义），同步内存镜像。
func (s *Store) DeleteTelegramConfig(userID string) {
	if s.db != nil {
		s.db.Where("user_id = ?", userID).Delete(&model.TelegramConfig{})
		s.mu.Lock()
		delete(s.telegram, userID)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telegram = nil
}
