package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/pkg/metrics"
	"github.com/prehisle/yapi/pkg/rules"
)

// Handler 暴露管理端的 REST API。
type Handler struct {
	service Service
	auth    *Authenticator
	logger  *slog.Logger
}

// NewHandler 创建管理端处理器。
func NewHandler(service Service, auth *Authenticator, opts ...Option) *Handler {
	h := &Handler{
		service: service,
		auth:    auth,
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.logger == nil {
		h.logger = slog.Default()
	}
	return h
}

// Option 定义 handler 可选项。
type Option func(*Handler)

// WithLogger 设置结构化日志记录器。
func WithLogger(logger *slog.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
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
	start := time.Now()
	query := parseListRulesQuery(c)
	result, err := h.service.ListRules(c.Request.Context())
	if err != nil {
		h.logError("list rules failed", err, nil)
		metrics.ObserveAdminAction("rules.list", false)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filtered := filterRules(result, query)
	total := len(filtered)
	enabledTotal := countEnabled(filtered)
	page := query.Page
	if total == 0 {
		page = 1
	} else {
		maxPage := (total + query.PageSize - 1) / query.PageSize
		if maxPage == 0 {
			maxPage = 1
		}
		if page > maxPage {
			page = maxPage
		}
	}
	items := paginateRules(filtered, page, query.PageSize)
	resp := listRulesResponse{
		Items:        items,
		Total:        total,
		EnabledTotal: enabledTotal,
		Page:         page,
		PageSize:     query.PageSize,
	}
	h.logInfo("list rules success", map[string]any{
		"user":          currentAdminUser(c),
		"page":          page,
		"page_size":     query.PageSize,
		"search":        query.Search,
		"enabled":       query.Enabled,
		"count":         len(items),
		"enabled_total": enabledTotal,
		"total":         total,
		"latency":       time.Since(start).Milliseconds(),
	})
	metrics.ObserveAdminAction("rules.list", true)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) createOrUpdateRule(c *gin.Context) {
	action := "rules.create"
	if c.Request.Method == http.MethodPut || c.Param("id") != "" {
		action = "rules.update"
	}
	var rule rules.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		metrics.ObserveAdminAction(action, false)
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
		h.logError("save rule failed", err, map[string]any{
			"user":   currentAdminUser(c),
			"rule":   rule.ID,
			"action": action,
		})
		metrics.ObserveAdminAction(action, false)
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.logInfo("rule saved", map[string]any{
		"user":   currentAdminUser(c),
		"rule":   rule.ID,
		"action": action,
	})
	metrics.ObserveAdminAction(action, true)
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
		if errors.Is(err, rules.ErrInvalidRule) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, rules.ErrRuleNotFound) {
			status = http.StatusNotFound
		}
		h.logError("delete rule failed", err, map[string]any{
			"user": currentAdminUser(c),
			"rule": id,
		})
		metrics.ObserveAdminAction("rules.delete", false)
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.logInfo("rule deleted", map[string]any{
		"user": currentAdminUser(c),
		"rule": id,
	})
	metrics.ObserveAdminAction("rules.delete", true)
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
	action := "auth.login"
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.ObserveAdminAction(action, false)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := h.auth.IssueToken(req.Username, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrTokenNotConfigured) {
			status = http.StatusNotImplemented
		}
		h.logError("login failed", err, map[string]any{"username": req.Username})
		metrics.ObserveAdminAction(action, false)
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.logInfo("login success", map[string]any{"username": req.Username})
	metrics.ObserveAdminAction(action, true)
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(h.auth.ttl.Seconds())})
}

type listRulesQuery struct {
	Page     int
	PageSize int
	Search   string
	Enabled  *bool
}

type listRulesResponse struct {
	Items        []rules.Rule `json:"items"`
	Total        int          `json:"total"`
	EnabledTotal int          `json:"enabled_total"`
	Page         int          `json:"page"`
	PageSize     int          `json:"page_size"`
}

func parseListRulesQuery(c *gin.Context) listRulesQuery {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	search := strings.TrimSpace(c.Query("q"))
	var enabled *bool
	if v := strings.TrimSpace(c.Query("enabled")); v != "" && strings.ToLower(v) != "all" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			enabled = &parsed
		}
	}
	return listRulesQuery{
		Page:     page,
		PageSize: pageSize,
		Search:   search,
		Enabled:  enabled,
	}
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func filterRules(all []rules.Rule, query listRulesQuery) []rules.Rule {
	if len(all) == 0 {
		return nil
	}
	var filtered []rules.Rule
	keyword := strings.ToLower(query.Search)
	for _, rule := range all {
		if query.Enabled != nil && rule.Enabled != *query.Enabled {
			continue
		}
		if keyword != "" && !matchesKeyword(rule, keyword) {
			continue
		}
		filtered = append(filtered, rule)
	}
	return filtered
}

func matchesKeyword(rule rules.Rule, keyword string) bool {
	if strings.Contains(strings.ToLower(rule.ID), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(rule.Matcher.PathPrefix), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(rule.Actions.SetTargetURL), keyword) {
		return true
	}
	return false
}

func paginateRules(items []rules.Rule, page, pageSize int) []rules.Rule {
	if pageSize <= 0 || len(items) == 0 {
		return []rules.Rule{}
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []rules.Rule{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func countEnabled(items []rules.Rule) int {
	count := 0
	for _, rule := range items {
		if rule.Enabled {
			count++
		}
	}
	return count
}

func currentAdminUser(c *gin.Context) string {
	if value, exists := c.Get("admin_user"); exists {
		if user, ok := value.(string); ok {
			return user
		}
	}
	return ""
}

func (h *Handler) logError(msg string, err error, attrs map[string]any) {
	if h.logger == nil {
		return
	}
	args := []any{"error", err}
	for k, v := range attrs {
		args = append(args, k, v)
	}
	h.logger.Error(msg, args...)
}

func (h *Handler) logInfo(msg string, attrs map[string]any) {
	if h.logger == nil {
		return
	}
	args := make([]any, 0, len(attrs)*2)
	for k, v := range attrs {
		args = append(args, k, v)
	}
	h.logger.Info(msg, args...)
}
