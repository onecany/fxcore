package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/pkg/cache"
	"fxcore/internal/store"
)

// signWindow 时间戳允许偏差（±5 分钟，文档 6.1）。
const signWindow = 5 * 60

// maxBodyBytes 写请求 body 上限（S3：无界 body 读取会内存 DoS，验签前整读入内存）。
const maxBodyBytes = 1 << 20 // 1 MiB

// RequireSignature 写操作请求签名校验（文档 6.1）：
// X-Signature = HMAC-SHA256(timestamp + path + body + nonce, user_secret)
// 依赖 RequiresAuth 先行执行（需从 store 取 user_secret）。
// S6 防重放：X-Nonce 一次性随机串，写入签名串 + nonce 缓存（±5min 内重复即拒绝）。
func RequireSignature(s *store.Store, nc cache.NonceCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		// S3：body 上限在读取前生效（超限的 Read 立即报错）
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

		claims := UserClaims(c)
		if claims == nil {
			WriteError(c, TokenInvalid("missing auth context"))
			return
		}
		u, ok := s.GetUserByID(claims.Subject)
		if !ok {
			WriteError(c, TokenInvalid("user not found"))
			return
		}

		tsStr := c.GetHeader("X-Timestamp")
		sig := c.GetHeader("X-Signature")
		nonce := c.GetHeader("X-Nonce")
		if tsStr == "" || sig == "" || nonce == "" {
			WriteError(c, BadRequest("missing X-Timestamp, X-Signature or X-Nonce header", nil))
			return
		}
		if len(nonce) > 64 {
			// S4：nonce 长度上限（防认证用户滥用存储 key 大小）
			WriteError(c, BadRequest("X-Nonce too long (max 64 chars)", nil))
			return
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			WriteError(c, BadRequest("invalid X-Timestamp", nil))
			return
		}
		if delta := time.Now().Unix() - ts; delta > signWindow || delta < -signWindow {
			WriteError(c, BadRequest("X-Timestamp out of ±5min window", nil))
			return
		}

		// 先读 body 验签，验签通过后才消耗 nonce。
		// 顺序必须如此：nonce 去重若在验签前，无有效签名的攻击者也能
		// 向 nonce 缓存写入条目（TTL 5min），形成无认证 DoS 面。
		// S3：读取错误（如超 1MiB 被 MaxBytesReader 截断）显式拒绝，
		// 不用截断 body 继续验签（模式干净 + 错误信息准确）。
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			WriteError(c, BadRequest("request body too large or unreadable: "+err.Error(), nil))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(u.SignSecret))
		_, _ = mac.Write([]byte(fmt.Sprintf("%d%s%s%s", ts, c.Request.URL.Path, body, nonce)))
		expect := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expect), []byte(sig)) {
			WriteError(c, BadRequest("invalid X-Signature", nil))
			return
		}

		// S6：nonce 一次性校验（防签名重放：同 nonce 的请求 5min 内重复即拒绝）。
		// 此刻签名已验，重放者必然携带截获的有效签名，nonce 缓存是唯一防线。
		nonceKey := "sig:" + claims.Subject + ":" + nonce
		first, err := nc.Add(c.Request.Context(), nonceKey, signWindow*time.Second)
		if err != nil {
			WriteError(c, Internal("nonce check failed: "+err.Error()))
			return
		}
		if !first {
			WriteError(c, BadRequest("request replay detected (duplicate nonce)", nil))
			return
		}
		c.Next()
	}
}
