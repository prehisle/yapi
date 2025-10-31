package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/pkg/rules"
)

// Handler 暴露管理端的 REST API。
type Handler struct {
	service rules.Service
}

// NewHandler 创建管理端处理器。
func NewHandler(service rules.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 将管理端路由挂载到给定分组。
func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/rules", handler.listRules)
	group.POST("/rules", handler.createOrUpdateRule)
	group.PUT("/rules/:id", handler.createOrUpdateRule)
	group.DELETE("/rules/:id", handler.deleteRule)
	group.GET("/healthz", handler.healthz)
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
	if err := h.service.UpsertRule(c.Request.Context(), rule); err != nil {
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
