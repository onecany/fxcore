package store

import (
	"time"

	"fxcore/internal/model"
)

// ========== 交易所账户 ==========

// CreateExchange 创建交易所账户（凭据已由 service 层加密）。
func (s *Store) CreateExchange(e *model.Exchange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = newID()
	}
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now
	s.exchanges[e.ID] = e
}

// GetExchange 按 ID 查未删除交易所（返回副本）。
func (s *Store) GetExchange(id string) (*model.Exchange, bool) {
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
func (s *Store) ListExchanges() []*model.Exchange {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Exchange, 0, len(s.exchanges))
	for _, e := range s.exchanges {
		if e.DeletedAt != nil {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// UpdateExchange 更新交易所（入参副本锁内替换）。
func (s *Store) UpdateExchange(e *model.Exchange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.UpdatedAt = time.Now().UTC()
	s.exchanges[e.ID] = e
}

// DeleteExchange 软删除，返回是否命中。
// §11：DELETE /exchanges/{id} 语义为「断开关联交易员」——软删保留审计痕迹。
func (s *Store) DeleteExchange(id string) bool {
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
