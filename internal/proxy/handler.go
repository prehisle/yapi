package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"

	"github.com/prehisle/yapi/internal/middleware"
	"github.com/prehisle/yapi/pkg/accounts"
	"github.com/prehisle/yapi/pkg/metrics"
	"github.com/prehisle/yapi/pkg/rules"
)

// ErrNoMatchingRule 表示没有规则匹配当前请求。
var ErrNoMatchingRule = errors.New("no matching rule")

// Handler 负责根据规则转发请求。
type Handler struct {
	service        rules.Service
	accountService accounts.Service
	defaultTarget  *url.URL
	transport      http.RoundTripper
	logger         *slog.Logger
}

// Option 定义 Handler 可配参数。
type Option func(*Handler)

// WithDefaultTarget 设置默认的上游地址。
func WithDefaultTarget(u *url.URL) Option {
	return func(h *Handler) {
		h.defaultTarget = u
	}
}

// WithTransport 自定义 HTTP 传输层，实现如链路追踪等能力。
func WithTransport(rt http.RoundTripper) Option {
	return func(h *Handler) {
		h.transport = rt
	}
}

// WithLogger 设置结构化日志记录器。
func WithLogger(logger *slog.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

// NewHandler 创建 Proxy Handler。
func NewHandler(service rules.Service, opts ...Option) *Handler {
	h := &Handler{
		service: service,
		transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.logger == nil {
		h.logger = slog.Default()
	}
	h.transport = wrapWithMetricsTransport(h.transport)
	return h
}

// RegisterRoutes 将代理注册为全局 fallback。
func RegisterRoutes(engine *gin.Engine, handler *Handler) {
	engine.NoRoute(handler.Handle)
	engine.NoMethod(handler.Handle)
}

// Handle 转发任意未命中的请求。
func (h *Handler) Handle(c *gin.Context) {
	binding, hasBinding := middleware.CurrentBinding(c)
	upstreamInfo, hasUpstream := middleware.CurrentUpstreamInfo(c)
	if hasBinding && hasUpstream {
		if err := h.authorizeBinding(c, binding, upstreamInfo); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
	}
	rule, err := h.matchRule(c)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrNoMatchingRule) {
			status = http.StatusNotFound
			if h.logger != nil {
				h.logger.Info("no matching rule",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
				)
			}
		}
		if h.logger != nil && !errors.Is(err, ErrNoMatchingRule) {
			h.logger.Error("match rule failed",
				"error", err,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
			)
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	targetURL, err := h.resolveTarget(c, rule)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("resolve target failed",
				"error", err,
				"rule_id", rule.ID,
				"path", c.Request.URL.Path,
			)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = h.transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		middleware.WithRequestID(req, middleware.RequestIDFromContext(c))
		if err := h.applyRuleActions(c, req, rule); err != nil {
			req.Header.Add("X-YAPI-Body-Rewrite-Error", err.Error())
			if h.logger != nil {
				h.logger.Warn("apply rule actions failed",
					"error", err,
					"rule_id", rule.ID,
					"path", req.URL.Path,
					"method", req.Method,
				)
			}
		}
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
		status := http.StatusBadGateway
		if errors.Is(proxyErr, context.Canceled) {
			status = 499 // 客户端主动取消
		}
		http.Error(rw, proxyErr.Error(), status)
	}
	start := time.Now()
	rec := &responseRecorder{ResponseWriter: c.Writer, status: http.StatusOK}
	proxy.ServeHTTP(rec, c.Request)
	if h.logger != nil {
		h.logger.Info("proxy upstream",
			"request_id", middleware.RequestIDFromContext(c),
			"rule_id", rule.ID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"target", targetURL.Host,
			"status", rec.status,
			"bytes", rec.bytes,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

func (h *Handler) matchRule(c *gin.Context) (rules.Rule, error) {
	allRules, err := h.service.ListRules(c.Request.Context())
	if err != nil {
		return rules.Rule{}, err
	}
	for _, rule := range allRules {
		if !rule.Enabled {
			continue
		}
		if matchesRequest(c, rule.Matcher) {
			return rule, nil
		}
	}
	if h.defaultTarget != nil {
		return rules.Rule{
			ID:       "default",
			Priority: -1,
			Matcher:  rules.Matcher{PathPrefix: "/"},
			Actions: rules.Actions{
				SetTargetURL: h.defaultTarget.String(),
			},
			Enabled: true,
		}, nil
	}
	return rules.Rule{}, ErrNoMatchingRule
}

func matchesRequest(c *gin.Context, matcher rules.Matcher) bool {
	if matcher.PathPrefix != "" && !strings.HasPrefix(c.FullPath(), matcher.PathPrefix) && !strings.HasPrefix(c.Request.URL.Path, matcher.PathPrefix) {
		return false
	}
	if len(matcher.Methods) > 0 {
		methodMatched := false
		for _, method := range matcher.Methods {
			if strings.EqualFold(c.Request.Method, method) {
				methodMatched = true
				break
			}
		}
		if !methodMatched {
			return false
		}
	}
	for key, pattern := range matcher.Headers {
		headerValue := c.GetHeader(key)
		if pattern == "" && headerValue == "" {
			continue
		}
		matched, err := regexp.MatchString(pattern, headerValue)
		if err != nil || !matched {
			return false
		}
	}

	apiKey, hasAPIKey := middleware.CurrentAPIKey(c)
	rawKey, _ := middleware.RawAPIKey(c)
	binding, hasBinding := middleware.CurrentBinding(c)
	upstreamInfo, hasUpstream := middleware.CurrentUpstreamInfo(c)
	user, hasUser := middleware.CurrentUser(c)
	if matcher.RequireBinding {
		if !hasBinding {
			return false
		}
	}
	if len(matcher.APIKeyIDs) > 0 {
		if !hasAPIKey {
			return false
		}
		if !stringInSliceTrimmed(matcher.APIKeyIDs, apiKey.ID, false) {
			return false
		}
	}
	if len(matcher.APIKeyPrefixes) > 0 {
		prefix := strings.TrimSpace(apiKey.Prefix)
		if prefix == "" && rawKey != "" {
			parts := strings.Split(rawKey, "_")
			if len(parts) >= 3 {
				prefix = parts[1]
			}
		}
		if prefix == "" || !stringInSliceTrimmed(matcher.APIKeyPrefixes, prefix, true) {
			return false
		}
	}
	if len(matcher.UserIDs) > 0 {
		if !hasUser {
			return false
		}
		if !stringInSliceTrimmed(matcher.UserIDs, user.ID, false) {
			return false
		}
	}
	if len(matcher.UserMetadata) > 0 {
		if !hasUser || user.Metadata == nil {
			return false
		}
		if !metadataContains(map[string]any(user.Metadata), matcher.UserMetadata) {
			return false
		}
	}
	if len(matcher.BindingUpstreamIDs) > 0 {
		if !hasBinding {
			return false
		}
		if !stringInSliceTrimmed(matcher.BindingUpstreamIDs, binding.UpstreamCredentialID, false) {
			return false
		}
	}
	if len(matcher.BindingProviders) > 0 {
		if !hasUpstream {
			return false
		}
		if !stringInSliceTrimmed(matcher.BindingProviders, upstreamInfo.Credential.Provider, true) {
			return false
		}
	}
	return true
}

func (h *Handler) resolveTarget(c *gin.Context, rule rules.Rule) (*url.URL, error) {
	if info, ok := middleware.CurrentUpstreamInfo(c); ok {
		if len(info.Endpoints) > 0 {
			target, err := url.Parse(strings.TrimSpace(info.Endpoints[0]))
			if err == nil {
				return target, nil
			}
		}
	}
	target := rule.Actions.SetTargetURL
	if target == "" && h.defaultTarget != nil {
		return h.defaultTarget, nil
	}
	if target == "" {
		return nil, errors.New("rule target not configured")
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (h *Handler) applyRuleActions(c *gin.Context, req *http.Request, rule rules.Rule) error {
	actions := rule.Actions
	for key, value := range actions.SetHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range actions.AddHeaders {
		req.Header.Add(key, value)
	}
	for _, key := range actions.RemoveHeaders {
		req.Header.Del(key)
	}
	if auth := strings.TrimSpace(actions.SetAuthorization); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	if expr := actions.RewritePathRegex; expr != nil {
		re, err := regexp.Compile(expr.Pattern)
		if err == nil {
			req.URL.Path = re.ReplaceAllString(req.URL.Path, expr.Replace)
		}
	}
	if len(actions.OverrideJSON) > 0 || len(actions.RemoveJSON) > 0 {
		if err := rewriteJSONBody(req, actions.OverrideJSON, actions.RemoveJSON); err != nil {
			return err
		}
	}
	if info, ok := middleware.CurrentUpstreamInfo(c); ok {
		if apiKey := strings.TrimSpace(info.Credential.APIKey); apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if provider := strings.TrimSpace(info.Credential.Provider); provider != "" {
			req.Header.Set("X-Upstream-Provider", provider)
		}
		if info.Credential.ID != "" {
			req.Header.Set("X-Upstream-Credential-ID", info.Credential.ID)
		}
	}
	if user, ok := middleware.CurrentUser(c); ok && strings.TrimSpace(user.ID) != "" {
		req.Header.Set("X-YAPI-User-ID", user.ID)
	}
	return nil
}

func rewriteJSONBody(req *http.Request, override map[string]any, remove []string) error {
	if req.Body == nil {
		return errors.New("missing request body")
	}
	contentType := strings.ToLower(req.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "application/json") {
		return errors.New("content type is not json")
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if err := req.Body.Close(); err != nil {
		return err
	}
	if len(bodyBytes) == 0 {
		return errors.New("empty body")
	}
	for key, value := range override {
		tokens, err := rules.ParseJSONPath(key)
		if err != nil {
			return err
		}
		normalized := tokensToSJSONPath(tokens)
		bodyBytes, err = sjson.SetBytesOptions(bodyBytes, normalized, value, &sjson.Options{Optimistic: true})
		if err != nil {
			return fmt.Errorf("override path %s: %w", key, err)
		}
	}
	for _, key := range remove {
		tokens, err := rules.ParseJSONPath(key)
		if err != nil {
			return err
		}
		normalized := tokensToSJSONPath(tokens)
		bodyBytes, err = sjson.DeleteBytes(bodyBytes, normalized)
		if err != nil {
			return fmt.Errorf("remove path %s: %w", key, err)
		}
	}
	reader := bytes.NewReader(bodyBytes)
	req.Body = io.NopCloser(reader)
	if req.GetBody != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}
	req.ContentLength = int64(len(bodyBytes))
	if req.Header != nil {
		req.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	}
	return nil
}

func stringInSliceTrimmed(list []string, target string, caseInsensitive bool) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range list {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if caseInsensitive {
			if strings.EqualFold(trimmed, target) {
				return true
			}
		} else if trimmed == target {
			return true
		}
	}
	return false
}

func metadataContains(metadata map[string]any, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	if len(metadata) == 0 {
		return false
	}
	for key, value := range expected {
		actual, ok := metadata[key]
		if !ok {
			return false
		}
		if strings.TrimSpace(fmt.Sprint(actual)) != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func (h *Handler) authorizeBinding(c *gin.Context, binding accounts.UserAPIKeyBinding, upstream middleware.UpstreamInfo) error {
	if upstream.Credential.ID == "" {
		return errors.New("upstream credential missing")
	}
	if binding.UpstreamCredentialID != upstream.Credential.ID {
		return errors.New("binding mismatch upstream")
	}
	if upstream.Credential.UserID != binding.UserID {
		return errors.New("binding ownership mismatch")
	}
	if user, ok := middleware.CurrentUser(c); ok && user.ID != "" && user.ID != binding.UserID {
		return errors.New("api key not authorized for user")
	}
	return nil
}

func tokensToSJSONPath(tokens []rules.JSONPathToken) string {
	parts := make([]string, len(tokens))
	for i, token := range tokens {
		if token.IsKey() {
			parts[i] = token.Key
		} else {
			parts[i] = strconv.Itoa(token.IndexValue())
		}
	}
	return strings.Join(parts, ".")
}

type responseRecorder struct {
	gin.ResponseWriter
	status int
	bytes  int64
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

type metricsRoundTripper struct {
	base http.RoundTripper
}

func wrapWithMetricsTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*metricsRoundTripper); ok {
		return base
	}
	return &metricsRoundTripper{base: base}
}

func (m *metricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.base == nil {
		m.base = http.DefaultTransport
	}
	start := time.Now()
	resp, err := m.base.RoundTrip(req)
	duration := time.Since(start)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	metrics.ObserveUpstream(req.URL.Host, status, duration, err != nil)
	return resp, err
}

// WithAccountsService enables account-aware routing.
func WithAccountsService(accounts accounts.Service) Option {
	return func(h *Handler) {
		h.accountService = accounts
	}
}
