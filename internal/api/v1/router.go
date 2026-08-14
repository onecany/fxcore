// Package v1 组装 /api/v1 版本化路由（API设计.md 第 4 节）。
// 架构铁律：v1 代码物理隔离于 internal/api/v1/，未来 v2 并行共存。
package v1

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"fxcore/internal/api/v1/handler"
	"fxcore/internal/api/v1/openapi3"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/backtest"
	"fxcore/internal/debate"
	"fxcore/internal/llm"
	"fxcore/internal/middleware"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/provider"
	"fxcore/internal/store"
	"fxcore/internal/trader/engine"
)

// Deps 路由依赖注入。
type Deps struct {
	Store        *store.Store
	JWT          *jwt.Manager
	Tokens       cache.TokenStore
	Limiter      cache.RateLimiter
	Nonces       cache.NonceCache
	KeyManager   *crypto.KeyManager
	AI           *llm.Client              // 统一 AI 客户端（backtest/debate/引擎共用）
	Models       llm.ModelProvider        // 模型配置访问（组装层注入 adapter）
	Klines       provider.KlineProvider   // 数据源链（hyperliquid→okx→coinank）
	Backtest     *backtest.Engine         // 回测引擎（可空：未注入则 backtest 路由不可用）
	Debate       *debate.Engine           // 辩论引擎（可空：未注入则 debate 路由不可用）
	Trader       *engine.Engine           // 交易引擎（可空：未注入则 traders 仅状态机）
	StaticDir    string                   // web/dist 目录（空 = 不托管前端）
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
	r.Use(middleware.RequestID(), middleware.AccessLog(), middleware.Recovery(), middleware.CORS(d.ExtraOrigins...))

	// 静态托管（web/dist 构建产物；STATIC_DIR 覆盖，空则不托管）
	if d.StaticDir != "" {
		// 缓存策略：hash 文件名资源（assets/exchange-icons）强缓存 immutable；
		// 其余非 API 路径（SPA 入口 index.html 与前端路由回退）一律 no-cache，
		// 防止浏览器启发式缓存旧 index.html 导致加载旧 bundle（K 线面板等新功能"消失"）。
		r.Use(func(c *gin.Context) {
			p := c.Request.URL.Path
			switch {
			case strings.HasPrefix(p, "/assets/"), strings.HasPrefix(p, "/exchange-icons/"):
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			case !strings.HasPrefix(p, "/api/"):
				c.Header("Cache-Control", "no-cache")
			}
			c.Next()
		})
		r.Static("/assets", filepath.Join(d.StaticDir, "assets"))
		r.Static("/exchange-icons", filepath.Join(d.StaticDir, "exchange-icons"))
		r.StaticFile("/", filepath.Join(d.StaticDir, "index.html"))
		r.StaticFile("/favicon.ico", filepath.Join(d.StaticDir, "favicon.ico"))
	}

	// 404/405 统一信封
	r.NoRoute(func(c *gin.Context) {
		// SPA 回退：静态托管开启时，非 API 路径交给 index.html（前端路由刷新不 404）
		if d.StaticDir != "" && !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.File(filepath.Join(d.StaticDir, "index.html"))
			return
		}
		middleware.WriteError(c, middleware.NotFound("route not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		middleware.WriteError(c, middleware.NotFound("method not allowed"))
	})

	// ========== WebSocket（独立于 gzip，压缩与升级互斥） ==========
	dashboardH := handler.NewDashboardHandler(d.Store, d.JWT)
	r.GET("/ws/dashboard", dashboardH.WS)

	// ========== Swagger UI（OpenAPI 3.0，API设计.md §2 基建） ==========
	// 访问 /swagger/index.html 查看文档。UI 通过 URL 参数加载 embed 的
	// OpenAPI 3.1 规范（/api/v1/openapi3.json），不依赖 docs 包——
	// 仓库根 docs/ 保持纯文档目录（无 Go 包）。
	// /swagger/doc.json 遗留端点（gin-swagger 内部读 SwaggerInfo，docs 包
	// 移除后会 500）在此显式接管为 3.1 spec。
	r.GET("/swagger/*any", func(c *gin.Context) {
		if c.Param("any") == "/doc.json" {
			c.Data(200, "application/json; charset=utf-8", openapi3.Spec)
			return
		}
		ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/v1/openapi3.json"))(c)
	})

	// ========== /api/v1 组（gzip 压缩传输，文档 2） ==========
	api := r.Group("/api/v1", gzip.Gzip(gzip.DefaultCompression))
	{
		// OpenAPI 3.1 规范（公开文档，无需认证）
		api.GET("/openapi3.json", func(c *gin.Context) {
			c.Data(200, "application/json; charset=utf-8", openapi3.Spec)
		})

		// --- 认证（auth 限流：5 次/分钟/IP） ---
		authH := handler.NewAuthHandler(d.Store, d.JWT, d.Tokens, d.AccessTTL)
		auth := api.Group("/auth", middleware.RateLimit(d.Limiter, "auth"))
		{
			auth.POST("/login", authH.Login)
			auth.POST("/register", authH.Register)
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
		traderH := handler.NewTraderHandler(service.NewTraderService(d.Store, d.Trader))
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

		// --- 交易所（§11 exchanges 路由） ---
		exchangeH := handler.NewExchangeHandler(service.NewExchangeService(d.Store, d.KeyManager))
		read.GET("/exchanges", exchangeH.List)
		read.GET("/exchanges/balances", exchangeH.ListBalances)
		write.POST("/exchanges", exchangeH.Create)
		write.PUT("/exchanges/:id", exchangeH.Update)
		write.DELETE("/exchanges/:id", exchangeH.Delete)

		// --- 策略（§11 strategies 路由；静态路由先于 :id 注册，gin 静态优先） ---
		strategyH := handler.NewStrategyHandler(service.NewStrategyService(d.Store))
		read.GET("/strategies", strategyH.List)
		read.GET("/strategies/active", strategyH.GetActive)
		read.GET("/strategies/default-config", strategyH.GetDefaultConfig)
		read.GET("/strategies/public", strategyH.Public)
		read.GET("/strategies/:id", strategyH.Get)
		write.POST("/strategies", strategyH.Create)
		write.POST("/strategies/preview-prompt", strategyH.PreviewPrompt)
		write.POST("/strategies/test-run", strategyH.TestRun)
		write.PUT("/strategies/:id", strategyH.Update)
		write.DELETE("/strategies/:id", strategyH.Delete)
		write.POST("/strategies/:id/activate", strategyH.Activate)
		write.POST("/strategies/:id/duplicate", strategyH.Duplicate)

		// --- 加密工具（§11 crypto 路由） ---
		cryptoH := handler.NewCryptoHandler(d.KeyManager)
		read.GET("/crypto/config", cryptoH.GetConfig)
		read.GET("/crypto/public-key", cryptoH.GetPublicKey)
		write.POST("/crypto/decrypt", cryptoH.Decrypt)

		// --- Telegram（§11 telegram 路由；绑定流程引擎层阶段 3） ---
		telegramH := handler.NewTelegramHandler(service.NewTelegramService(d.Store, d.KeyManager))
		read.GET("/telegram", telegramH.Get)
		write.POST("/telegram", telegramH.Save)
		write.POST("/telegram/model", telegramH.SetModel)
		write.DELETE("/telegram/binding", telegramH.DeleteBinding)

		// --- 数据/市场（§11 data + market 路由） ---
		dataH := handler.NewDataHandler(service.NewDataService(d.Store, d.Klines), d.Store)
		read.GET("/status", dataH.Status)
		read.GET("/account", dataH.Account)
		read.GET("/decisions", dataH.Decisions)
		read.GET("/decisions/latest", dataH.LatestDecision)
		read.GET("/statistics", dataH.Statistics)
		read.GET("/trades", dataH.Trades)
		read.GET("/orders", dataH.Orders)
		read.GET("/orders/:id/fills", dataH.OrderFills)
		read.GET("/open-orders", dataH.OpenOrders)
		read.GET("/klines", dataH.Klines)
		read.GET("/symbols", dataH.Symbols)
		read.GET("/positions/history", dataH.PositionHistory)
		read.GET("/equity-history", dataH.EquityHistory)
		read.POST("/equity-history-batch", dataH.EquityHistoryBatch) // 批量读，无副作用，免签名
		read.GET("/competition", dataH.Competition)
		read.GET("/top-traders", dataH.TopTraders)
		read.GET("/traders/:id/public-config", dataH.TraderPublicConfig)

		// --- 回测（§11 backtest 路由；引擎未注入时不注册） ---
		if d.Backtest != nil {
			backtestH := handler.NewBacktestHandler(d.Backtest)
			write.POST("/backtest/start", backtestH.Start)
			write.POST("/backtest/:action", backtestH.Control)
			read.GET("/backtest/status", backtestH.Status)
			read.GET("/backtest/runs", backtestH.Runs)
			read.GET("/backtest/equity", backtestH.Equity)
			read.GET("/backtest/trades", backtestH.Trades)
			read.GET("/backtest/metrics", backtestH.Metrics)
			read.GET("/backtest/trace", backtestH.Trace)
			read.GET("/backtest/decisions", backtestH.Decisions)
			read.GET("/backtest/export", backtestH.Export)
			read.GET("/backtest/klines", backtestH.Klines)
		}

		// --- 辩论（§11 debate 路由；引擎未注入时不注册） ---
		if d.Debate != nil {
			debateH := handler.NewDebateHandler(d.Debate)
			read.GET("/debates", debateH.List)
			read.GET("/debates/personalities", debateH.Personalities)
			write.POST("/debates", debateH.Create)
			read.GET("/debates/:id", debateH.Get)
			write.POST("/debates/:id/:action", debateH.Control)
			write.POST("/debates/:id/execute", debateH.Execute)
			write.DELETE("/debates/:id", debateH.Delete)
			read.GET("/debates/:id/messages", debateH.Messages)
			read.GET("/debates/:id/votes", debateH.Votes)
			read.GET("/debates/:id/stream", debateH.Stream)
		}
	}

	return r
}
