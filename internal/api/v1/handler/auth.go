package handler

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/pkg/mail"
	"fxcore/internal/store"
)

// refreshTTL Refresh Token 有效期（文档 6.1：7 天，Redis 存储）。
const refreshTTL = 7 * 24 * time.Hour

// resetTTL 密码重置令牌有效期（1 小时）。
const resetTTL = time.Hour

// resetMailSubject 重置邮件主题（ASCII，避免 RFC 2047 编码开销）。
const resetMailSubject = "[FXcore] Password Reset Request"

// AuthHandler 认证处理器（HttpOnly Cookie + Refresh 轮换）。
type AuthHandler struct {
	store        *store.Store
	jm           *jwt.Manager
	tokens       cache.TokenStore
	resets       cache.ResetStore // 密码重置令牌（独立命名空间，防被 refresh 消费）
	mailer       *mail.Mailer     // SMTP 邮件器（未配置 = dev 模式，链接写日志 + 响应返回）
	frontendBase string           // 重置链接前缀（FRONTEND_BASE_URL）
	accessTTL    time.Duration
}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler(s *store.Store, jm *jwt.Manager, tokens cache.TokenStore, resets cache.ResetStore, mailer *mail.Mailer, frontendBase string, accessTTL time.Duration) *AuthHandler {
	return &AuthHandler{store: s, jm: jm, tokens: tokens, resets: resets, mailer: mailer, frontendBase: frontendBase, accessTTL: accessTTL}
}

// Login POST /auth/login
// 校验账密 -> 签发 access token（HttpOnly Cookie）-> 生成 refresh token（7 天）。
// 响应携带 sign_secret（请求签名 user_secret）与 refresh_token（扩展字段，见复盘）。
// Login 登录。
// @Summary 登录
// @Description 校验账密，下发 access token（HttpOnly Cookie）与 refresh token / sign_secret
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.LoginRequest true "登录凭据"
// @Success 200 {object} dto.ApiResponse[dto.LoginResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 429 {object} dto.ErrorResponse "1005 登录限流（5 次/分钟/IP）"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	// 注册时已 ToLower 落库，登录同规范化（大小写变体一致命中）
	u, ok := h.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if !ok || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid email or password", nil))
		return
	}
	access, err := h.jm.Issue(u.ID, u.Email)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("issue token failed"))
		return
	}
	refresh := store.NewRefreshToken()
	if err := h.tokens.Set(c.Request.Context(), refresh, u.ID, refreshTTL); err != nil {
		middleware.WriteError(c, middleware.Internal("store refresh token failed"))
		return
	}
	middleware.SetAccessCookie(c, access, int(h.accessTTL.Seconds()))
	middleware.WriteOK(c, dto.LoginResponse{
		User:         userToDTO(u),
		AccessToken:  access,
		SignSecret:   u.SignSecret,
		RefreshToken: refresh,
	})
}

// Register POST /auth/register
// 注册：校验参数 -> 创建用户（email 唯一）-> 签发 access token + refresh token + sign_secret。
// Register 注册。
// @Summary 注册
// @Description 创建账号并直接登录（响应结构与 /auth/login 一致）
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RegisterRequest true "注册信息（密码 ≥8 位）"
// @Success 200 {object} dto.ApiResponse[dto.LoginResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 邮箱已注册"
// @Failure 429 {object} dto.ErrorResponse "1005 注册限流（5 次/分钟/IP）"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	if _, ok := h.store.GetUserByEmail(req.Email); ok {
		middleware.WriteError(c, middleware.BadRequest("email already registered", nil))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("hash password failed"))
		return
	}
	nickname := req.Nickname
	if nickname == "" {
		nickname = strings.Split(req.Email, "@")[0]
	}
	u := &model.User{
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: string(hash),
		Nickname:     nickname,
		SignSecret:   store.NewSignSecret(),
	}
	if err := h.store.CreateUser(u); err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			middleware.WriteError(c, middleware.BadRequest("email already registered", nil))
			return
		}
		middleware.WriteError(c, middleware.Internal("create user failed"))
		return
	}
	access, err := h.jm.Issue(u.ID, u.Email)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("issue token failed"))
		return
	}
	refresh := store.NewRefreshToken()
	if err := h.tokens.Set(c.Request.Context(), refresh, u.ID, refreshTTL); err != nil {
		middleware.WriteError(c, middleware.Internal("store refresh token failed"))
		return
	}
	middleware.SetAccessCookie(c, access, int(h.accessTTL.Seconds()))
	middleware.WriteOK(c, dto.LoginResponse{
		User:         userToDTO(u),
		AccessToken:  access,
		SignSecret:   u.SignSecret,
		RefreshToken: refresh,
	})
}

// Refresh POST /auth/refresh（Refresh 轮换）
// 原子轮换：旧 token 删除 + 新 token 写入一步完成（S4：并发用同一 token 刷新只有一个成功，
// 轮换后的旧 token 立即作废，重放即 1101）。
// Refresh 刷新令牌（轮换制：旧 token 立即作废，重放返回 1101）。
// @Summary 刷新令牌
// @Description 轮换 refresh token：旧 token 作废并返回新 token + 新 access token + sign_secret
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RefreshRequest true "旧 refresh token"
// @Success 200 {object} dto.ApiResponse[dto.RefreshResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 401 {object} dto.ErrorResponse "1101 refresh token 无效或已轮换"
// @Failure 429 {object} dto.ErrorResponse "1005 刷新限流"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	newRefresh := store.NewRefreshToken()
	userID, ok, err := h.tokens.Rotate(c.Request.Context(), req.RefreshToken, newRefresh, refreshTTL)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("rotate refresh token failed: "+err.Error()))
		return
	}
	if !ok {
		middleware.ClearAccessCookie(c)
		middleware.WriteError(c, middleware.RefreshInvalid("refresh token invalid or expired"))
		return
	}
	u, ok := h.store.GetUserByID(userID)
	if !ok {
		middleware.ClearAccessCookie(c)
		middleware.WriteError(c, middleware.RefreshInvalid("user not found"))
		return
	}
	access, err := h.jm.Issue(u.ID, u.Email)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("issue token failed"))
		return
	}
	middleware.SetAccessCookie(c, access, int(h.accessTTL.Seconds()))
	middleware.WriteOK(c, dto.RefreshResponse{AccessToken: access, RefreshToken: newRefresh, SignSecret: u.SignSecret})
}

// Logout POST /auth/logout：作废 refresh token + 清除 Cookie。
// Logout 登出（吊销 refresh token + 清除 Cookie）。
// @Summary 登出
// @Description 携带 refresh_token 时服务端吊销；始终清除 access Cookie
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RefreshRequest false "refresh token（可选，携带则吊销）"
// @Success 200 {object} dto.ApiResponse[any]
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.RefreshRequest
	_ = c.ShouldBindJSON(&req) // 可选 body
	if req.RefreshToken != "" {
		_ = h.tokens.Del(ctx, req.RefreshToken)
	}
	middleware.ClearAccessCookie(c)
	middleware.WriteOK[any](c, nil)
}

// ForgotPassword POST /auth/forgot-password
// 防用户枚举：无论邮箱是否注册，响应一致（200 + 通用文案）。
// SMTP 未配置时（dev 模式）：重置链接写服务日志，并在响应 dev_link 字段返回
// （仅开发环境便利，生产必须配置 SMTP_HOST）。
// ForgotPassword 忘记密码（发送重置邮件）。
// @Summary 忘记密码
// @Description 向注册邮箱发送密码重置链接；邮箱不存在时同样返回成功（防枚举）。
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.ForgotPasswordRequest true "注册邮箱"
// @Success 200 {object} dto.ApiResponse[dto.ForgotPasswordResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 429 {object} dto.ErrorResponse "1005 限流（5 次/分钟/IP）"
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	// 通用文案：不区分邮箱是否存在（防枚举）
	resp := dto.ForgotPasswordResponse{Message: "if the email is registered, a reset email has been sent"}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, ok := h.store.GetUserByEmail(email)
	if !ok {
		middleware.WriteOK(c, resp)
		return
	}
	token := store.NewRefreshToken()
	if err := h.resets.Set(c.Request.Context(), token, u.ID, resetTTL); err != nil {
		middleware.WriteError(c, middleware.Internal("store reset token failed"))
		return
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(h.frontendBase, "/"), url.QueryEscape(token))
	if h.mailer != nil && h.mailer.Enabled() {
		body := "You requested a password reset for your FXcore account.\n\n" +
			"Click the link below to set a new password (valid for 1 hour):\n\n" +
			link + "\n\n" +
			"If you did not request this, you can safely ignore this email."
		if err := h.mailer.Send(u.Email, resetMailSubject, body); err != nil {
			log.Printf("[auth] send reset email to %s failed: %v", u.Email, err)
			// 不泄露内部错误：响应与邮箱未注册时一致（防枚举）
			middleware.WriteOK(c, resp)
			return
		}
	} else {
		// dev 模式（SMTP 未配置）：链接写日志 + 随响应返回，保证无邮件服务器也可联调
		log.Printf("[auth] DEV MODE password reset link for %s: %s", u.Email, link)
		resp.DevLink = link
	}
	middleware.WriteOK(c, resp)
}

// ResetPassword POST /auth/reset-password
// 校验一次性令牌 -> 更新密码哈希 -> 消耗令牌（同一 token 只能成功重置一次）。
// ResetPassword 重置密码。
// @Summary 重置密码
// @Description 携带邮件链接中的一次性令牌设置新密码；成功后令牌立即作废
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.ResetPasswordRequest true "重置令牌 + 新密码（≥8 位）"
// @Success 200 {object} dto.ApiResponse[dto.ResetPasswordResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 令牌无效或已过期"
// @Failure 429 {object} dto.ErrorResponse "1005 限流（5 次/分钟/IP）"
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	ctx := c.Request.Context()
	userID, ok := h.resets.Get(ctx, req.Token)
	if !ok {
		middleware.WriteError(c, middleware.BadRequest("invalid or expired reset token", nil))
		return
	}
	u, ok := h.store.GetUserByID(userID)
	if !ok {
		_ = h.resets.Del(ctx, req.Token)
		middleware.WriteError(c, middleware.BadRequest("invalid or expired reset token", nil))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		middleware.WriteError(c, middleware.Internal("hash password failed"))
		return
	}
	if err := h.store.UpdateUserPassword(u.ID, string(hash)); err != nil {
		middleware.WriteError(c, middleware.Internal("update password failed"))
		return
	}
	_ = h.resets.Del(ctx, req.Token) // 一次性令牌：成功后立即作废
	middleware.WriteOK(c, dto.ResetPasswordResponse{Message: "password reset successful"})
}

// userToDTO model.User -> dto.UserDTO。
func userToDTO(u *model.User) dto.UserDTO {
	return dto.UserDTO{ID: u.ID, Email: u.Email, Nickname: u.Nickname, Avatar: u.Avatar}
}
