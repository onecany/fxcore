// FXcore API 服务入口（API设计.md 7.1 蓝图的 cmd/server/main.go）。
//
// @title FXcore API
// @version 1.0
// @description AI 驱动的加密货币交易智能体平台 API（v1）。认证：access token 存 HttpOnly Cookie；写操作需 X-Signature/X-Timestamp/X-Nonce 签名头（HMAC-SHA256，±5min，nonce 防重放）。
// @host localhost:8080
// @BasePath /api/v1
//
// 配置加载顺序（后加载者不覆盖已存在的值）：
//   1. 进程环境变量（shell export / systemd Environment）
//   2. .env 文件（ENV_FILE 指定路径，默认工作目录下 .env；不存在则忽略）
//
// 环境变量清单见仓库根目录 .env.example。
//
//	PORT               监听端口（默认 8080）
//	ADMIN_EMAIL        管理员邮箱（默认 admin@fxcore.local）
//	ADMIN_PASSWORD     管理员密码（默认 admin123，仅限开发）
//	ADMIN_SIGN_SECRET  管理员请求签名密钥（默认随机生成）
//	JWT_SECRET         access token 签名密钥（默认随机生成，重启后失效）
//	RSA_PRIVATE_KEY    API Key 加密私钥 PEM/base64（默认启动时生成，仅存内存）
//	REDIS_URL          Redis 连接串（默认降级内存实现）
//	CORS_ORIGINS       追加 CORS 白名单（逗号分隔）
//	TLS_CERT_FILE      TLS 证书 PEM 路径；与 TLS_KEY_FILE 同时设置时启用 HTTPS + HTTP/2
//	TLS_KEY_FILE       TLS 私钥 PEM 路径
//	ENV_FILE           .env 文件路径（默认 .env）
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/net/http2"

	v1 "fxcore/internal/api/v1"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

func main() {
	// 加载 .env：文件不存在/语法错误不阻断启动（本地无 .env 也能跑）
	envFile := getenv("ENV_FILE", ".env")
	if err := godotenv.Load(envFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("[main] ⚠️  load env file %s: %v (ignored, using process env)", envFile, err)
		}
	}

	port := getenv("PORT", "8080")

	// 管理员种子
	adminEmail := getenv("ADMIN_EMAIL", "admin@fxcore.local")
	adminPass := getenv("ADMIN_PASSWORD", "admin123")
	adminSignSecret := os.Getenv("ADMIN_SIGN_SECRET") // 空则自动生成

	st, err := store.New(store.Config{
		AdminEmail:      adminEmail,
		AdminPassword:   adminPass,
		AdminSignSecret: adminSignSecret,
	})
	if err != nil {
		log.Fatalf("[main] init store: %v", err)
	}
	log.Printf("[main] seeded admin: %s", adminEmail)
	if adminPass == "admin123" {
		log.Printf("[main] ⚠️  using default ADMIN_PASSWORD (dev only), set ADMIN_PASSWORD in production")
	}

	// JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = randomSecret()
		log.Printf("[main] ⚠️  JWT_SECRET not set, generated ephemeral secret (tokens invalid after restart)")
	}
	jm, err := jwt.NewManager(jwtSecret, 15*time.Minute)
	if err != nil {
		log.Fatalf("[main] init jwt: %v", err)
	}

	// RSA 密钥（API Key 加密）
	km, err := crypto.NewKeyManager(os.Getenv("RSA_PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("[main] init rsa: %v", err)
	}
	log.Printf("[main] rsa key ready: %d bits%s", km.KeySize(), rsaSourceHint(os.Getenv("RSA_PRIVATE_KEY")))

	// Redis（可选，降级内存）
	redisURL := os.Getenv("REDIS_URL")

	// 服务装配
	r := v1.NewRouter(v1.Deps{
		Store:        st,
		JWT:          jm,
		Tokens:       cache.NewTokenStore(redisURL),
		Limiter:      cache.NewRateLimiter(redisURL),
		Nonces:       cache.NewNonceCache(redisURL),
		KeyManager:   km,
		AccessTTL:    15 * time.Minute,
		ExtraOrigins: splitCSV(os.Getenv("CORS_ORIGINS")),
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// 慢请求防护：读超时覆盖请求头+body；WS 升级后连接被 hijack，不受 ReadTimeout 影响
		ReadTimeout:  30 * time.Second,
		IdleTimeout:  120 * time.Second,
		MaxHeaderBytes: 64 << 10,
	}

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("[main] shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[main] shutdown: %v", err)
		}
	}()

	// HTTP/2 显式配置（API设计.md §2 基建）：
	// - TLS 模式下 Go net/http 通过 ALPN 自动协商 h2，ConfigureServer 显式开启并限制并发流
	// - 明文模式无 TLS 无法协商 h2（h2c 不在范围），自动降级 HTTP/1.1
	certFile, keyFile := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")
	if certFile != "" && keyFile != "" {
		if err := http2.ConfigureServer(srv, &http2.Server{
			MaxConcurrentStreams: 100, // 单连接并发流上限（防 h2 多路复用耗尽连接）
		}); err != nil {
			log.Fatalf("[main] configure http2: %v", err)
		}
		log.Printf("[main] fxcore API listening on https://127.0.0.1:%s (HTTP/2 via TLS/ALPN, api/v1 + ws/dashboard)", port)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[main] listen: %v", err)
		}
	} else {
		log.Printf("[main] fxcore API listening on http://127.0.0.1:%s (HTTP/1.1; set TLS_CERT_FILE/TLS_KEY_FILE for HTTPS + HTTP/2)", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[main] listen: %v", err)
		}
	}
	log.Println("[main] bye")
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// randomSecret 生成随机密钥（JWT secret）。fail-closed：crypto/rand 失败即 panic，
// 不可退化为可预测时间戳（与 store.NewRefreshToken 一致）。
func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func rsaSourceHint(raw string) string {
	if raw == "" {
		return " (generated in-memory; set RSA_PRIVATE_KEY to persist)"
	}
	return " (loaded from RSA_PRIVATE_KEY)"
}
