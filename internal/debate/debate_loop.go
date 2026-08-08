package debate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/kernel"
	"fxcore/internal/llm"
	"fxcore/internal/model"
)

// debateLoop 辩论主循环：逐轮发言 → 投票 → 共识（§11 行为契约）。
func (e *Engine) debateLoop(ctx context.Context, id string) {
	s, _ := e.getSession(id)
	s.mu.Lock()
	maxRounds := s.meta.MaxRounds
	symbol := s.meta.Symbol
	s.mu.Unlock()

	participants := e.store.ListDebateParticipants(id)
	if len(participants) < 2 {
		e.broadcast(id, Event{Type: EventError, Data: map[string]string{"message": "insufficient participants"}})
		e.setStatus(id, dto.DebateCancelled)
		return
	}
	e.broadcast(id, Event{Type: EventInitial, Data: map[string]string{"session_id": id, "symbol": symbol, "participants": fmt.Sprintf("%d", len(participants))}})

	history := make([]llm.Message, 0, 16)
	for round := 1; round <= maxRounds; round++ {
		if ctx.Err() != nil {
			return
		}
		e.setRound(id, round)
		e.broadcast(id, Event{Type: EventRoundStart, Data: map[string]any{"round": round, "max_rounds": maxRounds}})

		for _, p := range participants {
			if ctx.Err() != nil {
				return
			}
			prompt := buildSpeakerPrompt(p, symbol, round, history)
			content, err := e.speak(ctx, p, prompt)
			if err != nil {
				content = fmt.Sprintf("[%s unavailable] %v", p.Personality, err)
			}
			msg := &model.DebateMessage{
				SessionID:     id,
				Round:         round,
				ParticipantID: p.ID,
				Personality:   p.Personality,
				AIModelName:   p.AIModelName,
				Content:       content,
			}
			e.store.AddDebateMessage(msg)
			e.broadcast(id, Event{Type: EventMessage, Data: messageToDTO(msg)})
			history = append(history, llm.Message{Role: "assistant", Content: fmt.Sprintf("[%s/%s] %s", p.Personality, p.AIModelName, content)})
		}
		e.broadcast(id, Event{Type: EventRoundEnd, Data: map[string]any{"round": round}})
	}

	// ---- 投票阶段 ----
	e.setStatus(id, dto.DebateVoting)
	e.broadcast(id, Event{Type: EventVote, Data: map[string]string{"phase": "start"}})
	for _, p := range participants {
		if ctx.Err() != nil {
			return
		}
		vote, err := e.castVote(ctx, p, symbol)
		if err != nil {
			continue // 单票失败不中断
		}
		v := &model.DebateVote{
			SessionID:     id,
			AIModelID:     p.AIModelID,
			Action:        vote.Action,
			Symbol:        symbol,
			Confidence:    vote.Confidence,
			Leverage:      vote.Leverage,
			PositionPct:   vote.PositionPct,
			StopLossPct:   vote.StopLossPct,
			TakeProfitPct: vote.TakeProfitPct,
			Reasoning:     vote.Reasoning,
		}
		e.store.AddDebateVote(v)
		e.broadcast(id, Event{Type: EventVote, Data: voteToDTO(v)})
	}

	// ---- 共识 ----
	consensus := e.aggregate(id)
	s.mu.Lock()
	s.consensus = consensus
	s.meta.Status = dto.DebateCompleted
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)
	e.broadcast(id, Event{Type: EventConsensus, Data: consensus})

	// 自动执行（仅 open 共识）
	if s.meta.AutoExecute && consensus != nil && (consensus.Action == model.ActionOpenLong || consensus.Action == model.ActionOpenShort) {
		s.mu.Lock()
		traderID := s.meta.TraderID
		s.mu.Unlock()
		if traderID != "" {
			_ = e.Execute(id, traderID)
		}
	}
}

// speak 单次发言。
func (e *Engine) speak(ctx context.Context, p *model.DebateParticipant, prompt string) (string, error) {
	m, ok := e.models.GetModel(p.AIModelID)
	if !ok {
		return "", fmt.Errorf("model not found")
	}
	res, err := e.ai.Chat(ctx, m, llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Content), nil
}

// castVote 单票。
func (e *Engine) castVote(ctx context.Context, p *model.DebateParticipant, symbol string) (*dto.DebateVoteDTO, error) {
	m, ok := e.models.GetModel(p.AIModelID)
	if !ok {
		return nil, fmt.Errorf("model not found")
	}
	prompt := fmt.Sprintf(`You are the %s in a crypto debate on %s. Cast your final vote as STRICT JSON object only:
{"action":"open_long|open_short|close_long|close_short|hold|wait","symbol":"%s","confidence":0-100,"leverage":1-20,"position_pct":0-100,"stop_loss_pct":0-10,"take_profit_pct":0-50,"reasoning":"one sentence"}`, p.Personality, symbol, symbol)
	res, err := e.ai.Chat(ctx, m, llm.ChatRequest{
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   512,
	})
	if err != nil {
		return nil, err
	}
	body := extractJSONObject(res.Content)
	if body == "" {
		return nil, fmt.Errorf("no vote object in output")
	}
	body = kernel.FixFullWidth(body)
	var v struct {
		Action        string  `json:"action"`
		Symbol        string  `json:"symbol"`
		Confidence    float64 `json:"confidence"`
		Leverage      int     `json:"leverage"`
		PositionPct   float64 `json:"position_pct"`
		StopLossPct   float64 `json:"stop_loss_pct"`
		TakeProfitPct float64 `json:"take_profit_pct"`
		Reasoning     string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return nil, err
	}
	if !kernel.ValidActions[v.Action] {
		return nil, fmt.Errorf("invalid vote action %q", v.Action)
	}
	if v.Symbol == "" {
		v.Symbol = symbol
	}
	return &dto.DebateVoteDTO{
		SessionID:     p.SessionID,
		AIModelID:     p.AIModelID,
		Action:        v.Action,
		Symbol:        v.Symbol,
		Confidence:    v.Confidence,
		Leverage:      v.Leverage,
		PositionPct:   v.PositionPct,
		StopLossPct:   v.StopLossPct,
		TakeProfitPct: v.TakeProfitPct,
		Reasoning:     v.Reasoning,
	}, nil
}

// aggregate 共识聚合：加权多数（置信为权重），平票取 wait。
func (e *Engine) aggregate(id string) *dto.DebateVoteDTO {
	votes := e.store.ListDebateVotes(id)
	if len(votes) == 0 {
		return nil
	}
	type acc struct {
		weight  float64
		sample  *model.DebateVote
	}
	counts := map[string]*acc{}
	for _, v := range votes {
		w := v.Confidence
		if w <= 0 {
			w = 50
		}
		a, ok := counts[v.Action]
		if !ok {
			counts[v.Action] = &acc{weight: w, sample: v}
			continue
		}
		a.weight += w
	}
	var best string
	var bestWeight float64
	for action, a := range counts {
		if a.weight > bestWeight {
			best, bestWeight = action, a.weight
		}
	}
	if best == "" || best == model.ActionWait || best == model.ActionHold {
		return &dto.DebateVoteDTO{SessionID: id, Action: model.ActionWait, Confidence: 50}
	}
	sample := counts[best].sample
	return &dto.DebateVoteDTO{
		SessionID:     id,
		AIModelID:     "consensus",
		Action:        best,
		Symbol:        sample.Symbol,
		Confidence:    bestWeight / float64(len(votes)),
		Leverage:      sample.Leverage,
		PositionPct:   sample.PositionPct,
		StopLossPct:   sample.StopLossPct,
		TakeProfitPct: sample.TakeProfitPct,
		Reasoning:     "weighted majority of participant votes",
	}
}

// setRound 更新当前轮次。
func (e *Engine) setRound(id string, round int) {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.meta.CurrentRound = round
	s.mu.Unlock()
	e.store.UpdateDebateSession(s.meta)
}

// buildSpeakerPrompt 发言提示词（人格 + 历史上下文）。
func buildSpeakerPrompt(p *model.DebateParticipant, symbol string, round int, history []llm.Message) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are a %s in a crypto futures trading debate on %s.\n", p.Personality, symbol))
	for _, h := range personalities {
		if h.ID == p.Personality {
			b.WriteString("Role: " + h.Description + "\n")
		}
	}
	fmt.Fprintf(&b, "Round %d. Speak with your personality, give a concrete directional view with reasoning.\n", round)
	if len(history) > 0 {
		b.WriteString("Previous speakers:\n")
		start := len(history) - 6
		if start < 0 {
			start = 0
		}
		for _, m := range history[start:] {
			b.WriteString(m.Content + "\n")
		}
	}
	b.WriteString("Keep it under 120 words. No JSON, plain prose.")
	return b.String()
}

// extractJSONObject 提取 JSON 对象主体（fence 或首个 { 到末个 }）。
func extractJSONObject(raw string) string {
	if i := strings.Index(raw, "```json"); i >= 0 {
		rest := raw[i+len("```json"):]
		if j := strings.Index(rest, "```"); j >= 0 {
			raw = rest[:j]
		}
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

var _ = time.Now
