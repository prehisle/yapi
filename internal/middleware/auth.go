package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminBasicAuth 采用 Basic Auth 校验管理端访问。
// 若用户名或密码为空则跳过校验，允许匿名访问。
func AdminBasicAuth(username, password string) gin.HandlerFunc {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	credential := username + ":" + password
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(credential))
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == expected {
			c.Next()
			return
		}
		c.Header("WWW-Authenticate", `Basic realm="admin"`)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}
