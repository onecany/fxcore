// Package middleware 提供跨版本共享的 Gin 中间件（API设计.md 7.1）。
// 含统一错误码矩阵、错误信封输出、RequestID、Recovery。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
)

// ========== 错误码矩阵（API设计.md 第 5 节，与 web/src/utils/errorCodes.ts 同步硬编码） ==========
const (
	CodeOK                  = 0
	CodeBadRequest          = 1001 // 参数校验失败 -> 前端高亮字段
	CodeTokenInvalid        = 1002 // Token 失效 -> 前端静默刷新队列
	CodeForbidden           = 1003 // 无权限 -> 前端隐藏按钮
	CodeNotFound            = 1004 // 资源不存在 / 状态机非法跳转 -> 404 占位
	CodeRateLimited         = 1005 // 限流 -> 读取 Retry-After 倒计时
	CodeRefreshInvalid      = 1101 // Refresh 无效 -> 强制登出
	CodeInsufficientBalance = 1201 // 余额不足 -> 跳转充值
	CodeTraderRunning       = 1204 // 交易员运行中 -> 禁用 Start
	CodeAIFailed            = 1301 // AI 失败 -> 显示重试按钮
	CodeAITimeout           = 1302 // AI 超时 -> 自动重试 3 次
	CodeInternal            = 1500 // 服务器内部错误（错误码矩阵扩展：原矩阵无内部错误码，禁止复用 1001/1301 表达服务端故障）
)

// APIError 带业务码的错误。
type APIError struct {
	Code    int
	HTTP    int
	Message string
	Fields  map[string]string // 1001 时用于前端字段高亮
}

func (e *APIError) Error() string { return e.Message }

// NewAPIError 构造错误。
func NewAPIError(code, httpStatus int, msg string) *APIError {
	return &APIError{Code: code, HTTP: httpStatus, Message: msg}
}

// BadRequest 参数校验失败（1001）。
func BadRequest(msg string, fields map[string]string) *APIError {
	return &APIError{Code: CodeBadRequest, HTTP: http.StatusBadRequest, Message: msg, Fields: fields}
}

// TokenInvalid Token 失效（1002）。
func TokenInvalid(msg string) *APIError {
	return &APIError{Code: CodeTokenInvalid, HTTP: http.StatusUnauthorized, Message: msg}
}

// Forbidden 无权限（1003）。
func Forbidden(msg string) *APIError {
	return &APIError{Code: CodeForbidden, HTTP: http.StatusForbidden, Message: msg}
}

// NotFound 资源不存在（1004）。
func NotFound(msg string) *APIError {
	return &APIError{Code: CodeNotFound, HTTP: http.StatusNotFound, Message: msg}
}

// RateLimited 限流（1005）。
func RateLimited(msg string) *APIError {
	return &APIError{Code: CodeRateLimited, HTTP: http.StatusTooManyRequests, Message: msg}
}

// RefreshInvalid Refresh 无效（1101）。
func RefreshInvalid(msg string) *APIError {
	return &APIError{Code: CodeRefreshInvalid, HTTP: http.StatusUnauthorized, Message: msg}
}

// InsufficientBalance 余额不足（1201）。
func InsufficientBalance(msg string) *APIError {
	return &APIError{Code: CodeInsufficientBalance, HTTP: http.StatusBadRequest, Message: msg}
}

// TraderRunning 交易员运行中（1204）。
func TraderRunning(msg string) *APIError {
	return &APIError{Code: CodeTraderRunning, HTTP: http.StatusConflict, Message: msg}
}

// AIFailed AI 调用失败（1301）。
func AIFailed(msg string) *APIError {
	return &APIError{Code: CodeAIFailed, HTTP: http.StatusServiceUnavailable, Message: msg}
}

// AITimeout AI 调用超时（1302）。
func AITimeout(msg string) *APIError {
	return &APIError{Code: CodeAITimeout, HTTP: http.StatusGatewayTimeout, Message: msg}
}

// Internal 服务器内部错误（1500，错误码矩阵扩展）。
func Internal(msg string) *APIError {
	return &APIError{Code: CodeInternal, HTTP: http.StatusInternalServerError, Message: msg}
}

// ========== RequestID ==========

const requestIDKey = "fx_request_id"

// RequestID 生成/透传 X-Request-Id，注入上下文。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			} else {
				id = time.Now().Format("20060102150405.000")
			}
		}
		c.Set(requestIDKey, id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

// GetRequestID 从上下文取 RequestID。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ========== 响应输出 ==========

// WriteOK 输出统一成功信封（code=0）。
func WriteOK[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.OK(data, GetRequestID(c)))
}

// WriteError 输出统一错误信封。
func WriteError(c *gin.Context, e *APIError) {
	fields := map[string]any(nil)
	if e.Fields != nil {
		fields = map[string]any{}
		for k, v := range e.Fields {
			fields[k] = v
		}
	}
	c.AbortWithStatusJSON(e.HTTP, gin.H{
		"code":      e.Code,
		"message":   e.Message,
		"data":      fields,
		"timestamp": time.Now().UnixMilli(),
		"requestId": GetRequestID(c),
	})
}

// Recovery 兜底 panic，输出 500 信封（1500 内部错误码，而非 1001 参数校验）而非裸栈。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[panic] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
				WriteError(c, Internal("internal server error"))
			}
		}()
		c.Next()
	}
}
