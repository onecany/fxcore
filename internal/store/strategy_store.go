package store

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"fxcore/internal/model"
)

// ========== 策略 ==========

// CreateStrategy 创建策略。Config 为 StrategyConfig JSON。
func (s *Store) CreateStrategy(st *model.Strategy) {
	if st.ID == "" {
		st.ID = newID()
	}
	now := time.Now().UTC()
	st.CreatedAt = now
	st.UpdatedAt = now
	if s.db != nil {
		s.db.Create(st)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategies[st.ID] = st
}

// GetStrategy 按 ID 查策略（返回深拷贝副本）。
func (s *Store) GetStrategy(id string) (*model.Strategy, bool) {
	if s.db != nil {
		var st model.Strategy
		if err := s.db.First(&st, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return cloneStrategy(&st), true
	}
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
	if s.db != nil {
		var rows []model.Strategy
		s.db.Order("created_at DESC").Find(&rows)
		out := make([]*model.Strategy, 0, len(rows))
		for i := range rows {
			out = append(out, cloneStrategy(&rows[i]))
		}
		return out
	}
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
	if s.db != nil {
		var st model.Strategy
		if err := s.db.Where("is_active = ?", true).First(&st).Error; err != nil {
			return nil, false
		}
		return cloneStrategy(&st), true
	}
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
	st.UpdatedAt = time.Now().UTC()
	if s.db != nil {
		s.db.Save(st)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategies[st.ID] = st
}

// DeleteStrategy 删除策略，返回是否命中。
func (s *Store) DeleteStrategy(id string) bool {
	if s.db != nil {
		res := s.db.Delete(&model.Strategy{}, "id = ?", id)
		return res.RowsAffected > 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.strategies[id]; !ok {
		return false
	}
	delete(s.strategies, id)
	return true
}

// ActivateStrategy 置为生效策略：先将其余 is_active 置 false（保证唯一），
// 再置目标为 true。返回更新后的副本。DB 路径走事务保证原子性。
func (s *Store) ActivateStrategy(id string) (*model.Strategy, bool) {
	if s.db != nil {
		var out *model.Strategy
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var target model.Strategy
			if err := tx.First(&target, "id = ?", id).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			if err := tx.Model(&model.Strategy{}).
				Where("is_active = ? AND id != ?", true, id).
				Update("is_active", false).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Strategy{}).Where("id = ?", id).
				Update("is_active", true).Error; err != nil {
				return err
			}
			target.IsActive = true
			target.UpdatedAt = now
			out = cloneStrategy(&target)
			return nil
		})
		if err != nil {
			return nil, false
		}
		return out, true
	}
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
