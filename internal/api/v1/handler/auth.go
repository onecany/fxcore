package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/cache"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

// refreshTTL Refresh Token 有效期（文档 6.1：7 天，Redis 存储）。
const refreshTTL = 7 * 24 * time.Hour

// AuthHandler 认证处理器（HttpOnly Cookie + Refresh 轮换）。
type AuthHandler struct {
	store     *store.Store
	jm        *jwt.Manager
	tokens    cache.TokenStore
	accessTTL time.Duration
}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler(s *store.Store, jm *jwt.Manager, tokens cache.TokenStore, accessTTL time.Duration) *AuthHandler {
	return &AuthHandler{store: s, jm: jm, tokens: tokens, accessTTL: accessTTL}
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
	u, ok := h.store.GetUserByEmail(req.Email)
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

// userToDTO model.User -> dto.UserDTO。
func userToDTO(u *model.User) dto.UserDTO {
	return dto.UserDTO{ID: u.ID, Email: u.Email, Nickname: u.Nickname, Avatar: u.Avatar}
}
