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
// List 交易员列表（分页 + 状态过滤 + fields 过滤）。
// @Summary 交易员列表
// @Description 分页查询交易员，支持 status 过滤与 fields 白名单
// @Tags traders
// @Produce json
// @Param page query int false "页码（1 起，默认 1，上限 100000）" default(1)
// @Param size query int false "每页数量（默认 20，上限 100）" default(20)
// @Param status query string false "按状态过滤（idle/running/paused/stopped/error）"
// @Param fields query string false "逗号分隔字段白名单"
// @Success 200 {object} dto.ApiResponse[dto.TraderPage]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /traders [get]
func (h *TraderHandler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	fields := splitFields(q.Fields)

	all := h.svc.List(currentUserID(c))
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
// Get 交易员详情。
// @Summary 交易员详情
// @Tags traders
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在"
// @Router /traders/{id} [get]
func (h *TraderHandler) Get(c *gin.Context) {
	t, apiErr := h.svc.Get(c.Param("id"), currentUserID(c))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Create POST /traders
// Create 创建交易员（写操作需签名头）。
// @Summary 创建交易员
// @Description 引用已存在的模型；初始状态 idle
// @Tags traders
// @Accept json
// @Produce json
// @Param body body dto.CreateTraderRequest true "交易员配置"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 引用模型不存在 / 签名缺失"
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /traders [post]
func (h *TraderHandler) Create(c *gin.Context) {
	var req dto.CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	t, apiErr := h.svc.Create(currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Update PATCH /traders/{id}（部分更新）
// Update 部分更新（PATCH；running 状态禁止修改配置）。
// @Summary 更新交易员
// @Description 指针字段语义：缺省不修改，显式 null 清空；running 时拒绝
// @Tags traders
// @Accept json
// @Produce json
// @Param id path string true "交易员 ID"
// @Param body body dto.UpdateTraderRequest true "部分字段"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / running 中不可修改"
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在"
// @Router /traders/{id} [patch]
func (h *TraderHandler) Update(c *gin.Context) {
	var req dto.UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	t, apiErr := h.svc.Update(c.Param("id"), currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, traderToDTO(t))
}

// Start POST /traders/{id}/start（幂等 1204）
// Start 启动交易员（幂等：已在 running 返回 1204）。
// @Summary 启动交易员
// @Tags traders
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在 / 非法迁移"
// @Failure 409 {object} dto.ErrorResponse "1204 已在运行"
// @Router /traders/{id}/start [post]
func (h *TraderHandler) Start(c *gin.Context) {
	if apiErr := h.svc.Start(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Pause POST /traders/{id}/pause
// Pause 暂停交易员（仅 running 可暂停）。
// @Summary 暂停交易员
// @Tags traders
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 已处于 paused"
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在 / 非法迁移"
// @Router /traders/{id}/pause [post]
func (h *TraderHandler) Pause(c *gin.Context) {
	if apiErr := h.svc.Pause(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Resume POST /traders/{id}/resume
// Resume 恢复运行（仅 paused 可恢复）。
// @Summary 恢复交易员
// @Tags traders
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在 / 非法迁移"
// @Failure 409 {object} dto.ErrorResponse "1204 已在运行"
// @Router /traders/{id}/resume [post]
func (h *TraderHandler) Resume(c *gin.Context) {
	if apiErr := h.svc.Resume(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Stop POST /traders/{id}/stop
// Stop 停止交易员（running/paused 可停止；停止后可重启）。
// @Summary 停止交易员
// @Tags traders
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 已处于 stopped"
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在 / 非法迁移"
// @Router /traders/{id}/stop [post]
func (h *TraderHandler) Stop(c *gin.Context) {
	if apiErr := h.svc.Stop(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// ListPositions GET /positions?symbol=&fields=
// ListPositions 持仓列表（symbol/fields 过滤）。
// @Summary 持仓列表
// @Description 当前未平仓持仓，支持 symbol 与 fields 过滤
// @Tags positions
// @Produce json
// @Param symbol query string false "按交易对过滤，如 BTC-USDT"
// @Param fields query string false "逗号分隔字段白名单"
// @Success 200 {object} dto.ApiResponse[dto.PositionDTOList]
// @Router /positions [get]
func (h *TraderHandler) ListPositions(c *gin.Context) {
	var q dto.ListQuery
	_ = c.ShouldBindQuery(&q)
	fields := splitFields(q.Fields)

	positions := h.svc.ListPositions(currentUserID(c), q.Symbol)
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
// ClosePosition 平仓（pnl 走 body；空 body 视为 pnl=0）。
// @Summary 平仓
// @Description pnl 通过 JSON body 传入（{"pnl":123.45}）；body 为空视为 0。
// 签名覆盖 body，pnl 不可被中间人篡改
// @Tags positions
// @Accept json
// @Produce json
// @Param id path string true "持仓 ID"
// @Param body body dto.ClosePositionRequest false "平仓盈亏（可选）"
// @Success 200 {object} dto.ApiResponse[dto.PositionDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 已平仓"
// @Failure 404 {object} dto.ErrorResponse "1004 持仓不存在"
// @Router /positions/{id} [delete]
func (h *TraderHandler) ClosePosition(c *gin.Context) {
	pnl := 0.0
	var req dto.ClosePositionRequest
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
	p, apiErr := h.svc.ClosePosition(currentUserID(c), c.Param("id"), pnl)
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
		ID:            p.ID,
		TraderID:      p.TraderID,
		ExchangeID:    p.ExchangeID,
		Symbol:        p.Symbol,
		Side:          p.Side,
		Size:          p.Size,
		EntryPrice:    p.EntryPrice,
		PnL:           p.PnL,
		OpenedAt:      p.OpenedAt,
		ClosedAt:      p.ClosedAt,
		EntryQuantity: p.EntryQuantity,
		Quantity:      p.Quantity,
		MarkPrice:     p.MarkPrice,
		UnrealizedPnL: p.UnrealizedPnL,
		Leverage:      p.Leverage,
		Status:        p.Status,
		EntryTime:     p.EntryTime,
		ExitTime:      p.ExitTime,
		ExitPrice:     p.ExitPrice,
		RealizedPnL:   p.RealizedPnL,
		Fee:           p.Fee,
		CloseReason:   p.CloseReason,
		Source:        p.Source,
	}
}
