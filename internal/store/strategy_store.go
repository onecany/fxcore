package store

import (
	"encoding/json"
	"time"

	"fxcore/internal/model"
)

// ========== 策略 ==========

// CreateStrategy 创建策略。Config 为 StrategyConfig JSON。
func (s *Store) CreateStrategy(st *model.Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.ID == "" {
		st.ID = newID()
	}
	now := time.Now().UTC()
	st.CreatedAt = now
	st.UpdatedAt = now
	s.strategies[st.ID] = st
}

// GetStrategy 按 ID 查策略（返回深拷贝副本）。
func (s *Store) GetStrategy(id string) (*model.Strategy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.strategies[id]
	if !ok {
		return nil, false
	}
	return cloneStrategy(st), true
}

// ListStrategies 列出全部策略（返回深拷贝副本切片）。
func (s *Store) ListStrategies() []*model.Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Strategy, 0, len(s.strategies))
	for _, st := range s.strategies {
		out = append(out, cloneStrategy(st))
	}
	return out
}

// GetActiveStrategy 取当前生效策略（is_active=true，单用户场景至多一条）。
func (s *Store) GetActiveStrategy() (*model.Strategy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.strategies {
		if st.IsActive {
			return cloneStrategy(st), true
		}
	}
	return nil, false
}

// UpdateStrategy 更新策略（入参副本锁内替换）。
func (s *Store) UpdateStrategy(st *model.Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.UpdatedAt = time.Now().UTC()
	s.strategies[st.ID] = st
}

// DeleteStrategy 删除策略，返回是否命中。
func (s *Store) DeleteStrategy(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.strategies[id]; !ok {
		return false
	}
	delete(s.strategies, id)
	return true
}

// ActivateStrategy 置为生效策略：写锁内先把其余 is_active 置 false（保证唯一），
// 再置目标为 true。返回更新后的副本。
func (s *Store) ActivateStrategy(id string) (*model.Strategy, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.strategies[id]
	if !ok {
		return nil, false
	}
	now := time.Now().UTC()
	for _, st := range s.strategies {
		if st.ID != id && st.IsActive {
			st.IsActive = false
			st.UpdatedAt = now
		}
	}
	target.IsActive = true
	target.UpdatedAt = now
	return cloneStrategy(target), true
}

// cloneStrategy 深拷贝：Config RawMessage 不共享底层数组。
func cloneStrategy(st *model.Strategy) *model.Strategy {
	cp := *st
	if st.Config != nil {
		cp.Config = append(json.RawMessage(nil), st.Config...)
	}
	return &cp
}
