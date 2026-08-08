package store

import (
	"encoding/json"
	"time"

	"fxcore/internal/model"
)

// ========== 辩论会话 ==========

// CreateDebateSession 创建辩论会话。
func (s *Store) CreateDebateSession(d *model.DebateSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = newID()
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	s.debateSessions[d.ID] = d
}

// GetDebateSession 按 ID 查会话（返回副本）。
func (s *Store) GetDebateSession(id string) (*model.DebateSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.debateSessions[id]
	if !ok {
		return nil, false
	}
	cp := *d
	if d.Consensus != "" {
		cp.Consensus = d.Consensus
	}
	return &cp, true
}

// ListDebateSessions 列出全部会话（返回副本切片，最新在前）。
func (s *Store) ListDebateSessions() []*model.DebateSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.DebateSession, 0, len(s.debateSessions))
	for _, d := range s.debateSessions {
		cp := *d
		out = append(out, &cp)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.After(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// UpdateDebateSession 更新会话。
func (s *Store) UpdateDebateSession(d *model.DebateSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d.UpdatedAt = time.Now().UTC()
	s.debateSessions[d.ID] = d
}

// DeleteDebateSession 删除会话（含关联参与者/消息/投票）。
func (s *Store) DeleteDebateSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.debateSessions[id]; !ok {
		return false
	}
	delete(s.debateSessions, id)
	s.debateParticipants = filterParticipantsBySession(s.debateParticipants, id)
	s.debateMessages = filterMessagesBySession(s.debateMessages, id)
	s.debateVotes = filterVotesBySession(s.debateVotes, id)
	return true
}

// ========== 参与者 ==========

// AddDebateParticipant 添加参与者。
func (s *Store) AddDebateParticipant(p *model.DebateParticipant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = newID()
	}
	s.debateParticipants = append(s.debateParticipants, p)
}

// ListDebateParticipants 按 session 列参与者。
func (s *Store) ListDebateParticipants(sessionID string) []*model.DebateParticipant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.DebateParticipant, 0, 8)
	for _, p := range s.debateParticipants {
		if p.SessionID != sessionID {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// ========== 消息 ==========

// AddDebateMessage 追加消息。
func (s *Store) AddDebateMessage(m *model.DebateMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = newID()
	}
	if m.Timestamp == 0 {
		m.Timestamp = time.Now().Unix()
	}
	m.CreatedAt = time.Now().UTC()
	s.debateMessages = append(s.debateMessages, m)
}

// ListDebateMessages 按 session 列消息（时间升序）。
func (s *Store) ListDebateMessages(sessionID string) []*model.DebateMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.DebateMessage, 0, 32)
	for _, m := range s.debateMessages {
		if m.SessionID != sessionID {
			continue
		}
		cp := *m
		out = append(out, &cp)
	}
	return out
}

// ========== 投票 ==========

// AddDebateVote 追加投票。
func (s *Store) AddDebateVote(v *model.DebateVote) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = newID()
	}
	s.debateVotes = append(s.debateVotes, v)
}

// ListDebateVotes 按 session 列投票。
func (s *Store) ListDebateVotes(sessionID string) []*model.DebateVote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.DebateVote, 0, 8)
	for _, v := range s.debateVotes {
		if v.SessionID != sessionID {
			continue
		}
		cp := *v
		if v.Extra != nil {
			cp.Extra = append(json.RawMessage(nil), v.Extra...)
		}
		out = append(out, &cp)
	}
	return out
}

// ========== 过滤辅助 ==========

func filterParticipantsBySession(ps []*model.DebateParticipant, id string) []*model.DebateParticipant {
	out := ps[:0]
	for _, p := range ps {
		if p.SessionID != id {
			out = append(out, p)
		}
	}
	return out
}

func filterMessagesBySession(ms []*model.DebateMessage, id string) []*model.DebateMessage {
	out := ms[:0]
	for _, m := range ms {
		if m.SessionID != id {
			out = append(out, m)
		}
	}
	return out
}

func filterVotesBySession(vs []*model.DebateVote, id string) []*model.DebateVote {
	out := vs[:0]
	for _, v := range vs {
		if v.SessionID != id {
			out = append(out, v)
		}
	}
	return out
}
