package handler

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

// wsHeartbeat 心跳间隔（文档：30s）。
const wsHeartbeat = 30 * time.Second

// wsPush 快照推送间隔。
const wsPush = 5 * time.Second

// WS 资源防护（S5）：全局连接上限 + 每用户上限 + 读帧上限 + 读超时。
const (
	maxWSConnections = 500
	maxWSConnPerUser = 10
	wsReadLimit      = 4 << 10 // 4 KiB
	wsReadTimeout    = 60 * time.Second
)

var (
	wsConnTotal  atomic.Int64
	wsConnPerUser sync.Map // userID -> *atomic.Int64
)

// DashboardHandler 仪表盘 3 合 1 聚合 + WebSocket 推送。
type DashboardHandler struct {
	store *store.Store
	jm    *jwt.Manager
}

// NewDashboardHandler 构造仪表盘处理器。
func NewDashboardHandler(s *store.Store, jm *jwt.Manager) *DashboardHandler {
	return &DashboardHandler{store: s, jm: jm}
}

// GetDashboard GET /dashboard
// 使用 sync.WaitGroup 并发聚合四路数据（文档 6.4 与 §8 防错清单）。
// GetDashboard 仪表盘 3 合 1（账户汇总 + 活跃交易员 + 最近持仓 + 健康度）。
// @Summary 仪表盘聚合
// @Description 账户汇总（基准 10000 + 累计已平仓 PnL）、活跃交易员、最近 5 条持仓、AI/交易所延迟
// @Tags dashboard
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.DashboardSummary]
// @Router /dashboard [get]
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	middleware.WriteOK(c, h.aggregate(currentUserID(c)))
}

// WS GET /ws/dashboard?token=xxx
// 鉴权（query token）-> 升级 WebSocket -> 5s 推送快照 + 30s 心跳。
// S5 资源防护：连接数上限、读帧上限、读超时。
// WS WebSocket 实时推送（5s 快照 + 30s 心跳）。
// @Summary WebSocket 实时推送
// @Description 鉴权：query token 为 JWT access token；5s 推一次仪表盘快照，30s 心跳
// @Tags dashboard
// @Success 101 {string} string "WebSocket 升级成功"
// @Failure 401 {object} dto.ErrorResponse "1002 token 无效"
// @Router /ws/dashboard [get]
func (h *DashboardHandler) WS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		middleware.WriteError(c, middleware.TokenInvalid("missing token query param"))
		return
	}
	claims, err := h.jm.Parse(token)
	if err != nil {
		middleware.WriteError(c, middleware.TokenInvalid("invalid token"))
		return
	}

	// S5：连接限额（全局 + 每用户）
	if !h.acquireConn(claims.Subject) {
		middleware.WriteError(c, middleware.RateLimited("too many websocket connections"))
		return
	}
	defer h.releaseConn(claims.Subject)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// 浏览器场景校验 Origin 白名单；服务端/工具客户端无 Origin
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			return allowedOrigin(origin)
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// S5：读帧上限 + 读超时（防超大帧撑爆内存 / 连接无限挂起）
	conn.SetReadLimit(wsReadLimit)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	})

	// 读协程：客户端断开/异常时关闭 done；每次读取刷新读超时
	done := make(chan struct{})
	go func() {
		for {
			if err := conn.SetReadDeadline(time.Now().Add(wsReadTimeout)); err != nil {
				close(done)
				return
			}
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	pushTicker := time.NewTicker(wsPush)
	heartbeatTicker := time.NewTicker(wsHeartbeat)
	defer func() {
		pushTicker.Stop()
		heartbeatTicker.Stop()
	}()

	for {
		select {
		case <-pushTicker.C:
			if err := conn.WriteJSON(gin.H{"type": "dashboard", "ts": time.Now().UnixMilli(), "data": h.aggregate(userIDOfClaims(claims))}); err != nil {
				return
			}
		case <-heartbeatTicker.C:
			if err := conn.WriteJSON(gin.H{"type": "ping", "ts": time.Now().Unix()}); err != nil {
				return
			}
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

// aggregate 并发聚合仪表盘数据（sync.WaitGroup）。
func (h *DashboardHandler) aggregate(userID string) dto.DashboardSummary {
	var (
		wg       sync.WaitGroup
		account  dto.AccountSummary
		traders  []dto.TraderDTO
		positions []dto.PositionDTO
		aiLat    int64
		exchLat  int64
	)

	wg.Add(4)
	go h.safeAggregate(&wg, func() { account = h.buildAccount(userID) })
	go h.safeAggregate(&wg, func() { traders = h.buildActiveTraders(userID) })
	go h.safeAggregate(&wg, func() { positions = h.buildRecentPositions(userID) })
	go h.safeAggregate(&wg, func() { aiLat, exchLat = h.buildHealth(userID) })
	wg.Wait()

	status := "healthy"
	if aiLat < 0 || exchLat < 0 {
		status = "degraded"
	}
	return dto.DashboardSummary{
		Account:         account,
		ActiveTraders:   traders,
		RecentPositions: positions,
		SystemHealth:    dto.SystemHealth{Status: status, AILatency: aiLat, ExchangeLatency: exchLat},
	}
}

// safeAggregate 包装聚合子任务：recover 兜底 + 保证 wg.Done。
// gin Recovery 只捕 handler 自身 goroutine，子 goroutine panic 不兜底会崩整个进程，
// 必须在这里自兜。
func (h *DashboardHandler) safeAggregate(wg *sync.WaitGroup, fn func()) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[dashboard] aggregation panic: %v", r)
		}
	}()
	fn()
}

// buildAccount 账户汇总：totalPnL/dailyPnL 来自已平仓记录；
// totalBalance = 10_000 基准 + totalPnL（演示口径，见复盘）。
func (h *DashboardHandler) buildAccount(userID string) dto.AccountSummary {
	const baseBalance = 10000.0
	var totalPnL, dailyPnL float64
	now := time.Now().UTC()
	for _, p := range h.store.ListPositions() {
		if !h.ownedPosition(userID, p.TraderID) {
			continue
		}
		if p.ClosedAt == nil {
			continue
		}
		totalPnL += p.PnL
		if p.ClosedAt.Year() == now.Year() && p.ClosedAt.YearDay() == now.YearDay() {
			dailyPnL += p.PnL
		}
	}
	return dto.AccountSummary{TotalBalance: baseBalance + totalPnL, TotalPnL: totalPnL, DailyPnL: dailyPnL}
}

// buildActiveTraders 运行中的前 5 个交易员。
func (h *DashboardHandler) buildActiveTraders(userID string) []dto.TraderDTO {
	out := make([]dto.TraderDTO, 0, 5)
	for _, t := range h.store.ListTraders(userID) {
		if t.Status != model.StatusRunning {
			continue
		}
		out = append(out, traderToDTO(t))
		if len(out) >= 5 {
			break
		}
	}
	return out
}

// buildRecentPositions 最新 5 条持仓（ListPositions 已按最新在前）。
func (h *DashboardHandler) buildRecentPositions(userID string) []dto.PositionDTO {
	out := make([]dto.PositionDTO, 0, 5)
	for _, p := range h.store.ListPositions() {
		if !h.ownedPosition(userID, p.TraderID) {
			continue
		}
		out = append(out, positionToDTO(p))
		if len(out) >= 5 {
			break
		}
	}
	return out
}

// ownedPosition 持仓归属校验（trader 属主 == 当前用户；UserID 空放行）。
func (h *DashboardHandler) ownedPosition(userID, traderID string) bool {
	if userID == "" {
		return true
	}
	t, ok := h.store.GetTrader(traderID)
	if !ok {
		return false
	}
	return t.UserID == "" || t.UserID == userID
}

// buildHealth 健康度：AI 延迟取模型最近测试延迟均值；交易所延迟当前阶段无连接源 -> -1。
func (h *DashboardHandler) buildHealth(userID string) (aiLatency, exchangeLatency int64) {
	var sum, count int64
	for _, m := range h.store.ListModels(userID) {
		if m.LastTestAt != nil && m.LastTestLatencyMS > 0 {
			sum += m.LastTestLatencyMS
			count++
		}
	}
	if count > 0 {
		aiLatency = sum / count
	} else {
		aiLatency = -1
	}
	exchangeLatency = -1 // 交易所连接为下一阶段工作（见复盘）
	return aiLatency, exchangeLatency
}

// acquireConn S5：全局与每用户连接配额。失败返回 false。
func (h *DashboardHandler) acquireConn(userID string) bool {
	if wsConnTotal.Load() >= maxWSConnections {
		return false
	}
	perUser, _ := h.perUserCounter(userID)
	if perUser.Load() >= maxWSConnPerUser {
		return false
	}
	// 双检：先 load 再累加，仍有竞争但可接受（限额是软上限）；
	// 严格的原子配额用 CompareAndSwap 循环，这里从简
	perUser.Add(1)
	wsConnTotal.Add(1)
	return true
}

func (h *DashboardHandler) releaseConn(userID string) {
	if v, ok := h.perUserCounter(userID); ok {
		v.Add(-1)
	}
	wsConnTotal.Add(-1)
}

// perUserCounter 获取用户的连接计数器（懒创建）。
func (h *DashboardHandler) perUserCounter(userID string) (*atomic.Int64, bool) {
	if v, ok := wsConnPerUser.Load(userID); ok {
		return v.(*atomic.Int64), true
	}
	// 懒创建（少量泄漏可接受：map 条目在用户断开后保留）
	v, _ := wsConnPerUser.LoadOrStore(userID, &atomic.Int64{})
	return v.(*atomic.Int64), true
}

// allowedOrigin WS Origin 白名单（与 CORS 中间件一致，含 :8080）。
func allowedOrigin(origin string) bool {
	for _, o := range []string{
		"https://cany.one",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:6000",
		"http://127.0.0.1:6000",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
	} {
		if o == origin {
			return true
		}
	}
	return false
}
