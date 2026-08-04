package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fxcore/internal/pkg/jwt"
)

const userKey = "fx_user_claims"

// accessCookie 名称（HttpOnly）。
const accessCookie = "fx_access_token"

// RequiresAuth 校验 HttpOnly Cookie（或 Authorization: Bearer）中的 access token。
func RequiresAuth(jm *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			WriteError(c, TokenInvalid("missing access token"))
			return
		}
		claims, err := jm.Parse(token)
		if err != nil {
			WriteError(c, TokenInvalid("invalid or expired access token"))
			return
		}
		c.Set(userKey, claims)
		c.Next()
	}
}

// UserClaims 从上下文取当前用户 claims（RequiresAuth 之后可用）。
func UserClaims(c *gin.Context) *jwt.Claims {
	if v, ok := c.Get(userKey); ok {
		if cl, ok := v.(*jwt.Claims); ok {
			return cl
		}
	}
	return nil
}

func extractToken(c *gin.Context) string {
	if v, err := c.Cookie(accessCookie); err == nil && v != "" {
		return v
	}
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// ClearAccessCookie 清除 access cookie（登出/刷新失败时）。
func ClearAccessCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     accessCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetAccessCookie 写入 HttpOnly access cookie。
func SetAccessCookie(c *gin.Context, token string, ttlSeconds int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     accessCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   ttlSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
