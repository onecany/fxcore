package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// defaultAllowedOrigins 严格 CORS 白名单（文档：https://cany.one + 本地开发端口）。
var defaultAllowedOrigins = []string{
	"https://cany.one",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:6000",
	"http://127.0.0.1:6000",
	"http://localhost:8080",
	"http://127.0.0.1:8080",
}

// CORS 严格白名单中间件。extraOrigins 可追加（如测试域名）。
func CORS(extraOrigins ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(defaultAllowedOrigins)+len(extraOrigins))
	for _, o := range append(defaultAllowedOrigins, extraOrigins...) {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id, X-Timestamp, X-Signature, X-Requested-With")
			c.Header("Access-Control-Expose-Headers", "X-Request-Id, Retry-After")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
