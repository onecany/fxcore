package store

import (
	"encoding/json"
	"time"

	"fxcore/internal/model"
)

// ========== 回测运行 ==========

// CreateBacktestRun 创建回测运行。
func (s *Store) CreateBacktestRun(r *model.BacktestRun) {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	if s.db != nil {
		s.db.Create(r)
		s.mu.Lock()
		s.backtestRuns[r.RunID] = r
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestRuns[r.RunID] = r
}

// GetBacktestRun 按 run_id 查运行（返回副本）。
func (s *Store) GetBacktestRun(runID string) (*model.BacktestRun, bool) {
	if s.db != nil {
		var r model.BacktestRun
		if err := s.db.First(&r, "run_id = ?", runID).Error; err != nil {
			return nil, false
		}
		return &r, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.backtestRuns[runID]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// ListBacktestRuns 列出全部运行（返回副本切片，最新在前）。
func (s *Store) ListBacktestRuns() []*model.BacktestRun {
	if s.db != nil {
		var rows []model.BacktestRun
		s.db.Order("created_at DESC").Find(&rows)
		out := make([]*model.BacktestRun, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.BacktestRun, 0, len(s.backtestRuns))
	for _, r := range s.backtestRuns {
		cp := *r
		out = append(out, &cp)
	}
	// 按 CreatedAt 降序
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.After(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// UpdateBacktestRun 更新运行（进度/状态）。
func (s *Store) UpdateBacktestRun(r *model.BacktestRun) {
	r.UpdatedAt = time.Now().UTC()
	if s.db != nil {
		s.db.Save(r)
		s.mu.Lock()
		s.backtestRuns[r.RunID] = r
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestRuns[r.RunID] = r
}

// DeleteBacktestRun 删除运行（含关联 equities/trades/decisions/checkpoint）。
func (s *Store) DeleteBacktestRun(runID string) bool {
	if s.db != nil {
		res := s.db.Delete(&model.BacktestRun{}, "run_id = ?", runID)
		if res.RowsAffected == 0 {
			return false
		}
		// 级联删除关联数据
		s.db.Where("run_id = ?", runID).Delete(&model.BacktestEquity{})
		s.db.Where("run_id = ?", runID).Delete(&model.BacktestTrade{})
		s.db.Where("run_id = ?", runID).Delete(&model.BacktestDecision{})
		s.db.Where("run_id = ?", runID).Delete(&model.BacktestCheckpoint{})
		s.mu.Lock()
		delete(s.backtestRuns, runID)
		s.backtestEquities = filterByRun(s.backtestEquities, runID)
		s.backtestTrades = filterTradesByRun(s.backtestTrades, runID)
		s.backtestDecisions = filterDecisionsByRun(s.backtestDecisions, runID)
		delete(s.backtestCheckpoints, runID)
		s.mu.Unlock()
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.backtestRuns[runID]; !ok {
		return false
	}
	delete(s.backtestRuns, runID)
	s.backtestEquities = filterByRun(s.backtestEquities, runID)
	s.backtestTrades = filterTradesByRun(s.backtestTrades, runID)
	s.backtestDecisions = filterDecisionsByRun(s.backtestDecisions, runID)
	delete(s.backtestCheckpoints, runID)
	return true
}

// ========== 权益/成交/决策 ==========

// AddBacktestEquity 追加权益点。
func (s *Store) AddBacktestEquity(e *model.BacktestEquity) {
	if e.ID == "" {
		e.ID = newID()
	}
	if s.db != nil {
		s.db.Create(e)
		s.mu.Lock()
		s.backtestEquities = append(s.backtestEquities, e)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestEquities = append(s.backtestEquities, e)
}

// ListBacktestEquities 按 run 列权益（时间升序）。
func (s *Store) ListBacktestEquities(runID string) []*model.BacktestEquity {
	if s.db != nil {
		var rows []model.BacktestEquity
		s.db.Where("run_id = ?", runID).Order("timestamp ASC").Find(&rows)
		out := make([]*model.BacktestEquity, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.BacktestEquity, 0, 16)
	for _, e := range s.backtestEquities {
		if e.RunID != runID {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// AddBacktestTrade 追加回测成交。
func (s *Store) AddBacktestTrade(t *model.BacktestTrade) {
	if t.ID == "" {
		t.ID = newID()
	}
	if s.db != nil {
		s.db.Create(t)
		s.mu.Lock()
		s.backtestTrades = append(s.backtestTrades, t)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestTrades = append(s.backtestTrades, t)
}

// ListBacktestTrades 按 run 列成交（时间升序）。
func (s *Store) ListBacktestTrades(runID string) []*model.BacktestTrade {
	if s.db != nil {
		var rows []model.BacktestTrade
		s.db.Where("run_id = ?", runID).Order("timestamp ASC").Find(&rows)
		out := make([]*model.BacktestTrade, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.BacktestTrade, 0, 16)
	for _, t := range s.backtestTrades {
		if t.RunID != runID {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	return out
}

// AddBacktestDecision 追加回测决策。
func (s *Store) AddBacktestDecision(d *model.BacktestDecision) {
	if d.ID == "" {
		d.ID = newID()
	}
	if s.db != nil {
		s.db.Create(d)
		s.mu.Lock()
		s.backtestDecisions = append(s.backtestDecisions, d)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestDecisions = append(s.backtestDecisions, d)
}

// ListBacktestDecisions 按 run 列决策（周期升序）。
func (s *Store) ListBacktestDecisions(runID string) []*model.BacktestDecision {
	if s.db != nil {
		var rows []model.BacktestDecision
		s.db.Where("run_id = ?", runID).Order("cycle ASC").Find(&rows)
		out := make([]*model.BacktestDecision, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.BacktestDecision, 0, 16)
	for _, d := range s.backtestDecisions {
		if d.RunID != runID {
			continue
		}
		cp := *d
		cp.Payload = append(json.RawMessage(nil), d.Payload...)
		out = append(out, &cp)
	}
	return out
}

// GetBacktestDecision 按 run+cycle 取决策。
func (s *Store) GetBacktestDecision(runID string, cycle int64) (*model.BacktestDecision, bool) {
	if s.db != nil {
		var d model.BacktestDecision
		if err := s.db.Where("run_id = ? AND cycle = ?", runID, cycle).First(&d).Error; err != nil {
			return nil, false
		}
		return &d, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.backtestDecisions {
		if d.RunID == runID && d.Cycle == cycle {
			cp := *d
			cp.Payload = append(json.RawMessage(nil), d.Payload...)
			return &cp, true
		}
	}
	return nil, false
}

// SaveBacktestCheckpoint 存检查点（upsert 语义：同 run 覆盖）。
func (s *Store) SaveBacktestCheckpoint(runID string, payload json.RawMessage) {
	if s.db != nil {
		cp := &model.BacktestCheckpoint{RunID: runID, Payload: payload}
		if err := s.db.Where("run_id = ?", runID).Delete(&model.BacktestCheckpoint{}).Error; err == nil {
			s.db.Create(cp)
		}
		s.mu.Lock()
		s.backtestCheckpoints[runID] = cp
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestCheckpoints[runID] = &model.BacktestCheckpoint{
		RunID:   runID,
		Payload: payload,
	}
}

// GetBacktestCheckpoint 取检查点。
func (s *Store) GetBacktestCheckpoint(runID string) (*model.BacktestCheckpoint, bool) {
	if s.db != nil {
		var cp model.BacktestCheckpoint
		if err := s.db.First(&cp, "run_id = ?", runID).Error; err != nil {
			return nil, false
		}
		return &cp, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.backtestCheckpoints[runID]
	if !ok {
		return nil, false
	}
	out := &model.BacktestCheckpoint{RunID: cp.RunID, Payload: append(json.RawMessage(nil), cp.Payload...)}
	return out, true
}

// ========== 过滤辅助 ==========

func filterByRun(es []*model.BacktestEquity, runID string) []*model.BacktestEquity {
	out := es[:0]
	for _, e := range es {
		if e.RunID != runID {
			out = append(out, e)
		}
	}
	return out
}

func filterTradesByRun(ts []*model.BacktestTrade, runID string) []*model.BacktestTrade {
	out := ts[:0]
	for _, t := range ts {
		if t.RunID != runID {
			out = append(out, t)
		}
	}
	return out
}

func filterDecisionsByRun(ds []*model.BacktestDecision, runID string) []*model.BacktestDecision {
	out := ds[:0]
	for _, d := range ds {
		if d.RunID != runID {
			out = append(out, d)
		}
	}
	return out
}
