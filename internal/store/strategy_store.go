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
		s.mu.Lock()
		s.strategies[st.ID] = st
		s.mu.Unlock()
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
func (s *Store) ListStrategies(userID string) []*model.Strategy {
	if s.db != nil {
		var rows []model.Strategy
		q := s.db.Order("created_at DESC")
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		q.Find(&rows)
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
		if userID != "" && st.UserID != userID {
			continue
		}
		out = append(out, cloneStrategy(st))
	}
	return out
}

// GetActiveStrategy 取当前生效策略（is_active=true，单用户场景至多一条）。
func (s *Store) GetActiveStrategy(userID string) (*model.Strategy, bool) {
	if s.db != nil {
		var st model.Strategy
		q := s.db.Where("is_active = ?", true)
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		if err := q.First(&st).Error; err != nil {
			return nil, false
		}
		return cloneStrategy(&st), true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.strategies {
		if st.IsActive && (userID == "" || st.UserID == "" || st.UserID == userID) {
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
		// 同步内存镜像（读优先 DB，镜像保持一致性防御）
		s.mu.Lock()
		s.strategies[st.ID] = st
		s.mu.Unlock()
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
		if res.RowsAffected > 0 {
			s.mu.Lock()
			delete(s.strategies, id)
			s.mu.Unlock()
		}
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
func (s *Store) ActivateStrategy(userID, id string) (*model.Strategy, bool) {
	if s.db != nil {
		var out *model.Strategy
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var target model.Strategy
			if err := tx.First(&target, "id = ?", id).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			// 只去激活同属主用户的生效策略（P1-7：is_active 按 user 作用域，不跨用户干扰）
			deact := tx.Model(&model.Strategy{}).Where("is_active = ? AND id != ?", true, id)
			if userID != "" {
				deact = deact.Where("user_id = ?", userID)
			}
			if err := deact.Update("is_active", false).Error; err != nil {
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
		// 同步内存镜像（激活态唯一性在镜像中保持一致）
		s.mu.Lock()
		for _, st := range s.strategies {
			st.IsActive = false
			st.UpdatedAt = time.Now().UTC()
		}
		if cur, ok := s.strategies[id]; ok {
			cur.IsActive = true
			cur.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()
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
		if st.ID != id && st.IsActive && (userID == "" || st.UserID == "" || st.UserID == userID) {
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
