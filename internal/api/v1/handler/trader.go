package handler

import (
	"errors"
	"io"
	"math"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// TraderHandler 交易员 CRUD + 状态机控制 + 持仓。
type TraderHandler struct {
	svc *service.TraderService
}

// NewTraderHandler 构造交易员处理器。
func NewTraderHandler(svc *service.TraderService) *TraderHandler {
	return &TraderHandler{svc: svc}
}

// List GET /traders?page=&size=&status=&fields=
func (h *TraderHandler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	fields := splitFields(q.Fields)

	all := h.svc.List()
	var filtered []*model.Trader
	for _, t := range all {
		if q.Status != "" && t.Status != q.Status {
			continue
		}
		filtered = append(filtered, t)
	}
	total := len(filtered)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	items := make([]dto.TraderDTO, 0, end-start)
	for _, t := range filtered[start:end] {
		items = append(items, traderToDTO(t))
	}
	if len(fields) > 0 {
		filteredItems := make([]any, 0, len(items))
		for _, item := range items {
			filteredItems = append(filteredItems, dto.FilterFields(item, fields))
		}
		middleware.WriteOK(c, dto.PaginatedData[any]{Items: filteredItems, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
		return
	}
	middleware.WriteOK(c, dto.PaginatedData[dto.TraderDTO]{Items: items, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// Get GET /traders/{id}
func (h *TraderHandler) Get(c *gin.Context) {
	t, apiErr := h.svc.Get(c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Create POST /traders
func (h *TraderHandler) Create(c *gin.Context) {
	var req dto.CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	t, apiErr := h.svc.Create(&req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Update PATCH /traders/{id}（部分更新）
func (h *TraderHandler) Update(c *gin.Context) {
	var req dto.UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	t, apiErr := h.svc.Update(c.Param("id"), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Start POST /traders/{id}/start（幂等 1204）
func (h *TraderHandler) Start(c *gin.Context) {
	if apiErr := h.svc.Start(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Pause POST /traders/{id}/pause
func (h *TraderHandler) Pause(c *gin.Context) {
	if apiErr := h.svc.Pause(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Resume POST /traders/{id}/resume
func (h *TraderHandler) Resume(c *gin.Context) {
	if apiErr := h.svc.Resume(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Stop POST /traders/{id}/stop
func (h *TraderHandler) Stop(c *gin.Context) {
	if apiErr := h.svc.Stop(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// ListPositions GET /positions?symbol=&fields=
func (h *TraderHandler) ListPositions(c *gin.Context) {
	var q dto.ListQuery
	_ = c.ShouldBindQuery(&q)
	fields := splitFields(q.Fields)

	positions := h.svc.ListPositions(q.Symbol)
	out := make([]dto.PositionDTO, 0, len(positions))
	for _, p := range positions {
		out = append(out, positionToDTO(p))
	}
	if len(fields) > 0 {
		filtered := make([]any, 0, len(out))
		for _, item := range out {
			filtered = append(filtered, dto.FilterFields(item, fields))
		}
		middleware.WriteOK(c, filtered)
		return
	}
	middleware.WriteOK(c, out)
}

// ClosePosition DELETE /positions/{id} 平仓。
// pnl 通过 JSON body 传入（如 {"pnl": 123.45}）——必须走 body 而非 query：
// 请求签名只覆盖 timestamp+path+body+nonce，query 不在保护范围，放 query 可被中间人篡改。
// L2：拒绝 NaN/Inf（ParseFloat 不报错，非有限值会永久污染聚合指标）。
// 真实引擎接入后由引擎自行结算 PnL，调用方可省略。
func (h *TraderHandler) ClosePosition(c *gin.Context) {
	pnl := 0.0
	var req struct {
		PnL *float64 `json:"pnl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		// 空 body 合法（pnl 默认 0）；EOF 之外才是坏 JSON
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	if req.PnL != nil {
		if math.IsNaN(*req.PnL) || math.IsInf(*req.PnL, 0) {
			middleware.WriteError(c, middleware.BadRequest("pnl must be a finite number", map[string]string{"pnl": "must be finite"}))
			return
		}
		pnl = *req.PnL
	}
	p, apiErr := h.svc.ClosePosition(c.Param("id"), pnl)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, positionToDTO(p))
}

// ========== DTO 转换 ==========

func traderToDTO(t *model.Trader) dto.TraderDTO {
	out := dto.TraderDTO{
		ID:         t.ID,
		Name:       t.Name,
		Exchange:   t.Exchange,
		StrategyID: t.StrategyID,
		Status:     t.Status,
		ModelConfig: dto.ModelConfig{
			Provider:   t.ModelConfig.Provider,
			ModelID:    t.ModelConfig.ModelID,
			Parameters: t.ModelConfig.Parameters,
		},
		RiskConfig: dto.RiskConfig{
			MaxPositionSize: t.RiskConfig.MaxPositionSize,
			StopLoss:        t.RiskConfig.StopLoss,
			TakeProfit:      t.RiskConfig.TakeProfit,
			MaxDailyLoss:    t.RiskConfig.MaxDailyLoss,
		},
		Metrics: dto.MetricsDTO{
			TotalPnL:   t.Metrics.TotalPnL,
			WinRate:    t.Metrics.WinRate,
			TradeCount: t.Metrics.TradeCount,
			DailyPnL:   t.Metrics.DailyPnL,
		},
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Schedule != nil {
		sch := &dto.Schedule{Interval: t.Schedule.Interval}
		for _, h := range t.Schedule.ActiveHours {
			sch.ActiveHours = append(sch.ActiveHours, dto.ActiveHours{Start: h.Start, End: h.End})
		}
		out.Schedule = sch
	}
	return out
}

func positionToDTO(p *model.Position) dto.PositionDTO {
	return dto.PositionDTO{
		ID:         p.ID,
		Symbol:     p.Symbol,
		Side:       p.Side,
		Size:       p.Size,
		EntryPrice: p.EntryPrice,
		PnL:        p.PnL,
		TraderID:   p.TraderID,
		OpenedAt:   p.OpenedAt,
		ClosedAt:   p.ClosedAt,
	}
}
