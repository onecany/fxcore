// Package v1 组装 /api/v1 版本化路由（API设计.md 第 4 节）。
// 架构铁律：v1 代码物理隔离于 internal/api/v1/，未来 v2 并行共存。
package v1

import (
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/handler"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

// Deps 路由依赖注入。
type Deps struct {
	Store        *store.Store
	JWT          *jwt.Manager
	Tokens       cache.TokenStore
	Limiter      cache.RateLimiter
	Nonces       cache.NonceCache
	KeyManager   *crypto.KeyManager
	AccessTTL    time.Duration
	ExtraOrigins []string // 追加 CORS 白名单
}

// NewRouter 构建 gin.Engine（含 /api/v1 与 /ws/dashboard）。
func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// S2：关闭代理信任。gin 默认 trustedProxies=["0.0.0.0/0"]，攻击者可伪造
	// X-Forwarded-For 使 ClientIP() 返回任意值，绕过基于 IP 的限流。
	// 直连部署用真实 socket IP；若前置可信反代，改为显式信任其网段。
	if err := r.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	r.Use(middleware.RequestID(), middleware.Recovery(), middleware.CORS(d.ExtraOrigins...))

	// 404/405 统一信封
	r.NoRoute(func(c *gin.Context) {
		middleware.WriteError(c, middleware.NotFound("route not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		middleware.WriteError(c, middleware.NotFound("method not allowed"))
	})

	// ========== WebSocket（独立于 gzip，压缩与升级互斥） ==========
	dashboardH := handler.NewDashboardHandler(d.Store, d.JWT)
	r.GET("/ws/dashboard", dashboardH.WS)

	// ========== /api/v1 组（gzip 压缩传输，文档 2） ==========
	api := r.Group("/api/v1", gzip.Gzip(gzip.DefaultCompression))
	{
		// --- 认证（auth 限流：5 次/分钟/IP） ---
		authH := handler.NewAuthHandler(d.Store, d.JWT, d.Tokens, d.AccessTTL)
		auth := api.Group("/auth", middleware.RateLimit(d.Limiter, "auth"))
		{
			auth.POST("/login", authH.Login)
			auth.POST("/logout", authH.Logout)
		}
		// refresh 独立配额（L11：多标签页 15min 并发刷新不应撞 auth 的 5/min/IP 防爆破配额被误登出）
		api.POST("/auth/refresh", middleware.RateLimit(d.Limiter, "refresh"), authH.Refresh)

		// --- 鉴权组（RequiresAuth：HttpOnly Cookie 中的 access token） ---
		authed := api.Group("", middleware.RequiresAuth(d.JWT))
		// 读接口（限流 120 次/分钟/用户）
		read := authed.Group("", middleware.RateLimit(d.Limiter, "read"))
		// 写接口（限流在前拦截、HMAC 签名 + nonce 防重放在后：
		// L6 限流拒绝的请求不消耗 nonce；无签名攻击者先被 IP/用户限流挡住，省 HMAC CPU）
		write := authed.Group("", middleware.RateLimit(d.Limiter, "write"), middleware.RequireSignature(d.Store, d.Nonces))

		// --- AI 模型 ---
		modelH := handler.NewModelHandler(service.NewModelService(d.Store, d.KeyManager))
		read.GET("/models", modelH.List)
		read.GET("/models/providers", modelH.Providers)
		write.POST("/models", modelH.Create)
		write.PUT("/models/:id", modelH.Update)
		write.DELETE("/models/:id", modelH.Delete)
		write.POST("/models/:id/test", modelH.Test)

		// --- 交易员 ---
		traderH := handler.NewTraderHandler(service.NewTraderService(d.Store))
		read.GET("/traders", traderH.List)
		read.GET("/traders/:id", traderH.Get)
		write.POST("/traders", traderH.Create)
		write.PATCH("/traders/:id", traderH.Update)
		write.POST("/traders/:id/start", traderH.Start)
		write.POST("/traders/:id/pause", traderH.Pause)
		write.POST("/traders/:id/resume", traderH.Resume)
		write.POST("/traders/:id/stop", traderH.Stop)

		// --- 仪表盘（3 合 1 聚合，减少轮询） ---
		read.GET("/dashboard", dashboardH.GetDashboard)

		// --- 持仓 ---
		read.GET("/positions", traderH.ListPositions)
		write.DELETE("/positions/:id", traderH.ClosePosition)
	}

	return r
}
