package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// DataHandler 数据/市场只读端点（API设计.md §11 data + market 路由）。
type DataHandler struct {
	svc *service.DataService
}

// NewDataHandler 构造数据处理 handler。
func NewDataHandler(svc *service.DataService) *DataHandler {
	return &DataHandler{svc: svc}
}

// Status GET /status?trader_id=
// Status 交易员运行状态。
// @Summary 运行状态
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.StatusDTO]
// @Router /status [get]
func (h *DataHandler) Status(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Status(c.Query("trader_id")))
}

// Account GET /account?trader_id=
// Account 账户信息（权益/可用/持仓数）。
// @Summary 账户信息
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.AccountInfoDTO]
// @Router /account [get]
func (h *DataHandler) Account(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Account(c.Query("trader_id")))
}

// Decisions GET /decisions?trader_id=&page=&size=
// Decisions 决策记录分页（含 cot_trace / 原始响应）。
// @Summary 决策记录
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /decisions [get]
func (h *DataHandler) Decisions(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	traderID := c.Query("trader_id")
	items, total := h.svc.Decisions(traderID, page, size)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	anyItems := make([]any, 0, len(items))
	for _, d := range items {
		anyItems = append(anyItems, d)
	}
	middleware.WriteOK(c, dto.PaginatedData[any]{Items: anyItems, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// LatestDecision GET /decisions/latest?trader_id=
// LatestDecision 最近一轮决策。
// @Summary 最近决策
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1004 无决策记录"
// @Router /decisions/latest [get]
func (h *DataHandler) LatestDecision(c *gin.Context) {
	d, ok := h.svc.LatestDecision(c.Query("trader_id"))
	if !ok {
		middleware.WriteError(c, middleware.NotFound("no decision record"))
		return
	}
	middleware.WriteOK(c, d)
}

// Statistics GET /statistics?trader_id=
// Statistics 交易统计（胜率/盈亏因子/均值）。
// @Summary 交易统计
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.StatisticsDTO]
// @Router /statistics [get]
func (h *DataHandler) Statistics(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Statistics(c.Query("trader_id")))
}

// Trades GET /trades?trader_id=&page=&size=
// Trades 成交事件分页。
// @Summary 成交记录
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /trades [get]
func (h *DataHandler) Trades(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	traderID := c.Query("trader_id")
	items := h.svc.Trades(traderID, page, size)
	total := h.svc.CountTrades(traderID)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	out := make([]dto.TradeEventDTO, 0, len(items))
	for _, f := range items {
		out = append(out, fillToTradeDTO(f))
	}
	middleware.WriteOK(c, dto.PaginatedData[dto.TradeEventDTO]{Items: out, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// Orders GET /orders?trader_id=&page=&size=
// Orders 订单分页。
// @Summary 订单列表
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /orders [get]
func (h *DataHandler) Orders(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	traderID := c.Query("trader_id")
	items := h.svc.Orders(traderID, page, size)
	total := h.svc.CountOrders(traderID)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	out := make([]dto.OrderDTO, 0, len(items))
	for _, o := range items {
		out = append(out, orderToDTO(o))
	}
	middleware.WriteOK(c, dto.PaginatedData[dto.OrderDTO]{Items: out, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// OrderFills GET /orders/{id}/fills
// OrderFills 订单成交明细。
// @Summary 订单成交
// @Tags data
// @Produce json
// @Param id path string true "订单 ID"
// @Success 200 {object} dto.ApiResponse[[]dto.FillDTO]
// @Router /orders/{id}/fills [get]
func (h *DataHandler) OrderFills(c *gin.Context) {
	fills := h.svc.OrderFills(c.Param("id"))
	out := make([]dto.FillDTO, 0, len(fills))
	for _, f := range fills {
		out = append(out, fillToDTO(f))
	}
	middleware.WriteOK(c, out)
}

// OpenOrders GET /open-orders?trader_id=
// OpenOrders 交易所实时挂单。骨架：引擎层（阶段 3）接入交易所后填充。
// @Summary 实时挂单
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[[]dto.OrderDTO]
// @Router /open-orders [get]
func (h *DataHandler) OpenOrders(c *gin.Context) {
	orders := h.svc.OpenOrders(c.Query("trader_id"))
	out := make([]dto.OrderDTO, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToDTO(o))
	}
	middleware.WriteOK(c, out)
}

// Klines GET /klines?symbol=&interval=&limit=
// Klines K 线行情。骨架：数据源链（阶段 3）就位后填充。
// @Summary K 线行情
// @Tags market
// @Produce json
// @Param symbol query string true "交易对，如 BTC-USDT"
// @Param interval query string false "周期，如 15m" default(15m)
// @Param limit query int false "数量" default(200)
// @Success 200 {object} dto.ApiResponse[[]dto.KlineDTO]
// @Router /klines [get]
func (h *DataHandler) Klines(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		middleware.WriteError(c, middleware.BadRequest("symbol is required", map[string]string{"symbol": "required"}))
		return
	}
	interval := c.DefaultQuery("interval", "15m")
	limit := 200
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	klines, err := h.svc.Klines(symbol, interval, limit)
	if err != nil {
		middleware.WriteError(c, middleware.NewAPIError(middleware.CodeExchangeFailed, 400, "fetch klines failed: "+err.Error()))
		return
	}
	middleware.WriteOK(c, klines)
}

// Symbols GET /symbols
// Symbols 交易对列表。骨架：provider 就位后填充。
// @Summary 交易对列表
// @Tags market
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]string]
// @Router /symbols [get]
func (h *DataHandler) Symbols(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Symbols())
}

// PositionHistory GET /positions/history?trader_id=&page=&size=
// PositionHistory 平仓历史（分页）。
// @Summary 平仓历史
// @Tags data
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Param symbol query string false "按币对过滤"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /positions/history [get]
func (h *DataHandler) PositionHistory(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	traderID := c.Query("trader_id")
	items := h.svc.Store().ListPositionHistory(traderID, q.Symbol, (page-1)*size, size)
	total := h.svc.Store().CountPositionHistory(traderID, q.Symbol)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	out := make([]any, 0, len(items))
	for _, p := range items {
		out = append(out, p)
	}
	middleware.WriteOK(c, dto.PaginatedData[any]{Items: out, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// EquityHistory GET /equity-history?trader_id=
// EquityHistory 权益曲线。
// @Summary 权益曲线
// @Tags market
// @Produce json
// @Param trader_id query string false "交易员 ID"
// @Success 200 {object} dto.ApiResponse[[]dto.EquityPointDTO]
// @Router /equity-history [get]
func (h *DataHandler) EquityHistory(c *gin.Context) {
	snapshots := h.svc.EquityHistory(c.Query("trader_id"))
	out := make([]dto.EquityPointDTO, 0, len(snapshots))
	for _, e := range snapshots {
		out = append(out, equityToDTO(e))
	}
	middleware.WriteOK(c, out)
}

// EquityHistoryBatch POST /equity-history-batch
// EquityHistoryBatch 多交易员权益（批量）。
// @Summary 批量权益曲线
// @Tags market
// @Accept json
// @Produce json
// @Param body body dto.EquityBatchRequest true "trader_ids 列表"
// @Success 200 {object} dto.ApiResponse[[][]dto.EquityPointDTO]
// @Router /equity-history-batch [post]
func (h *DataHandler) EquityHistoryBatch(c *gin.Context) {
	var req dto.EquityBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	out := make([][]dto.EquityPointDTO, 0, len(req.TraderIDs))
	for _, id := range req.TraderIDs {
		snapshots := h.svc.EquityHistory(id)
		pts := make([]dto.EquityPointDTO, 0, len(snapshots))
		for _, e := range snapshots {
			pts = append(pts, equityToDTO(e))
		}
		out = append(out, pts)
	}
	middleware.WriteOK(c, out)
}

// Competition GET /competition
// Competition 公开市场数据。骨架：引擎层就位后填充。
// @Summary 公开市场
// @Tags market
// @Produce json
// @Success 200 {object} dto.ApiResponse[any]
// @Router /competition [get]
func (h *DataHandler) Competition(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Competition())
}

// TopTraders GET /top-traders
// TopTraders 排行榜。骨架：引擎层就位后填充。
// @Summary 交易员排行榜
// @Tags market
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]any]
// @Router /top-traders [get]
func (h *DataHandler) TopTraders(c *gin.Context) {
	traders := h.svc.TopTraders()
	out := make([]any, 0, len(traders))
	for _, t := range traders {
		out = append(out, t)
	}
	middleware.WriteOK(c, out)
}

// TraderPublicConfig GET /traders/{id}/public-config
// TraderPublicConfig 脱敏公开配置。
// @Summary 交易员公开配置
// @Tags market
// @Produce json
// @Param id path string true "交易员 ID"
// @Success 200 {object} dto.ApiResponse[dto.TraderConfigDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 交易员不存在"
// @Router /traders/{id}/public-config [get]
func (h *DataHandler) TraderPublicConfig(c *gin.Context) {
	t, apiErr := h.svc.TraderPublicConfig(c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, dto.TraderConfigDTO{
		Name:     t.Name,
		Exchange: t.Exchange,
		StrategyID: t.StrategyID,
	})
}

// ========== DTO 转换 ==========

func fillToTradeDTO(f *model.Fill) dto.TradeEventDTO {
	return dto.TradeEventDTO{
		ID:          f.ID,
		TraderID:    f.TraderID,
		OrderID:     f.OrderID,
		Symbol:      f.Symbol,
		Side:        f.Side,
		Quantity:    f.Quantity,
		Price:       f.Price,
		Fee:         f.Commission,
		FeeAsset:    f.CommissionAsset,
		RealizedPnL: f.RealizedPnL,
		IsMaker:     f.IsMaker,
		CreatedAt:   f.CreatedAt,
	}
}

func fillToDTO(f *model.Fill) dto.FillDTO {
	return dto.FillDTO{
		ID:              f.ID,
		TraderID:        f.TraderID,
		OrderID:         f.OrderID,
		ExchangeTradeID: f.ExchangeTradeID,
		Symbol:          f.Symbol,
		Side:            f.Side,
		Price:           f.Price,
		Quantity:        f.Quantity,
		QuoteQuantity:   f.QuoteQuantity,
		Commission:      f.Commission,
		CommissionAsset: f.CommissionAsset,
		RealizedPnL:     f.RealizedPnL,
		IsMaker:         f.IsMaker,
		CreatedAt:       f.CreatedAt,
	}
}

func orderToDTO(o *model.Order) dto.OrderDTO {
	return dto.OrderDTO{
		ID:              o.ID,
		TraderID:        o.TraderID,
		ExchangeID:      o.ExchangeID,
		ExchangeOrderID: o.ExchangeOrderID,
		ClientOrderID:   o.ClientOrderID,
		Symbol:          o.Symbol,
		Side:            o.Side,
		PositionSide:    o.PositionSide,
		Type:            o.Type,
		TimeInForce:     o.TimeInForce,
		Quantity:        o.Quantity,
		Price:           o.Price,
		StopPrice:       o.StopPrice,
		Status:          o.Status,
		FilledQuantity:  o.FilledQuantity,
		AvgFillPrice:    o.AvgFillPrice,
		Commission:      o.Commission,
		CommissionAsset: o.CommissionAsset,
		Leverage:        o.Leverage,
		ReduceOnly:      o.ReduceOnly,
		CreatedAt:       o.CreatedAt,
	}
}

func equityToDTO(e *model.EquitySnapshot) dto.EquityPointDTO {
	return dto.EquityPointDTO{
		Timestamp:   e.Timestamp.Unix(),
		Equity:      e.TotalEquity,
		Balance:     e.Balance,
		PnL:         e.UnrealizedPnL,
		DrawdownPct: e.MarginUsedPct,
	}
}
