package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/debate"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// DebateHandler 辩论竞技场 10 端点 + SSE（API设计.md §11）。
type DebateHandler struct {
	engine *debate.Engine
}

// NewDebateHandler 构造辩论处理器。
func NewDebateHandler(engine *debate.Engine) *DebateHandler {
	return &DebateHandler{engine: engine}
}

// List GET /debates
// List 辩论会话列表。
// @Summary 辩论会话列表
// @Tags debates
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]dto.DebateSessionDTO]
// @Router /debates [get]
func (h *DebateHandler) List(c *gin.Context) {
	sessions := h.engine.List(currentUserID(c))
	out := make([]dto.DebateSessionDTO, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, debateSessionToDTO(s))
	}
	middleware.WriteOK(c, out)
}

// Personalities GET /debates/personalities
// Personalities 固定 5 人格。
// @Summary 辩论人格
// @Tags debates
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]dto.Personality]
// @Router /debates/personalities [get]
func (h *DebateHandler) Personalities(c *gin.Context) {
	middleware.WriteOK(c, h.engine.Personalities())
}

// Create POST /debates
// Create 创建辩论会话（participants 2-5，max_rounds ≤5）。
// @Summary 创建辩论
// @Tags debates
// @Accept json
// @Produce json
// @Param body body dto.CreateDebateRequest true "辩论配置"
// @Success 200 {object} dto.ApiResponse[dto.DebateSessionDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Router /debates [post]
func (h *DebateHandler) Create(c *gin.Context) {
	var req dto.CreateDebateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	sess, apiErr := h.engine.Create(currentUserID(c), req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, debateSessionToDTO(sess))
}

// Get GET /debates/{id}
// Get 会话详情（含参与者/消息/投票）。
// @Summary 辩论详情
// @Tags debates
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.ApiResponse[dto.SessionWithDetailsDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 会话不存在"
// @Router /debates/{id} [get]
func (h *DebateHandler) Get(c *gin.Context) {
	detail, apiErr := h.engine.Get(c.Param("id"), currentUserID(c))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, detail)
}

// Control POST /debates/{id}/start|cancel
// Control 启动/取消辩论。
// @Summary 启动或取消辩论
// @Tags debates
// @Produce json
// @Param id path string true "会话 ID"
// @Param action path string true "start|cancel"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 400 {object} dto.ErrorResponse "1001 状态非法"
// @Router /debates/{id}/{action} [post]
func (h *DebateHandler) Control(c *gin.Context) {
	id := c.Param("id")
	var apiErr *middleware.APIError
	switch c.Param("action") {
	case "start":
		apiErr = h.engine.Start(currentUserID(c), id)
	case "cancel":
		apiErr = h.engine.Cancel(currentUserID(c), id)
	default:
		middleware.WriteError(c, middleware.BadRequest("invalid action", map[string]string{"action": "start|cancel"}))
		return
	}
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	// 返回控制后的会话详情（对齐前端 Promise<DebateSession>：start/cancel 后状态对前端有用）
	detail, apiErr2 := h.engine.Get(id, currentUserID(c))
	if apiErr2 != nil {
		middleware.WriteError(c, apiErr2)
		return
	}
	middleware.WriteOK(c, detail)
}

// Execute POST /debates/{id}/execute
// Execute 共识执行（仅 completed 且有 open 共识）。
// @Summary 执行辩论共识
// @Tags debates
// @Accept json
// @Produce json
// @Param id path string true "会话 ID"
// @Param body body dto.DebateExecuteRequest true "目标交易员"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 409 {object} dto.ErrorResponse "1420 不可执行"
// @Router /debates/{id}/execute [post]
func (h *DebateHandler) Execute(c *gin.Context) {
	var req dto.DebateExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	if _, apiErr := h.engine.Get(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	if apiErr := h.engine.Execute(c.Param("id"), req.TraderID); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	// 对齐前端 executeDebate 声明 {executed, message}：共识执行意图已记录（实际开仓由引擎周期跑）
	middleware.WriteOK(c, map[string]any{"executed": true, "message": "共识执行已记录"})
}

// Delete DELETE /debates/{id}
// Delete 删除会话。
// @Summary 删除辩论
// @Tags debates
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 400 {object} dto.ErrorResponse "1001 活跃会话不可删"
// @Router /debates/{id} [delete]
func (h *DebateHandler) Delete(c *gin.Context) {
	if _, apiErr := h.engine.Get(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	if apiErr := h.engine.Delete(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Messages GET /debates/{id}/messages
// Messages 消息列表。
// @Summary 辩论消息
// @Tags debates
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.ApiResponse[[]dto.DebateMessageDTO]
// @Router /debates/{id}/messages [get]
func (h *DebateHandler) Messages(c *gin.Context) {
	if _, apiErr := h.engine.Get(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	msgs, apiErr := h.engine.Messages(c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	out := make([]dto.DebateMessageDTO, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, debateMessageToDTO(m))
	}
	middleware.WriteOK(c, out)
}

// Votes GET /debates/{id}/votes
// Votes 投票列表。
// @Summary 辩论投票
// @Tags debates
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.ApiResponse[[]dto.DebateVoteDTO]
// @Router /debates/{id}/votes [get]
func (h *DebateHandler) Votes(c *gin.Context) {
	if _, apiErr := h.engine.Get(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	votes, apiErr := h.engine.Votes(c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	out := make([]dto.DebateVoteDTO, 0, len(votes))
	for _, v := range votes {
		out = append(out, debateVoteToDTO(v))
	}
	middleware.WriteOK(c, out)
}

// Stream GET /debates/{id}/stream
// Stream SSE 事件流（initial/round_start/message/round_end/vote/consensus/error + 心跳 15s）。
// @Summary 辩论 SSE 流
// @Tags debates
// @Produce text/event-stream
// @Param id path string true "会话 ID"
// @Success 200 {object} string
// @Router /debates/{id}/stream [get]
func (h *DebateHandler) Stream(c *gin.Context) {
	id := c.Param("id")
	// 先校验归属再订阅：非属主订阅后 initial 404 但 SSE 继续推流（P0-2）
	if _, apiErr := h.engine.Get(id, currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	ch, unsub, apiErr := h.engine.Subscribe(id)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	defer unsub()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// initial：当前会话快照
	if detail, err := h.engine.Get(id, currentUserID(c)); err == nil {
		writeSSE(c, "initial", detail)
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(c, ev.Type, ev.Data)
		case <-ticker.C:
			_, _ = c.Writer.WriteString(": heartbeat\n\n")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// writeSSE 写事件帧。
func writeSSE(c *gin.Context, eventType string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, raw)
	c.Writer.Flush()
}

// ========== DTO 转换 ==========

func debateSessionToDTO(s *model.DebateSession) dto.DebateSessionDTO {
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

func debateMessageToDTO(m *model.DebateMessage) dto.DebateMessageDTO {
	return dto.DebateMessageDTO{
		ID:            m.ID,
		SessionID:     m.SessionID,
		Round:         m.Round,
		ParticipantID: m.ParticipantID,
		Personality:   m.Personality,
		AIModelName:   m.AIModelName,
		Content:       m.Content,
		Timestamp:     m.Timestamp,
	}
}

func debateVoteToDTO(v *model.DebateVote) dto.DebateVoteDTO {
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
