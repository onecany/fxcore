package store

import (
	"encoding/json"
	"time"

	"fxcore/internal/model"
)

// ========== 决策记录 ==========

// AddDecision 落决策记录（每交易员每周期一条，追加序）。
func (s *Store) AddDecision(d *model.DecisionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = newID()
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now().UTC()
	}
	s.decisions = append(s.decisions, d)
}

// ListDecisionsByTrader 按 trader 列决策（返回深拷贝副本切片，最新在前）。
// offset/limit 分页；traderID 为空则返回全部。
func (s *Store) ListDecisionsByTrader(traderID string, offset, limit int) []*model.DecisionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.DecisionRecord, 0, 16)
	for i := len(s.decisions) - 1; i >= 0; i-- {
		d := s.decisions[i]
		if traderID != "" && d.TraderID != traderID {
			continue
		}
		out = append(out, cloneDecision(d))
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end]
}

// CountDecisions 按 trader 统计决策条数。
func (s *Store) CountDecisions(traderID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, d := range s.decisions {
		if traderID == "" || d.TraderID == traderID {
			n++
		}
	}
	return n
}

// LatestDecisionByTrader 取最近一轮决策（含失败轮；无记录返回 false）。
func (s *Store) LatestDecisionByTrader(traderID string) (*model.DecisionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.decisions) - 1; i >= 0; i-- {
		d := s.decisions[i]
		if d.TraderID == traderID {
			return cloneDecision(d), true
		}
	}
	return nil, false
}

// cloneDecision 深拷贝：Decisions 切片、CandidateCoins/ExecutionLog RawMessage 不共享底层。
func cloneDecision(d *model.DecisionRecord) *model.DecisionRecord {
	cp := *d
	if d.CandidateCoins != nil {
		cp.CandidateCoins = append(json.RawMessage(nil), d.CandidateCoins...)
	}
	if d.ExecutionLog != nil {
		cp.ExecutionLog = append(json.RawMessage(nil), d.ExecutionLog...)
	}
	if d.Decisions != nil {
		cp.Decisions = append([]model.DecisionAction(nil), d.Decisions...)
	}
	return &cp
}

// ========== 权益快照 ==========

// AddEquitySnapshot 落权益快照（交易循环每周期先存，与 AI 无关）。
func (s *Store) AddEquitySnapshot(e *model.EquitySnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = newID()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	s.equities = append(s.equities, e)
}

// ListEquityByTrader 按 trader 列权益快照（返回副本切片，时间升序，最早在前）。
func (s *Store) ListEquityByTrader(traderID string) []*model.EquitySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.EquitySnapshot, 0, 16)
	for _, e := range s.equities {
		if traderID != "" && e.TraderID != traderID {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	return out
}
