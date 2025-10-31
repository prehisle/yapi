package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/pkg/rules"
)

// ErrNoMatchingRule 表示没有规则匹配当前请求。
var ErrNoMatchingRule = errors.New("no matching rule")

// Handler 负责根据规则转发请求。
type Handler struct {
	service       rules.Service
	defaultTarget *url.URL
	transport     http.RoundTripper
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
	return h
}

// RegisterRoutes 挂载代理相关路由。
func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.Any("/*proxyPath", handler.handle)
}

func (h *Handler) handle(c *gin.Context) {
	rule, err := h.matchRule(c)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrNoMatchingRule) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	targetURL, err := h.resolveTarget(c, rule)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = h.transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		applyRuleActions(req, rule.Actions)
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
		status := http.StatusBadGateway
		if errors.Is(proxyErr, context.Canceled) {
			status = 499 // 客户端主动取消
		}
		http.Error(rw, proxyErr.Error(), status)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
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
	return true
}

func (h *Handler) resolveTarget(c *gin.Context, rule rules.Rule) (*url.URL, error) {
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

func applyRuleActions(req *http.Request, actions rules.Actions) {
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
			req.Header.Add("X-YAPI-Body-Rewrite-Error", err.Error())
		}
	}
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
	var payload any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return err
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return errors.New("request body is not a json object")
	}
	for key, value := range override {
		setJSONValue(root, key, value)
	}
	for _, key := range remove {
		removeJSONValue(root, key)
	}
	newBody, err := json.Marshal(root)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(newBody)
	req.Body = io.NopCloser(reader)
	if req.GetBody != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(newBody)), nil
		}
	}
	req.ContentLength = int64(len(newBody))
	if req.Header != nil {
		req.Header.Set("Content-Length", strconv.FormatInt(int64(len(newBody)), 10))
	}
	return nil
}

func setJSONValue(root map[string]any, path string, value any) {
	segments := strings.Split(path, ".")
	current := root
	for i, seg := range segments {
		if i == len(segments)-1 {
			current[seg] = value
			return
		}
		next, ok := current[seg]
		if !ok {
			child := make(map[string]any)
			current[seg] = child
			current = child
			continue
		}
		childMap, ok := next.(map[string]any)
		if !ok {
			childMap = make(map[string]any)
			current[seg] = childMap
		}
		current = childMap
	}
}

func removeJSONValue(root map[string]any, path string) {
	segments := strings.Split(path, ".")
	current := root
	for i, seg := range segments {
		if i == len(segments)-1 {
			delete(current, seg)
			return
		}
		next, ok := current[seg]
		if !ok {
			return
		}
		childMap, ok := next.(map[string]any)
		if !ok {
			return
		}
		current = childMap
	}
}
