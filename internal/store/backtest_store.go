package store

import (
	"encoding/json"
	"time"

	"fxcore/internal/model"
)

// ========== 回测运行 ==========

// CreateBacktestRun 创建回测运行。
func (s *Store) CreateBacktestRun(r *model.BacktestRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	s.backtestRuns[r.RunID] = r
}

// GetBacktestRun 按 run_id 查运行（返回副本）。
func (s *Store) GetBacktestRun(runID string) (*model.BacktestRun, bool) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	r.UpdatedAt = time.Now().UTC()
	s.backtestRuns[r.RunID] = r
}

// DeleteBacktestRun 删除运行（含关联 equities/trades/decisions/checkpoint）。
func (s *Store) DeleteBacktestRun(runID string) bool {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = newID()
	}
	s.backtestEquities = append(s.backtestEquities, e)
}

// ListBacktestEquities 按 run 列权益（时间升序）。
func (s *Store) ListBacktestEquities(runID string) []*model.BacktestEquity {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = newID()
	}
	s.backtestTrades = append(s.backtestTrades, t)
}

// ListBacktestTrades 按 run 列成交（时间升序）。
func (s *Store) ListBacktestTrades(runID string) []*model.BacktestTrade {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = newID()
	}
	s.backtestDecisions = append(s.backtestDecisions, d)
}

// ListBacktestDecisions 按 run 列决策（周期升序）。
func (s *Store) ListBacktestDecisions(runID string) []*model.BacktestDecision {
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

// SaveBacktestCheckpoint 存检查点。
func (s *Store) SaveBacktestCheckpoint(runID string, payload json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtestCheckpoints[runID] = &model.BacktestCheckpoint{
		RunID:   runID,
		Payload: payload,
	}
}

// GetBacktestCheckpoint 取检查点。
func (s *Store) GetBacktestCheckpoint(runID string) (*model.BacktestCheckpoint, bool) {
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
