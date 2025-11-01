package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/pkg/accounts"
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

	group.GET("/users", handler.listUsers)
	group.POST("/users", handler.createUser)
	group.DELETE("/users/:id", handler.deleteUser)

	group.GET("/users/:id/api-keys", handler.listUserAPIKeys)
	group.POST("/users/:id/api-keys", handler.createUserAPIKey)
	group.DELETE("/api-keys/:id", handler.deleteUserAPIKey)

	group.GET("/users/:id/upstreams", handler.listUpstreamCredentials)
	group.POST("/users/:id/upstreams", handler.createUpstreamCredential)
	group.DELETE("/upstreams/:id", handler.deleteUpstreamCredential)

	group.POST("/api-keys/:id/binding", handler.bindAPIKey)
	group.GET("/api-keys/:id/binding", handler.getAPIKeyBinding)
}

// RegisterPublicRoutes 注册无需认证的公共路由。
func RegisterPublicRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/healthz", handler.healthz)
	group.POST("/login", handler.login)
}

type createUserRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
}

type createAPIKeyRequest struct {
	Label string `json:"label"`
}

type createUpstreamCredentialRequest struct {
	Provider  string         `json:"provider" binding:"required"`
	Label     string         `json:"label"`
	Plaintext string         `json:"plaintext" binding:"required"`
	Endpoints []string       `json:"endpoints"`
	Metadata  map[string]any `json:"metadata"`
}

type bindAPIKeyRequest struct {
	UserID               string `json:"user_id" binding:"required"`
	UpstreamCredentialID string `json:"upstream_credential_id" binding:"required"`
}

type userResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type apiKeyResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Label      string     `json:"label"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type upstreamCredentialResponse struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Provider  string         `json:"provider"`
	Label     string         `json:"label"`
	Endpoints []string       `json:"endpoints,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type apiKeyBindingResponse struct {
	ID                   string                     `json:"id"`
	UserID               string                     `json:"user_id"`
	UserAPIKeyID         string                     `json:"user_api_key_id"`
	UpstreamCredentialID string                     `json:"upstream_credential_id"`
	Metadata             map[string]any             `json:"metadata,omitempty"`
	CreatedAt            time.Time                  `json:"created_at"`
	UpdatedAt            time.Time                  `json:"updated_at"`
	Upstream             upstreamCredentialResponse `json:"upstream"`
}

func toUserResponse(user accounts.User) userResponse {
	var metadata map[string]any
	if user.Metadata != nil {
		metadata = map[string]any(user.Metadata)
	}
	return userResponse{
		ID:          user.ID,
		Name:        user.Name,
		Description: user.Description,
		Metadata:    metadata,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func toAPIKeyResponse(key accounts.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:         key.ID,
		UserID:     key.UserID,
		Label:      key.Label,
		Prefix:     key.Prefix,
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
	}
}

func toUpstreamCredentialResponse(cred accounts.UpstreamCredential, endpoints []string) upstreamCredentialResponse {
	var metadata map[string]any
	if cred.Metadata != nil {
		metadata = map[string]any(cred.Metadata)
	}
	return upstreamCredentialResponse{
		ID:        cred.ID,
		UserID:    cred.UserID,
		Provider:  cred.Provider,
		Label:     cred.Label,
		Endpoints: endpoints,
		Metadata:  metadata,
		CreatedAt: cred.CreatedAt,
		UpdatedAt: cred.UpdatedAt,
	}
}

func toBindingResponse(binding accounts.UserAPIKeyBinding, upstream upstreamCredentialResponse) apiKeyBindingResponse {
	var metadata map[string]any
	if binding.Metadata != nil {
		metadata = map[string]any(binding.Metadata)
	}
	return apiKeyBindingResponse{
		ID:                   binding.ID,
		UserID:               binding.UserID,
		UserAPIKeyID:         binding.UserAPIKeyID,
		UpstreamCredentialID: binding.UpstreamCredentialID,
		Metadata:             metadata,
		CreatedAt:            binding.CreatedAt,
		UpdatedAt:            binding.UpdatedAt,
		Upstream:             upstream,
	}
}

func decodeEndpoints(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var endpoints []string
	if err := json.Unmarshal(raw, &endpoints); err != nil {
		return nil
	}
	return endpoints
}

func mergeAttrs(base map[string]any, extra map[string]any) map[string]any {
	if extra == nil {
		return base
	}
	if base == nil {
		base = make(map[string]any, len(extra))
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

func (h *Handler) handleAccountsError(c *gin.Context, action string, err error, attrs map[string]any) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrAccountsUnavailable):
		status = http.StatusNotImplemented
	case errors.Is(err, accounts.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, accounts.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, accounts.ErrNotFound):
		status = http.StatusNotFound
	}
	metrics.ObserveAdminAction(action, false)
	attrCopy := mergeAttrs(map[string]any{
		"user":   currentAdminUser(c),
		"action": action,
	}, attrs)
	h.logError("accounts action failed", err, attrCopy)
	c.JSON(status, gin.H{"error": err.Error()})
	return true
}

func (h *Handler) createUser(c *gin.Context) {
	action := "accounts.users.create"
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.ObserveAdminAction(action, false)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.service.CreateUser(c.Request.Context(), accounts.CreateUserParams{
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	})
	if h.handleAccountsError(c, action, err, nil) {
		return
	}
	metrics.ObserveAdminAction(action, true)
	h.logInfo("user created", map[string]any{
		"user":   currentAdminUser(c),
		"userID": user.ID,
	})
	c.JSON(http.StatusCreated, toUserResponse(user))
}

func (h *Handler) listUsers(c *gin.Context) {
	action := "accounts.users.list"
	users, err := h.service.ListUsers(c.Request.Context())
	if h.handleAccountsError(c, action, err, nil) {
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, user := range users {
		resp = append(resp, toUserResponse(user))
	}
	metrics.ObserveAdminAction(action, true)
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *Handler) deleteUser(c *gin.Context) {
	action := "accounts.users.delete"
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.service.DeleteUser(c.Request.Context(), id)
	if h.handleAccountsError(c, action, err, map[string]any{"target_user": id}) {
		return
	}
	metrics.ObserveAdminAction(action, true)
	h.logInfo("user deleted", map[string]any{
		"user":        currentAdminUser(c),
		"target_user": id,
	})
	c.Status(http.StatusNoContent)
}

func (h *Handler) createUserAPIKey(c *gin.Context) {
	action := "accounts.api_keys.create"
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.ObserveAdminAction(action, false)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key, secret, err := h.service.CreateUserAPIKey(c.Request.Context(), accounts.CreateAPIKeyParams{
		UserID: userID,
		Label:  req.Label,
	})
	if h.handleAccountsError(c, action, err, map[string]any{"target_user": userID}) {
		return
	}
	metrics.ObserveAdminAction(action, true)
	h.logInfo("api key created", map[string]any{
		"user":        currentAdminUser(c),
		"target_user": userID,
		"api_key_id":  key.ID,
	})
	c.JSON(http.StatusCreated, gin.H{
		"api_key": toAPIKeyResponse(key),
		"secret":  secret,
	})
}

func (h *Handler) listUserAPIKeys(c *gin.Context) {
	action := "accounts.api_keys.list"
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}
	keys, err := h.service.ListUserAPIKeys(c.Request.Context(), userID)
	if h.handleAccountsError(c, action, err, map[string]any{"target_user": userID}) {
		return
	}
	resp := make([]apiKeyResponse, 0, len(keys))
	for _, key := range keys {
		resp = append(resp, toAPIKeyResponse(key))
	}
	metrics.ObserveAdminAction(action, true)
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *Handler) deleteUserAPIKey(c *gin.Context) {
	action := "accounts.api_keys.delete"
	apiKeyID := c.Param("id")
	if apiKeyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api key id is required"})
		return
	}
	err := h.service.RevokeUserAPIKey(c.Request.Context(), apiKeyID)
	if h.handleAccountsError(c, action, err, map[string]any{"api_key_id": apiKeyID}) {
		return
	}
	metrics.ObserveAdminAction(action, true)
	h.logInfo("api key revoked", map[string]any{
		"user":       currentAdminUser(c),
		"api_key_id": apiKeyID,
	})
	c.Status(http.StatusNoContent)
}

func (h *Handler) createUpstreamCredential(c *gin.Context) {
	action := "accounts.upstreams.create"
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}
	var req createUpstreamCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.ObserveAdminAction(action, false)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cred, err := h.service.CreateUpstreamCredential(c.Request.Context(), accounts.CreateUpstreamCredentialParams{
		UserID:    userID,
		Provider:  req.Provider,
		Label:     req.Label,
		Plaintext: req.Plaintext,
		Endpoints: req.Endpoints,
		Metadata:  req.Metadata,
	})
	if h.handleAccountsError(c, action, err, map[string]any{"target_user": userID}) {
		return
	}
	endpoints := decodeEndpoints(cred.Endpoints)
	metrics.ObserveAdminAction(action, true)
	h.logInfo("upstream credential created", map[string]any{
		"user":       currentAdminUser(c),
		"credential": cred.ID,
		"provider":   cred.Provider,
	})
	c.JSON(http.StatusCreated, toUpstreamCredentialResponse(cred, endpoints))
}

func (h *Handler) listUpstreamCredentials(c *gin.Context) {
	action := "accounts.upstreams.list"
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}
	creds, err := h.service.ListUpstreamCredentials(c.Request.Context(), userID)
	if h.handleAccountsError(c, action, err, map[string]any{"target_user": userID}) {
		return
	}
	resp := make([]upstreamCredentialResponse, 0, len(creds))
	for _, cred := range creds {
		resp = append(resp, toUpstreamCredentialResponse(cred, decodeEndpoints(cred.Endpoints)))
	}
	metrics.ObserveAdminAction(action, true)
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *Handler) deleteUpstreamCredential(c *gin.Context) {
	action := "accounts.upstreams.delete"
	credentialID := c.Param("id")
	if credentialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential id is required"})
		return
	}
	err := h.service.DeleteUpstreamCredential(c.Request.Context(), credentialID)
	if h.handleAccountsError(c, action, err, map[string]any{"credential": credentialID}) {
		return
	}
	metrics.ObserveAdminAction(action, true)
	h.logInfo("upstream credential deleted", map[string]any{
		"user":       currentAdminUser(c),
		"credential": credentialID,
	})
	c.Status(http.StatusNoContent)
}

func (h *Handler) bindAPIKey(c *gin.Context) {
	action := "accounts.api_keys.bind"
	apiKeyID := c.Param("id")
	if apiKeyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api key id is required"})
		return
	}
	var req bindAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.ObserveAdminAction(action, false)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	binding, err := h.service.BindAPIKey(c.Request.Context(), accounts.BindAPIKeyParams{
		UserID:               req.UserID,
		UserAPIKeyID:         apiKeyID,
		UpstreamCredentialID: req.UpstreamCredentialID,
	})
	if h.handleAccountsError(c, action, err, map[string]any{"api_key_id": apiKeyID}) {
		return
	}
	_, cred, err := h.service.GetBindingByAPIKeyID(c.Request.Context(), apiKeyID)
	if h.handleAccountsError(c, action, err, map[string]any{"api_key_id": apiKeyID}) {
		return
	}
	resp := toBindingResponse(binding, toUpstreamCredentialResponse(cred, decodeEndpoints(cred.Endpoints)))
	metrics.ObserveAdminAction(action, true)
	h.logInfo("api key bound", map[string]any{
		"user":       currentAdminUser(c),
		"api_key_id": apiKeyID,
		"credential": cred.ID,
	})
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) getAPIKeyBinding(c *gin.Context) {
	action := "accounts.api_keys.binding.get"
	apiKeyID := c.Param("id")
	if apiKeyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api key id is required"})
		return
	}
	binding, cred, err := h.service.GetBindingByAPIKeyID(c.Request.Context(), apiKeyID)
	if h.handleAccountsError(c, action, err, map[string]any{"api_key_id": apiKeyID}) {
		return
	}
	resp := toBindingResponse(binding, toUpstreamCredentialResponse(cred, decodeEndpoints(cred.Endpoints)))
	metrics.ObserveAdminAction(action, true)
	c.JSON(http.StatusOK, resp)
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
