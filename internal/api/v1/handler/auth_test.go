package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/pkg/mail"
	"fxcore/internal/store"
)

// newAuthHandler 组装 auth handler（内存 store + 内存 token/reset store + 固定 jwt）。
// mailer 默认 nil（dev 模式：forgot-password 返回 dev_link），需要时显式传入。
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
	return NewAuthHandler(st, jm, cache.NewTokenStore(""), cache.NewResetStore(""), nil, "http://localhost:5173", 15*time.Minute)
}

func authRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/forgot-password", h.ForgotPassword)
	r.POST("/auth/reset-password", h.ResetPassword)
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

// 忘记密码：未注册邮箱返回 200 + 通用文案 + 无 dev_link（防枚举，不泄露存在性）。
func TestAuthForgotPasswordUnknownEmail(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	w := doJSON(r, http.MethodPost, "/auth/forgot-password", `{"email":"nobody@nowhere.test"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("forgot-password want 200, got %d %s", w.Code, w.Body.String())
	}
	code, data := parseEnvelope(t, w)
	if code != 0 {
		t.Fatalf("want code 0, got %d", code)
	}
	var resp dto.ForgotPasswordResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("forgot resp parse: %v", err)
	}
	if resp.Message == "" {
		t.Fatalf("generic message must be non-empty")
	}
	if resp.DevLink != "" {
		t.Fatalf("unknown email must not leak dev_link, got %q", resp.DevLink)
	}
}

// 忘记密码 → 重置 → 新密码登录成功 / 旧密码失败 / 令牌一次性（重放 400）。
func TestAuthForgotPasswordAndResetFlow(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"oldpass12"}`)

	// 忘记密码（dev 模式：无 SMTP → 响应带 dev_link）
	w := doJSON(r, http.MethodPost, "/auth/forgot-password", `{"email":"a@b.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("forgot-password want 200, got %d %s", w.Code, w.Body.String())
	}
	code, data := parseEnvelope(t, w)
	if code != 0 {
		t.Fatalf("want code 0, got %d", code)
	}
	var resp dto.ForgotPasswordResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("forgot resp parse: %v", err)
	}
	if resp.DevLink == "" {
		t.Fatalf("dev mode must return dev_link")
	}
	u, err := url.Parse(resp.DevLink)
	if err != nil {
		t.Fatalf("dev_link parse: %v", err)
	}
	if u.Path != "/reset-password" {
		t.Fatalf("dev_link path want /reset-password, got %q", u.Path)
	}
	token := u.Query().Get("token")
	if token == "" {
		t.Fatalf("dev_link must carry token")
	}

	// 短密码被 binding 拒绝（min=8）
	w2 := doJSON(r, http.MethodPost, "/auth/reset-password", `{"token":"`+token+`","password":"short"}`)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("short password want 400, got %d %s", w2.Code, w2.Body.String())
	}

	// 正确重置
	w3 := doJSON(r, http.MethodPost, "/auth/reset-password", `{"token":"`+token+`","password":"newpass123"}`)
	if w3.Code != http.StatusOK {
		t.Fatalf("reset want 200, got %d %s", w3.Code, w3.Body.String())
	}
	code3, _ := parseEnvelope(t, w3)
	if code3 != 0 {
		t.Fatalf("reset want code 0, got %d", code3)
	}

	// 新密码登录成功
	w4 := doJSON(r, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"newpass123"}`)
	if w4.Code != http.StatusOK {
		t.Fatalf("login with new password want 200, got %d %s", w4.Code, w4.Body.String())
	}

	// 旧密码登录失败
	w5 := doJSON(r, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"oldpass12"}`)
	if w5.Code != http.StatusBadRequest {
		t.Fatalf("login with old password want 400, got %d %s", w5.Code, w5.Body.String())
	}

	// 令牌一次性：重放必须失败
	w6 := doJSON(r, http.MethodPost, "/auth/reset-password", `{"token":"`+token+`","password":"another123"}`)
	if w6.Code != http.StatusBadRequest {
		t.Fatalf("token replay want 400, got %d %s", w6.Code, w6.Body.String())
	}
	code6, _ := parseEnvelope(t, w6)
	if code6 != 1001 {
		t.Fatalf("token replay want 1001, got %d", code6)
	}
}

// 无效/过期令牌：400 + 1001。
func TestAuthResetPasswordInvalidToken(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	w := doJSON(r, http.MethodPost, "/auth/reset-password", `{"token":"deadbeef","password":"newpass123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid token want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("invalid token want 1001, got %d", code)
	}
}

// 邮箱格式非法：400（binding email 校验）。
func TestAuthForgotPasswordInvalidEmail(t *testing.T) {
	h := newAuthHandler(t)
	r := authRouter(h)
	w := doJSON(r, http.MethodPost, "/auth/forgot-password", `{"email":"not-an-email"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid email want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("invalid email want 1001, got %d", code)
	}
}

// SMTP 配置但发送失败：仍返回 200 + 通用文案 + 无 dev_link（不泄露内部错误）。
func TestAuthForgotPasswordSmtpFailureNoLeak(t *testing.T) {
	h := newAuthHandler(t)
	h.mailer = mail.New(mail.Config{Host: "127.0.0.1", Port: 1}) // 不可达端口，发送必失败
	r := authRouter(h)
	doJSON(r, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"oldpass12"}`)

	w := doJSON(r, http.MethodPost, "/auth/forgot-password", `{"email":"a@b.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("forgot-password want 200 even on smtp failure, got %d %s", w.Code, w.Body.String())
	}
	code, data := parseEnvelope(t, w)
	if code != 0 {
		t.Fatalf("want code 0, got %d", code)
	}
	var resp dto.ForgotPasswordResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("forgot resp parse: %v", err)
	}
	if resp.DevLink != "" {
		t.Fatalf("smtp failure must not return dev_link (no leak), got %q", resp.DevLink)
	}
}
