// Package debate 辩论竞技场引擎（API设计.md §11/§15 行为契约）。
// 状态机：pending→running→voting→completed|cancelled；SSE 事件流
// （initial/round_start/message/round_end/vote/consensus/error + 心跳）。
package debate

import (
	"context"
	"sync"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/llm"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// EventType SSE 事件类型（§11 /debates/{id}/stream）。
const (
	EventInitial    = "initial"
	EventRoundStart = "round_start"
	EventMessage    = "message"
	EventRoundEnd   = "round_end"
	EventVote       = "vote"
	EventConsensus  = "consensus"
	EventError      = "error"
	EventHeartbeat  = "heartbeat"
)

// Event SSE 事件。
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// personalities 固定 5 人格（GET /debates/personalities）。
var personalities = []dto.Personality{
	{ID: dto.PersonalityBull, Name: "Bull", Description: "Trend-following optimist; identifies momentum and upside targets", Color: "#22c55e"},
	{ID: dto.PersonalityBear, Name: "Bear", Description: "Risk-aware pessimist; spots distribution and downside risks", Color: "#ef4444"},
	{ID: dto.PersonalityAnalyst, Name: "Analyst", Description: "Data-driven technician; reads structure, volume and indicators", Color: "#3b82f6"},
	{ID: dto.PersonalityContrarian, Name: "Contrarian", Description: "Opposes consensus; finds crowded trades and reversals", Color: "#f59e0b"},
	{ID: dto.PersonalityRiskManager, Name: "Risk Manager", Description: "Capital preservation first; sizes positions and sets stop losses", Color: "#8b5cf6"},
}

// Engine 辩论引擎。
type Engine struct {
	store  *store.Store
	models llm.ModelProvider
	ai     *llm.Client

	mu       sync.Mutex
	sessions map[string]*session
	subs     map[string]map[chan Event]struct{}
}

// session 单个会话运行时状态。
type session struct {
	mu        sync.Mutex
	meta      *model.DebateSession
	cancel    context.CancelFunc
	consensus *dto.DebateVoteDTO // 最终共识（completed 后）
}

// NewEngine 构造引擎。
func NewEngine(s *store.Store, models llm.ModelProvider, ai *llm.Client) *Engine {
	return &Engine{
		store:    s,
		models:   models,
		ai:       ai,
		sessions: make(map[string]*session),
		subs:     make(map[string]map[chan Event]struct{}),
	}
}

// Personalities 固定 5 人格。
func (e *Engine) Personalities() []dto.Personality {
	return personalities
}

// Create 创建会话（§11：participants≥2，max_rounds≤5）。
func (e *Engine) Create(in dto.CreateDebateRequest) (*model.DebateSession, *middleware.APIError) {
	if len(in.Participants) < 2 || len(in.Participants) > 5 {
		return nil, middleware.BadRequest("participants must be 2-5", map[string]string{"participants": "2-5 AI model IDs"})
	}
	if in.MaxRounds < 1 || in.MaxRounds > 5 {
		return nil, middleware.BadRequest("max_rounds must be 1-5", map[string]string{"max_rounds": "1-5"})
	}
	if in.Symbol == "" {
		return nil, middleware.BadRequest("symbol is required", map[string]string{"symbol": "required"})
	}
	// 校验模型存在
	seen := map[string]bool{}
	for _, mid := range in.Participants {
		if seen[mid] {
			return nil, middleware.BadRequest("duplicate participant model", map[string]string{"participants": "unique model ids"})
		}
		seen[mid] = true
		if _, ok := e.models.GetModel(mid); !ok {
			return nil, middleware.NotFound("model not found: " + mid)
		}
	}
	if in.IntervalMinutes < 1 {
		in.IntervalMinutes = 5
	}

	sess := &model.DebateSession{
		Name:            in.Name,
		StrategyID:      in.StrategyID,
		Status:          dto.DebatePending,
		Symbol:          in.Symbol,
		MaxRounds:       in.MaxRounds,
		IntervalMinutes: in.IntervalMinutes,
		PromptVariant:   in.PromptVariant,
		AutoExecute:     in.AutoExecute,
		TraderID:        in.TraderID,
	}
	e.store.CreateDebateSession(sess)

	// 参与者（人格按顺序分配，speak_order=索引）
	for i, mid := range in.Participants {
		m, ok := e.models.GetModel(mid)
		if !ok {
			continue
		}
		persona := personalities[i%len(personalities)]
		e.store.AddDebateParticipant(&model.DebateParticipant{
			SessionID:   sess.ID,
			AIModelID:   mid,
			AIModelName: m.ModelName,
			Provider:    m.Provider,
			Personality: persona.ID,
			Color:       persona.Color,
			SpeakOrder:  i,
		})
	}

	e.mu.Lock()
	e.sessions[sess.ID] = &session{meta: sess}
	e.mu.Unlock()

	created, _ := e.store.GetDebateSession(sess.ID)
	return created, nil
}

// List 会话列表。
func (e *Engine) List() []*model.DebateSession {
	return e.store.ListDebateSessions()
}

// Get 会话详情（含参与者/消息/投票）。
func (e *Engine) Get(id string) (*dto.SessionWithDetailsDTO, *middleware.APIError) {
	sess, ok := e.store.GetDebateSession(id)
	if !ok {
		return nil, middleware.NotFound("debate session not found")
	}
	out := &dto.SessionWithDetailsDTO{
		DebateSessionDTO: sessionToDTO(sess),
	}
	ps := e.store.ListDebateParticipants(id)
	out.Participants = make([]dto.DebateParticipantDTO, 0, len(ps))
	for _, p := range ps {
		out.Participants = append(out.Participants, participantToDTO(p))
	}
	ms := e.store.ListDebateMessages(id)
	out.Messages = make([]dto.DebateMessageDTO, 0, len(ms))
	for _, m := range ms {
		out.Messages = append(out.Messages, messageToDTO(m))
	}
	vs := e.store.ListDebateVotes(id)
	out.Votes = make([]dto.DebateVoteDTO, 0, len(vs))
	for _, v := range vs {
		out.Votes = append(out.Votes, voteToDTO(v))
	}
	return out, nil
}

// Start 启动辩论（pending→running）。
func (e *Engine) Start(id string) *middleware.APIError {
	s, apiErr := e.getSession(id)
	if apiErr != nil {
		return apiErr
	}
	s.mu.Lock()
	if s.meta.Status != dto.DebatePending {
		status := s.meta.Status
		s.mu.Unlock()
		return middleware.BadRequest("session not pending (status="+status+")", nil)
	}
	s.meta.Status = dto.DebateRunning
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)

	go e.debateLoop(ctx, id)
	return nil
}

// Cancel 取消（running/voting→cancelled）。
func (e *Engine) Cancel(id string) *middleware.APIError {
	s, apiErr := e.getSession(id)
	if apiErr != nil {
		return apiErr
	}
	s.mu.Lock()
	if s.meta.Status != dto.DebateRunning && s.meta.Status != dto.DebateVoting {
		status := s.meta.Status
		s.mu.Unlock()
		return middleware.BadRequest("session not active (status="+status+")", nil)
	}
	s.meta.Status = dto.DebateCancelled
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)
	e.broadcast(id, Event{Type: EventError, Data: map[string]string{"message": "debate cancelled"}})
	return nil
}

// Execute 共识执行（§11：仅 completed 且有 open_long/open_short 共识）。
// 真实下单由交易引擎（阶段 3d）接管；此处完成校验与记录。
func (e *Engine) Execute(id, traderID string) *middleware.APIError {
	s, apiErr := e.getSession(id)
	if apiErr != nil {
		return apiErr
	}
	s.mu.Lock()
	status := s.meta.Status
	consensus := s.consensus
	s.mu.Unlock()
	if status != dto.DebateCompleted {
		return middleware.DebateNotExecutable("debate not completed (status=" + status + ")")
	}
	if consensus == nil || (consensus.Action != model.ActionOpenLong && consensus.Action != model.ActionOpenShort) {
		return middleware.DebateNotExecutable("no open consensus in votes")
	}
	// 记录执行意图（trader 关联）
	s.mu.Lock()
	s.meta.TraderID = traderID
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)
	return nil
}

// Delete 删除会话（completed/cancelled 可删）。
func (e *Engine) Delete(id string) *middleware.APIError {
	s, apiErr := e.getSession(id)
	if apiErr != nil {
		return apiErr
	}
	s.mu.Lock()
	if s.meta.Status == dto.DebateRunning || s.meta.Status == dto.DebateVoting {
		s.mu.Unlock()
		return middleware.BadRequest("cannot delete an active debate", nil)
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	e.mu.Lock()
	delete(e.sessions, id)
	delete(e.subs, id)
	e.mu.Unlock()
	if !e.store.DeleteDebateSession(id) {
		return middleware.NotFound("debate session not found")
	}
	return nil
}

// Messages 消息列表。
func (e *Engine) Messages(id string) ([]*model.DebateMessage, *middleware.APIError) {
	if _, apiErr := e.getSession(id); apiErr != nil {
		return nil, apiErr
	}
	return e.store.ListDebateMessages(id), nil
}

// Votes 投票列表。
func (e *Engine) Votes(id string) ([]*model.DebateVote, *middleware.APIError) {
	if _, apiErr := e.getSession(id); apiErr != nil {
		return nil, apiErr
	}
	return e.store.ListDebateVotes(id), nil
}

// Subscribe 订阅会话事件流（SSE）。返回事件通道与退订函数。
func (e *Engine) Subscribe(id string) (<-chan Event, func(), *middleware.APIError) {
	if _, apiErr := e.getSession(id); apiErr != nil {
		return nil, nil, apiErr
	}
	ch := make(chan Event, 64)
	e.mu.Lock()
	if e.subs[id] == nil {
		e.subs[id] = make(map[chan Event]struct{})
	}
	e.subs[id][ch] = struct{}{}
	e.mu.Unlock()

	unsub := func() {
		e.mu.Lock()
		if m, ok := e.subs[id]; ok {
			delete(m, ch)
			if len(m) == 0 {
				delete(e.subs, id)
			}
		}
		e.mu.Unlock()
		close(ch)
	}
	return ch, unsub, nil
}

// broadcast 非阻塞广播（订阅者积压 64 条后丢弃该订阅者，避免阻塞辩论循环）。
func (e *Engine) broadcast(id string, ev Event) {
	e.mu.Lock()
	subs := e.subs[id]
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
	e.mu.Unlock()
}

// getSession 取会话运行控制。
func (e *Engine) getSession(id string) (*session, *middleware.APIError) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sessions[id]; ok {
		// 同步 store 元数据（进程内状态权威）
		if meta, exists := e.store.GetDebateSession(id); exists {
			s.mu.Lock()
			s.meta.Status = meta.Status
			s.meta.CurrentRound = meta.CurrentRound
			s.mu.Unlock()
		}
		return s, nil
	}
	if _, exists := e.store.GetDebateSession(id); exists {
		return &session{meta: &model.DebateSession{ID: id, Status: dto.DebateCompleted}}, nil
	}
	return nil, middleware.NotFound("debate session not found")
}

// setStatus 更新状态（store + 运行时）。
func (e *Engine) setStatus(id string, status string) {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.meta.Status = status
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)
}

// ========== DTO 转换 ==========

func sessionToDTO(s *model.DebateSession) dto.DebateSessionDTO {
	return dto.DebateSessionDTO{
		ID:              s.ID,
		Name:            s.Name,
		StrategyID:      s.StrategyID,
		Status:          s.Status,
		Symbol:          s.Symbol,
		MaxRounds:       s.MaxRounds,
		CurrentRound:    s.CurrentRound,
		IntervalMinutes: s.IntervalMinutes,
		PromptVariant:   s.PromptVariant,
		AutoExecute:     s.AutoExecute,
		TraderID:        s.TraderID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func participantToDTO(p *model.DebateParticipant) dto.DebateParticipantDTO {
	return dto.DebateParticipantDTO{
		ID:          p.ID,
		SessionID:   p.SessionID,
		AIModelID:   p.AIModelID,
		AIModelName: p.AIModelName,
		Provider:    p.Provider,
		Personality: p.Personality,
		Color:       p.Color,
		SpeakOrder:  p.SpeakOrder,
	}
}

func messageToDTO(m *model.DebateMessage) dto.DebateMessageDTO {
	return dto.DebateMessageDTO{
		ID:           m.ID,
		SessionID:    m.SessionID,
		Round:        m.Round,
		ParticipantID: m.ParticipantID,
		Personality:  m.Personality,
		AIModelName:  m.AIModelName,
		Content:      m.Content,
		Timestamp:    m.Timestamp,
	}
}

func voteToDTO(v *model.DebateVote) dto.DebateVoteDTO {
	return dto.DebateVoteDTO{
		SessionID:     v.SessionID,
		AIModelID:     v.AIModelID,
		Action:        v.Action,
		Symbol:        v.Symbol,
		Confidence:    v.Confidence,
		Leverage:      v.Leverage,
		PositionPct:   v.PositionPct,
		StopLossPct:   v.StopLossPct,
		TakeProfitPct: v.TakeProfitPct,
		Reasoning:     v.Reasoning,
	}
}
