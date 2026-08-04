package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/pkg/cache"
)

// rateLimit 类型（文档 6.3）。
const (
	rateAuth  = "auth"  // 5 次/分钟/IP
	rateRead  = "read"  // 120 次/分钟/用户
	rateWrite = "write" // 30 次/分钟/用户
)

var rateLimits = map[string]int{
	rateAuth:  5,
	rateRead:  120,
	rateWrite: 30,
}

// RateLimit 滑动窗口限流中间件。
// kind: auth / read / write；key 维度 IP，已登录时叠加用户 ID。
func RateLimit(rl cache.RateLimiter, kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, ok := rateLimits[kind]
		if !ok {
			c.Next()
			return
		}
		key := kind + ":" + c.ClientIP()
		if claims := UserClaims(c); claims != nil {
			key += ":" + claims.Subject
		}
		allowed, retryAfter := rl.Allow(c.Request.Context(), key, limit, time.Minute)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			WriteError(c, RateLimited("too many requests, retry in "+strconv.Itoa(retryAfter)+"s"))
			return
		}
		c.Next()
	}
}
