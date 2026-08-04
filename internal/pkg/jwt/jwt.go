// Package jwt 签发/校验 Access Token（API设计.md 6.1：15 分钟、HttpOnly Cookie 承载）。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 业务声明。
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// Manager JWT 签发与校验器。
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager 构造。secret 为空时返回错误（不允许空密钥）。
func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if secret == "" {
		return nil, errors.New("jwt secret must not be empty")
	}
	return &Manager{secret: []byte(secret), ttl: ttl}, nil
}

// Issue 为用户签发 access token。
func (m *Manager) Issue(userID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Issuer:    "fxcore",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse 校验并解析 token。
func (m *Manager) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer("fxcore"), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token claims")
	}
	if claims.Subject == "" {
		return nil, errors.New("token missing subject") // L9：Subject 必填
	}
	return claims, nil
}
