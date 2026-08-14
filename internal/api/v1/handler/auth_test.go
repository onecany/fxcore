package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

// newAuthHandler 组装 auth handler（内存 store + 内存 token store + 固定 jwt）。
func newAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jm, err := jwt.NewManager("test-secret-123456", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return NewAuthHandler(st, jm, cache.NewTokenStore(""), 15*time.Minute)
}

func authRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	r.POST("/auth/logout", h.Logout)
	return r
}

// doJSON 发 JSON 请求并返回响应。
func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func parseEnvelope(t *testing.T, w *httptest.ResponseRecorder) (code int, data json.RawMessage) {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope parse: %v body=%s", err, w.Body.String())
	}
	return env.Code, env.Data
}

// 注册成功：200 + code 0 + sign_secret/refresh_token 下发 + access Cookie 设置。
func TestAuthRegisterSuccess(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	w := doJSON(r, http.MethodPost, "/auth/register", `{"email":"New@Example.com","password":"pass1234","nickname":"neo"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("register want 200, got %d %s", w.Code, w.Body.String())
	}
	code, data := parseEnvelope(t, w)
	if code != 0 {
		t.Fatalf("want code 0, got %d", code)
	}
	var resp dto.LoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("login resp parse: %v", err)
	}
	if resp.User.Email != "new@example.com" {
		t.Fatalf("email must be lowercased, got %q", resp.User.Email)
	}
	if resp.SignSecret == "" || resp.RefreshToken == "" || resp.AccessToken == "" {
		t.Fatalf("sign_secret/refresh/access must be non-empty")
	}
	cookies := w.Result().Cookies()
	hasAccess := false
	for _, ck := range cookies {
		if ck.Name == "fx_access_token" && ck.HttpOnly {
			hasAccess = true
		}
	}
	if !hasAccess {
		t.Fatalf("want HttpOnly access cookie, got %v", cookies)
	}
}

// 注册后登录：小写邮箱命中 + sign_secret 与注册时一致。
func TestAuthLoginAfterRegister(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"pass1234"}`)

	w := doJSON(r, http.MethodPost, "/auth/login", `{"email":"A@B.com","password":"pass1234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login want 200, got %d %s", w.Code, w.Body.String())
	}
	_, data := parseEnvelope(t, w)
	var resp dto.LoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("login resp parse: %v", err)
	}
	if resp.User.Email != "a@b.com" {
		t.Fatalf("want lowercased email, got %q", resp.User.Email)
	}
}

// 错误密码：400 + 1001。
func TestAuthLoginWrongPassword(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"pass1234"}`)

	w := doJSON(r, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"wrongpass"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 刷新：注册拿 refresh → refresh 轮换 → 新 token 可用；旧 token 重放 → 1101。
func TestAuthRefreshRotationAndReplay(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	regW := doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"pass1234"}`)
	_, regData := parseEnvelope(t, regW)
	var reg dto.LoginResponse
	if err := json.Unmarshal(regData, &reg); err != nil {
		t.Fatalf("register resp parse: %v", err)
	}

	// 第一次 refresh：成功（1101 之外的 0）
	w := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+reg.RefreshToken+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh want 200, got %d %s", w.Code, w.Body.String())
	}
	code, data := parseEnvelope(t, w)
	if code != 0 {
		t.Fatalf("refresh want code 0, got %d", code)
	}
	var ref dto.RefreshResponse
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("refresh resp parse: %v", err)
	}
	if ref.RefreshToken == reg.RefreshToken {
		t.Fatalf("refresh must rotate token")
	}

	// 旧 token 重放：1101 + 401
	w2 := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+reg.RefreshToken+`"}`)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("replay want 401, got %d %s", w2.Code, w2.Body.String())
	}
	code2, _ := parseEnvelope(t, w2)
	if code2 != 1101 {
		t.Fatalf("replay want 1101, got %d", code2)
	}
}

// 登出：带 refresh 登出后该 token 再 refresh → 1101。
func TestAuthLogoutRevokesRefresh(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	regW := doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"pass1234"}`)
	_, regData := parseEnvelope(t, regW)
	var reg dto.LoginResponse
	if err := json.Unmarshal(regData, &reg); err != nil {
		t.Fatalf("register resp parse: %v", err)
	}

	w := doJSON(r, http.MethodPost, "/auth/logout", `{"refresh_token":"`+reg.RefreshToken+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("logout want 200, got %d", w.Code)
	}

	w2 := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+reg.RefreshToken+`"}`)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout want 401, got %d %s", w2.Code, w2.Body.String())
	}
}

// 重复注册同邮箱：400 + 1001。
func TestAuthRegisterDuplicateEmail(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"pass1234"}`)

	w := doJSON(r, http.MethodPost, "/auth/register", `{"email":"A@B.com","password":"pass1234"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate register want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 密码过短：400 + 1001（binding min=8）。
func TestAuthRegisterShortPassword(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	w := doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"short"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("short password want 400, got %d %s", w.Code, w.Body.String())
	}
}
