package admin

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidCredential 当用户名或密码不匹配时返回。
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrTokenNotConfigured 当未配置签发密钥时返回。
	ErrTokenNotConfigured = errors.New("token signing secret not configured")
)

// Authenticator 负责管理员认证与令牌签发。
type Authenticator struct {
	username string
	password string
	secret   []byte
	ttl      time.Duration
}

// NewAuthenticator 创建认证器。
func NewAuthenticator(username, password, tokenSecret string, ttl time.Duration) *Authenticator {
	return &Authenticator{
		username: strings.TrimSpace(username),
		password: password,
		secret:   []byte(tokenSecret),
		ttl:      ttl,
	}
}

// CredentialsConfigured 判断是否配置了用户名密码。
func (a *Authenticator) CredentialsConfigured() bool {
	return a != nil && a.username != "" && a.password != ""
}

// TokenEnabled 判断是否可签发 JWT。
func (a *Authenticator) TokenEnabled() bool {
	return a != nil && len(a.secret) > 0
}

// IssueToken 验证凭证并签发短期访问令牌。
func (a *Authenticator) IssueToken(username, password string) (string, error) {
	if !a.CredentialsConfigured() {
		return "", ErrInvalidCredential
	}
	if !a.matchCredential(username, password) {
		return "", ErrInvalidCredential
	}
	if !a.TokenEnabled() {
		return "", ErrTokenNotConfigured
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   a.username,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

// ValidateToken 校验 JWT 且确认未过期。
func (a *Authenticator) ValidateToken(token string) error {
	if !a.TokenEnabled() {
		return ErrTokenNotConfigured
	}
	if strings.TrimSpace(token) == "" {
		return ErrInvalidCredential
	}
	_, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	})
	return err
}

// Middleware 返回 Gin 中间件验证 Bearer / Basic。
func (a *Authenticator) Middleware() gin.HandlerFunc {
	if a == nil || !a.CredentialsConfigured() {
		// 未配置凭据则跳过认证。
		return func(c *gin.Context) {
			c.Next()
		}
	}
	expectedBasic := "Basic " + basicHeader(a.username, a.password)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") && a.TokenEnabled() {
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if err := a.ValidateToken(token); err == nil {
				c.Set("admin_user", a.username)
				c.Next()
				return
			}
		}
		if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedBasic)) == 1 {
			c.Set("admin_user", a.username)
			c.Next()
			return
		}
		c.Header("WWW-Authenticate", `Basic realm="admin", Bearer realm="admin"`)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

// matchCredential 验证用户名密码是否匹配。
func (a *Authenticator) matchCredential(username, password string) bool {
	if a.username == "" && a.password == "" {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(a.username)) != 1 {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		return false
	}
	return true
}

func basicHeader(username, password string) string {
	raw := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(raw))
}
