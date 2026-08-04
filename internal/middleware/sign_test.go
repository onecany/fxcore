package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

const testSecret = "test-sign-secret-0123456789abcdef"

// newSignEngine 构造带完整签名链的最小 engine：POST /api/v1/test
func newSignEngine(t *testing.T) (*gin.Engine, *jwt.Manager, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "x", AdminSignSecret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	admin, ok := st.GetUserByEmail("a@b.c")
	if !ok {
		t.Fatal("seeded admin missing")
	}
	jm, err := jwt.NewManager("test-jwt-secret", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	nc := cache.NewNonceCache("")

	r := gin.New()
	r.Use(RequiresAuth(jm))
	r.POST("/api/v1/test", RequireSignature(st, nc), func(c *gin.Context) {
		body := make([]byte, 0)
		if c.Request.Body != nil {
			buf := make([]byte, 4096)
			n, _ := c.Request.Body.Read(buf)
			body = buf[:n]
		}
		c.String(http.StatusOK, "ok:%d", len(body))
	})
	return r, jm, testSecret, admin.ID
}

// signReq 构造带签名/时间戳/nonce 的请求。
func signReq(t *testing.T, jm *jwt.Manager, secret, userID, method, path, body, nonce string) *http.Request {
	t.Helper()
	token, err := jm.Issue(userID, "a@b.c")
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s%s%s%s", ts, path, body, nonce)))
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sig)
	return req
}

func doSign(t *testing.T, r *gin.Engine, req *http.Request) int {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Logf("resp body: %s", w.Body.String())
	}
	return w.Code
}

// 合法签名 + 空 body -> 200（ClosePosition EOF 修复的中间件侧验证）。
func TestSignatureValidEmptyBody(t *testing.T) {
	r, jm, secret, uid := newSignEngine(t)
	code := doSign(t, r, signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "", "nonce-1"))
	if code != http.StatusOK {
		t.Fatalf("valid signed empty-body request should be 200, got %d", code)
	}
}

// 同 nonce 重放 -> 400（验签后 nonce 去重）。
func TestSignatureNonceReplayRejected(t *testing.T) {
	r, jm, secret, uid := newSignEngine(t)
	req := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "{}", "replay-nonce")
	if code := doSign(t, r, req); code != http.StatusOK {
		t.Fatalf("first use should be 200, got %d", code)
	}
	req2 := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "{}", "replay-nonce")
	if code := doSign(t, r, req2); code != http.StatusBadRequest {
		t.Fatalf("nonce replay should be 400, got %d", code)
	}
}

// 超长 body（>1MiB）-> 400：MaxBytesReader 在读 body 前生效（S3）。
func TestSignatureOversizedBodyRejected(t *testing.T) {
	r, jm, secret, uid := newSignEngine(t)
	big := strings.Repeat("x", maxBodyBytes+1024)
	req := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", big, "nonce-big")
	code := doSign(t, r, req)
	if code != http.StatusBadRequest {
		t.Fatalf("oversized body should be 400, got %d", code)
	}
}

// 缺 nonce -> 400。
func TestSignatureMissingNonce(t *testing.T) {
	r, jm, secret, uid := newSignEngine(t)
	req := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "{}", "nonce-missing")
	req.Header.Del("X-Nonce")
	// 签名里含 nonce，删除 header 后签名不匹配 -> 400
	code := doSign(t, r, req)
	if code != http.StatusBadRequest {
		t.Fatalf("missing nonce should be 400, got %d", code)
	}
}

// 错误签名 -> 400，且不消耗 nonce（验签在前，nonce 在后）。
func TestSignatureInvalidNoNonceConsumption(t *testing.T) {
	r, jm, secret, uid := newSignEngine(t)
	req := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "{}", "nonce-x")
	// 篡改签名
	req.Header.Set("X-Signature", strings.Repeat("0", 64))
	if code := doSign(t, r, req); code != http.StatusBadRequest {
		t.Fatalf("bad signature should be 400, got %d", code)
	}
	// 同 nonce 的合法请求随后应仍可用（坏签名未消耗 nonce）
	req2 := signReq(t, jm, secret, uid, http.MethodPost, "/api/v1/test", "{}", "nonce-x")
	if code := doSign(t, r, req2); code != http.StatusOK {
		t.Fatalf("nonce consumed by failed signature check: got %d", code)
	}
}
