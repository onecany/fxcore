package store

import (
	"time"

	"fxcore/internal/model"
)

// ========== 交易所账户 ==========

// CreateExchange 创建交易所账户（凭据已由 service 层加密）。
func (s *Store) CreateExchange(e *model.Exchange) {
	if e.ID == "" {
		e.ID = newID()
	}
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now
	if s.db != nil {
		s.db.Create(e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges[e.ID] = e
}

// GetExchange 按 ID 查未删除交易所（返回副本）。
// GORM 路径：DeletedAt 非空自动排除（软删除约定）。
func (s *Store) GetExchange(id string) (*model.Exchange, bool) {
	if s.db != nil {
		var e model.Exchange
		if err := s.db.First(&e, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return &e, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.exchanges[id]
	if !ok || e.DeletedAt != nil {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// ListExchanges 列出全部未删除交易所（返回副本切片）。
func (s *Store) ListExchanges(userID string) []*model.Exchange {
	if s.db != nil {
		var rows []model.Exchange
		q := s.db.Order("created_at DESC")
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		q.Find(&rows)
		out := make([]*model.Exchange, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Exchange, 0, len(s.exchanges))
	for _, e := range s.exchanges {
		if e.DeletedAt != nil {
			continue
		}
		if userID != "" && e.UserID != userID {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// UpdateExchange 更新交易所（入参副本锁内替换）。
func (s *Store) UpdateExchange(e *model.Exchange) {
	e.UpdatedAt = time.Now().UTC()
	if s.db != nil {
		s.db.Save(e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges[e.ID] = e
}

// DeleteExchange 软删除，返回是否命中。
// §11：DELETE /exchanges/{id} 语义为「断开关联交易员」——软删保留审计痕迹。
func (s *Store) DeleteExchange(id string) bool {
	if s.db != nil {
		// GORM 标准软删除（模型带 DeletedAt → UPDATE deleted_at=now）
		res := s.db.Delete(&model.Exchange{}, "id = ?", id)
		return res.RowsAffected > 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.exchanges[id]
	if !ok || e.DeletedAt != nil {
		return false
	}
	now := time.Now().UTC()
	e.DeletedAt = &now
	e.UpdatedAt = now
	return true
}
