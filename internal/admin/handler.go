package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/pkg/rules"
)

// Handler 暴露管理端的 REST API。
type Handler struct {
	service Service
	auth    *Authenticator
}

// NewHandler 创建管理端处理器。
func NewHandler(service Service, auth *Authenticator) *Handler {
	return &Handler{service: service, auth: auth}
}

// RegisterProtectedRoutes 将受保护的管理路由挂载到给定分组。
func RegisterProtectedRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/rules", handler.listRules)
	group.POST("/rules", handler.createOrUpdateRule)
	group.PUT("/rules/:id", handler.createOrUpdateRule)
	group.DELETE("/rules/:id", handler.deleteRule)
}

// RegisterPublicRoutes 注册无需认证的公共路由。
func RegisterPublicRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/healthz", handler.healthz)
	group.POST("/login", handler.login)
}

func (h *Handler) listRules(c *gin.Context) {
	result, err := h.service.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) createOrUpdateRule(c *gin.Context) {
	var rule rules.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if id := c.Param("id"); id != "" && rule.ID == "" {
		rule.ID = id
	}
	if err := h.service.CreateOrUpdateRule(c.Request.Context(), rule); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, rules.ErrInvalidRule) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.service.DeleteRule(c.Request.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, rules.ErrRuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) login(c *gin.Context) {
	if h.auth == nil || !h.auth.CredentialsConfigured() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "login disabled"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := h.auth.IssueToken(req.Username, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrTokenNotConfigured) {
			status = http.StatusNotImplemented
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(h.auth.ttl.Seconds())})
}
