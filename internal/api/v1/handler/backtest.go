package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/backtest"
	"fxcore/internal/middleware"
	"fxcore/internal/pkg/jwt"
)

// BacktestHandler 回测 15 端点（API设计.md §11）。
type BacktestHandler struct {
	engine *backtest.Engine
}

// authorizeRun 校验 run 归属当前用户（多用户隔离；越权/不存在 → 404）。
func (h *BacktestHandler) authorizeRun(c *gin.Context, runID string) bool {
	if !h.engine.OwnsRun(runID, currentUserID(c)) {
		middleware.WriteError(c, middleware.NotFound("backtest run not found"))
		return false
	}
	return true
}

// NewBacktestHandler 构造回测处理器。
func NewBacktestHandler(engine *backtest.Engine) *BacktestHandler {
	return &BacktestHandler{engine: engine}
}

// Start POST /backtest/start
// Start 启动回测。
// @Summary 启动回测
// @Tags backtest
// @Accept json
// @Produce json
// @Param body body dto.StartBacktestRequest true "回测配置"
// @Success 200 {object} dto.ApiResponse[dto.RunMetadata]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 1411 已有运行中回测"
// @Router /backtest/start [post]
func (h *BacktestHandler) Start(c *gin.Context) {
	var req dto.StartBacktestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	meta, apiErr := h.engine.Start(currentUserID(c), req.Config)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, meta)
}

// Control POST /backtest/{pause|resume|stop|label|delete}
// Control 回测控制（pause/resume/stop/label/delete）。
// @Summary 回测控制
// @Tags backtest
// @Accept json
// @Produce json
// @Param action path string true "pause|resume|stop|label|delete"
// @Param body body dto.BacktestControlRequest true "run_id（label 时含 label）"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1412 run 不存在"
// @Router /backtest/{action} [post]
func (h *BacktestHandler) Control(c *gin.Context) {
	var req dto.BacktestControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	switch c.Param("action") {
	case "pause":
		if !h.authorizeRun(c, req.RunID) {
		return
	}
	meta, apiErr := h.engine.Pause(req.RunID)
		if apiErr != nil {
			middleware.WriteError(c, apiErr)
			return
		}
		middleware.WriteOK(c, meta)
	case "resume":
		if !h.authorizeRun(c, req.RunID) {
		return
	}
	meta, apiErr := h.engine.Resume(req.RunID)
		if apiErr != nil {
			middleware.WriteError(c, apiErr)
			return
		}
		middleware.WriteOK(c, meta)
	case "stop":
		if !h.authorizeRun(c, req.RunID) {
		return
	}
	meta, apiErr := h.engine.Stop(req.RunID)
		if apiErr != nil {
			middleware.WriteError(c, apiErr)
			return
		}
		middleware.WriteOK(c, meta)
	case "label":
		if !h.authorizeRun(c, req.RunID) {
		return
	}
	meta, apiErr := h.engine.Label(req.RunID, req.Label)
		if apiErr != nil {
			middleware.WriteError(c, apiErr)
			return
		}
		middleware.WriteOK(c, meta)
	case "delete":
		if !h.authorizeRun(c, req.RunID) {
		return
	}
	if apiErr := h.engine.Delete(req.RunID); apiErr != nil {
			middleware.WriteError(c, apiErr)
			return
		}
		middleware.WriteOK[any](c, nil)
	default:
		middleware.WriteError(c, middleware.BadRequest("invalid action", map[string]string{"action": "pause|resume|stop|label|delete"}))
	}
}

// Status GET /backtest/status?run_id=
// Status 回测状态。
// @Summary 回测状态
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Success 200 {object} dto.ApiResponse[dto.BacktestStatusPayload]
// @Failure 404 {object} dto.ErrorResponse "1412 run 不存在"
// @Router /backtest/status [get]
func (h *BacktestHandler) Status(c *gin.Context) {
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	payload, apiErr := h.engine.Status(c.Query("run_id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, payload)
}

// Runs GET /backtest/runs?state=&search=&page=&size=
// Runs 回测运行列表（分页 + state 过滤）。
// @Summary 回测运行列表
// @Tags backtest
// @Produce json
// @Param state query string false "按状态过滤"
// @Param search query string false "按 run_id/label 模糊搜索"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /backtest/runs [get]
func (h *BacktestHandler) Runs(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	items, total := h.engine.List(currentUserID(c), q.Status, c.Query("search"), page, size)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	anyItems := make([]any, 0, len(items))
	for _, r := range items {
		anyItems = append(anyItems, r)
	}
	middleware.WriteOK(c, dto.PaginatedData[any]{Items: anyItems, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// Equity GET /backtest/equity?run_id=&tf=&limit=
// Equity 权益序列。
// @Summary 回测权益曲线
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Param tf query string false "降采样周期（保留参数，当前全量返回）"
// @Param limit query int false "条数上限" default(5000)
// @Success 200 {object} dto.ApiResponse[[]dto.BacktestEquityPoint]
// @Router /backtest/equity [get]
func (h *BacktestHandler) Equity(c *gin.Context) {
	limit := 5000
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	items, apiErr := h.engine.Equity(c.Query("run_id"), limit)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	out := make([]dto.BacktestEquityPoint, 0, len(items))
	for _, e := range items {
		out = append(out, dto.BacktestEquityPoint{
			Timestamp: e.Timestamp, Equity: e.Equity, Available: e.Available,
			PnL: e.PnL, PnLPct: e.PnLPct, DrawdownPct: e.DrawdownPct, Cycle: e.Cycle,
		})
	}
	middleware.WriteOK(c, out)
}

// Trades GET /backtest/trades?run_id=
// Trades 回测成交。
// @Summary 回测成交
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Success 200 {object} dto.ApiResponse[[]dto.BacktestTradeEvent]
// @Router /backtest/trades [get]
func (h *BacktestHandler) Trades(c *gin.Context) {
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	items, apiErr := h.engine.Trades(c.Query("run_id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	out := make([]dto.BacktestTradeEvent, 0, len(items))
	for _, t := range items {
		out = append(out, dto.BacktestTradeEvent{
			Timestamp: t.Timestamp, Symbol: t.Symbol, Action: t.Action, Side: t.Side,
			Quantity: t.Quantity, Price: t.Price, Fee: t.Fee, Slippage: t.Slippage,
			OrderValue: t.OrderValue, RealizedPnL: t.RealizedPnL, Leverage: t.Leverage,
			Cycle: t.Cycle, PositionAfter: t.PositionAfter, LiquidationFlag: t.Liquidation, Note: t.Note,
		})
	}
	middleware.WriteOK(c, out)
}

// Metrics GET /backtest/metrics?run_id=
// Metrics 回测指标（未就绪 202）。
// @Summary 回测指标
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Success 200 {object} dto.ApiResponse[dto.BacktestMetrics]
// @Failure 202 {object} dto.ApiResponse[any] "回测未完成"
// @Router /backtest/metrics [get]
func (h *BacktestHandler) Metrics(c *gin.Context) {
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	m, ready, apiErr := h.engine.Metrics(c.Query("run_id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	if !ready {
		c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "backtest not ready", "data": nil, "timestamp": time.Now().UnixMilli(), "request_id": middleware.GetRequestID(c)})
		return
	}
	middleware.WriteOK(c, m)
}

// Trace GET /backtest/trace?run_id=&cycle=
// Trace 单周期决策留痕。
// @Summary 回测决策 trace
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Param cycle query int true "周期号"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1412 决策不存在"
// @Router /backtest/trace [get]
func (h *BacktestHandler) Trace(c *gin.Context) {
	cycle, err := strconv.ParseInt(c.Query("cycle"), 10, 64)
	if err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid cycle", nil))
		return
	}
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	rec, apiErr := h.engine.Trace(c.Query("run_id"), cycle)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, rec)
}

// Decisions GET /backtest/decisions?run_id=&page=&size=
// Decisions 回测决策分页。
// @Summary 回测决策列表
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Router /backtest/decisions [get]
func (h *BacktestHandler) Decisions(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	items, total, apiErr := h.engine.Decisions(c.Query("run_id"), page, size)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
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

// Export GET /backtest/export?run_id=
// Export 导出 zip 附件（run + trades + equities + decisions）。
// @Summary 回测导出
// @Tags backtest
// @Produce application/zip
// @Param run_id query string true "run_id"
// @Success 200 {file} binary "zip 附件"
// @Failure 404 {object} dto.ErrorResponse "1412 run 不存在"
// @Router /backtest/export [get]
func (h *BacktestHandler) Export(c *gin.Context) {
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	data, apiErr := h.engine.Export(c.Query("run_id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=backtest-"+c.Query("run_id")+".zip")
	c.Data(http.StatusOK, "application/zip", data)
}

// Klines GET /backtest/klines?run_id=&symbol=&timeframe=
// Klines 回测用 K 线（数据源链实时）。
// @Summary 回测 K 线
// @Tags backtest
// @Produce json
// @Param run_id query string true "run_id"
// @Param symbol query string true "交易对"
// @Param timeframe query string false "周期" default(15m)
// @Success 200 {object} dto.ApiResponse[map[string]any]
// @Router /backtest/klines [get]
func (h *BacktestHandler) Klines(c *gin.Context) {
	if !h.authorizeRun(c, c.Query("run_id")) {
		return
	}
	items, apiErr := h.engine.Klines(c.Query("run_id"), c.Query("symbol"), c.Query("timeframe"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, map[string]any{"klines": items})
}

// currentUserID 当前请求用户 ID（JWT Subject = 用户 ID；未登录/异常回退单用户标识）。
func currentUserID(c *gin.Context) string {
	if cl := middleware.UserClaims(c); cl != nil {
		return cl.Subject
	}
	return "single-user"
}

// userIDOfClaims 从已解析的 JWT claims 取用户 ID（WS 链路无 gin.Context）。
func userIDOfClaims(cl *jwt.Claims) string {
	if cl != nil && cl.Subject != "" {
		return cl.Subject
	}
	return "single-user"
}
