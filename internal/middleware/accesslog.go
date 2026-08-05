package middleware

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/pkg/logger"
)

// AccessLog API 访问日志中间件（结构化 JSON，一行一条，文件+终端双写）。
// 必须放在中间件链最外层（RequestID 之后）：c.Next() 返回时 Recovery 已完成
// panic 处理并写入状态码，此处能记录真实的最终状态（含 500）。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		l := logger.Access()
		if l == nil {
			return // logger 未初始化（理论上不会发生：main 先 Init）
		}
		entry := struct {
			Time       string `json:"time"`
			RequestID  string `json:"request_id"`
			Method     string `json:"method"`
			Path       string `json:"path"`
			Status     int    `json:"status"`
			LatencyMs  int64  `json:"latency_ms"`
			ClientIP   string `json:"client_ip"`
			UserAgent  string `json:"user_agent"`
			BodyBytes  int    `json:"body_bytes"`
			ReqErrors  bool   `json:"req_errors"`
			ErrMessage string `json:"err_message,omitempty"`
		}{
			Time:       start.UTC().Format(time.RFC3339),
			RequestID:  GetRequestID(c),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			Status:     c.Writer.Status(),
			LatencyMs:  time.Since(start).Milliseconds(),
			ClientIP:   c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			BodyBytes:  c.Writer.Size(),
			ReqErrors:  c.Writer.Status() >= 400,
			ErrMessage: lastErrMsg(c),
		}
		buf, err := json.Marshal(entry)
		if err != nil {
			l.Printf("accesslog marshal failed: %v", err)
			return
		}
		l.Printf("%s", buf)
	}
}

// lastErrMsg 取 gin 错误链最后一条的错误信息。
func lastErrMsg(c *gin.Context) string {
	if e := c.Errors.Last(); e != nil && e.Err != nil {
		return e.Err.Error()
	}
	return ""
}
