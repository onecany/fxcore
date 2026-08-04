package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// S2：SetTrustedProxies(nil) 后 ClientIP 必须忽略伪造的 X-Forwarded-For，
// 限流 key 基于真实 socket IP，无法通过换头绕过。
func TestClientIPIgnoresXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	r.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.7:12345" // 真实来源
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != "203.0.113.7" {
		t.Fatalf("ClientIP=%s, want real socket IP 203.0.113.7 (XFF spoofing must be ignored)", got)
	}
}
