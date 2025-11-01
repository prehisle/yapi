//go:build compose_test

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/prehisle/yapi/internal/admin"
	"github.com/prehisle/yapi/internal/middleware"
	"github.com/prehisle/yapi/internal/proxy"
	"github.com/prehisle/yapi/pkg/config"
	"github.com/prehisle/yapi/pkg/rules"
)

const (
	defaultGatewayURL = "http://localhost:8080"
)

func TestGateway_AdminAndProxyIntegration(t *testing.T) {
	loadEnvFile(t)

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store, dbCleanup := setupStore(t, ctx, cfg)
	t.Cleanup(dbCleanup)

	redisClient, cache, eventBus := setupRedis(t, ctx, cfg)
	if redisClient != nil {
		t.Cleanup(func() {
			_ = redisClient.Close()
		})
	}

	var opts []rules.ServiceOption
	if cache != nil {
		opts = append(opts, rules.WithCache(cache))
	}
	if eventBus != nil {
		opts = append(opts, rules.WithEventBus(eventBus))
	}
	ruleService := rules.NewService(store, opts...)
	ruleService.StartBackgroundSync(ctx)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	upstream := startUpstream(t)
	t.Cleanup(upstream.Close)

	server := startGateway(t, cfg, logger, ruleService)
	t.Cleanup(server.Close)

	baseURL := server.URL()
	if baseURL == "" {
		baseURL = defaultGatewayURL
	}

	client := &http.Client{Timeout: 5 * time.Second}

	adminUser := strings.TrimSpace(getEnvDefault("ADMIN_USERNAME", cfg.AdminUsername))
	adminPass := getEnvDefault("ADMIN_PASSWORD", cfg.AdminPassword)

	token := loginAndGetToken(t, client, baseURL, adminUser, adminPass)

	pathPrefix := fmt.Sprintf("/int-%s", strings.ToLower(uuid.NewString())[:8])
	ruleID := fmt.Sprintf("it-%s", uuid.NewString())

	rule := rules.Rule{
		ID:       ruleID,
		Priority: 9999,
		Matcher: rules.Matcher{
			PathPrefix: pathPrefix,
			Methods:    []string{"POST"},
		},
		Actions: rules.Actions{
			SetTargetURL: upstream.TargetURL(),
			SetHeaders: map[string]string{
				"X-Test-Integration": "true",
			},
			SetAuthorization: "Bearer upstream-token",
			OverrideJSON: map[string]any{
				"messages[0].role":    "assistant",
				"messages[0].content": "rewritten by gateway",
			},
			RemoveJSON: []string{"metadata.debug"},
			RewritePathRegex: &rules.RewritePathExpression{
				Pattern: fmt.Sprintf("^%s", pathPrefix),
				Replace: "/anything",
			},
		},
		Enabled: true,
	}

	upsertRule(t, client, baseURL, token, rule)
	t.Cleanup(func() {
		deleteRule(t, client, baseURL, token, ruleID)
	})

	assertRuleVisible(t, client, baseURL, token, ruleID)

	body := `{"messages":[{"role":"user","content":"original"}],"metadata":{"debug":true,"trace_id":"foo"}}`
	status, data, err := doProxyRequest(client, baseURL, pathPrefix, body)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, status, "gateway proxy status unexpected body=%s", string(data))

	var respBody map[string]any
	require.NoErrorf(t, json.Unmarshal(data, &respBody), "proxy response not json: %s", string(data))

	call := upstream.WaitForCall(t)
	require.Equal(t, "/anything/chat", call.Path)
	require.Equal(t, http.MethodPost, call.Method)
	require.Equal(t, "Bearer upstream-token", call.Headers.Get("Authorization"))
	require.Equal(t, "true", call.Headers.Get("X-Test-Integration"))

	require.Len(t, call.Body.Messages, 1)
	require.Equal(t, "assistant", call.Body.Messages[0].Role)
	require.Equal(t, "rewritten by gateway", call.Body.Messages[0].Content)
	if call.Body.Metadata != nil {
		_, exists := call.Body.Metadata["debug"]
		require.False(t, exists, "metadata.debug should be stripped")
		require.Equal(t, "foo", call.Body.Metadata["trace_id"])
	}
}

func startGateway(t *testing.T, cfg config.Config, logger *slog.Logger, ruleService rules.Service) *httptestServer {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID(), middleware.AccessLogger(logger))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	adminAuth := admin.NewAuthenticator(cfg.AdminUsername, cfg.AdminPassword, cfg.AdminTokenSecret, cfg.AdminTokenTTL)
	adminService := admin.NewService(ruleService)
	adminHandler := admin.NewHandler(adminService, adminAuth, admin.WithLogger(logger))

	adminGroup := router.Group("/admin")
	admin.RegisterPublicRoutes(adminGroup, adminHandler)
	protected := adminGroup.Group("")
	protected.Use(adminAuth.Middleware())
	admin.RegisterProtectedRoutes(protected, adminHandler)

	defaultTarget := mustParseURL(cfg.UpstreamBaseURL)
	proxyHandler := proxy.NewHandler(ruleService, proxy.WithLogger(logger), proxy.WithDefaultTarget(defaultTarget))
	proxy.RegisterRoutes(router, proxyHandler)

	server := &httptestServer{
		Engine: router,
		Server: httptest.NewUnstartedServer(router),
	}
	server.Server.Start()
	return server
}

type httptestServer struct {
	Engine *gin.Engine
	Server *httptest.Server
}

func (s *httptestServer) Close() {
	if s != nil && s.Server != nil {
		s.Server.Close()
	}
}

func (s *httptestServer) URL() string {
	if s == nil || s.Server == nil {
		return ""
	}
	return s.Server.URL
}

func setupStore(t *testing.T, ctx context.Context, cfg config.Config) (rules.Store, func()) {
	t.Helper()
	if cfg.DatabaseDSN == "" {
		return rules.NewMemoryStore(), func() {}
	}
	gormLogger := logger.New(log.New(os.Stdout, "gorm: ", log.LstdFlags), logger.Config{
		SlowThreshold: time.Second,
		LogLevel:      logger.Silent,
	})
	db, err := gorm.Open(postgres.Open(cfg.DatabaseDSN), &gorm.Config{
		Logger: gormLogger,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.PingContext(ctx))

	store := rules.NewDBStore(db)
	require.NoError(t, store.AutoMigrate(ctx))
	require.NoError(t, db.WithContext(ctx).Exec("TRUNCATE TABLE rule_records").Error)

	cleanup := func() {
		_ = sqlDB.Close()
	}
	return store, cleanup
}

func setupRedis(t *testing.T, ctx context.Context, cfg config.Config) (*redis.Client, rules.Cache, rules.EventBus) {
	t.Helper()
	if cfg.RedisAddr == "" {
		return nil, nil, nil
	}
	options := &redis.Options{Addr: cfg.RedisAddr}
	switch cfg.RedisMaintMode {
	case config.RedisMaintModeAuto:
		options.MaintNotificationsConfig = maintnotifications.DefaultConfig()
	case config.RedisMaintModeEnabled:
		options.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeEnabled}
	default:
		options.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	}

	client := redis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(pingCtx).Err())
	require.NoError(t, client.FlushAll(ctx).Err())

	cache := rules.NewRedisCache(client, "rules:all", 0)
	eventBus := rules.NewRedisEventBus(client, cfg.RedisChannel)
	return client, cache, eventBus
}

func doProxyRequest(client *http.Client, baseURL, pathPrefix, body string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL+pathPrefix+"/chat", bytes.NewBufferString(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

func upsertRule(t *testing.T, client *http.Client, baseURL, token string, rule rules.Rule) {
	buf, err := json.Marshal(rule)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/admin/rules", bytes.NewReader(buf))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func assertRuleVisible(t *testing.T, client *http.Client, baseURL, token, ruleID string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/admin/rules", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Items []rules.Rule `json:"items"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))

	found := false
	for _, item := range payload.Items {
		if item.ID == ruleID {
			found = true
			require.True(t, item.Enabled)
			break
		}
	}
	require.True(t, found, "rule must exist")
}

func deleteRule(t *testing.T, client *http.Client, baseURL, token, ruleID string) {
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/admin/rules/"+ruleID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func loginAndGetToken(t *testing.T, client *http.Client, baseURL, username, password string) string {
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	buf, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/admin/login", bytes.NewReader(buf))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == http.StatusNotImplemented {
		t.Skip("token login disabled")
	}
	require.Equalf(t, http.StatusOK, resp.StatusCode, "login failed: %s", string(body))
	var result struct {
		AccessToken string `json:"access_token"`
	}
	require.NoErrorf(t, json.Unmarshal(body, &result), "login response invalid: %s", string(body))
	require.NotEmpty(t, result.AccessToken)
	return result.AccessToken
}

func getEnvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadEnvFile(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Logf("warning: unable to determine cwd: %v", err)
		return
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			if err := godotenv.Overload(candidate); err != nil {
				t.Logf("warning: failed to load %s: %v", candidate, err)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

func mustParseURL(raw string) *url.URL {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

type upstreamServer struct {
	listener net.Listener
	server   *http.Server
	calls    chan upstreamCall
}

type upstreamCall struct {
	Path    string
	Method  string
	Headers http.Header
	Body    requestPayload
}

type requestPayload struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Metadata map[string]any `json:"metadata"`
}

func startUpstream(t *testing.T) *upstreamServer {
	t.Helper()
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)

	calls := make(chan upstreamCall, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload requestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case calls <- upstreamCall{
			Path:    r.URL.Path,
			Method:  r.Method,
			Headers: r.Header.Clone(),
			Body:    payload,
		}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"url": fmt.Sprintf("http://%s%s", r.Host, r.URL.Path),
		})
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()

	return &upstreamServer{
		listener: ln,
		server:   srv,
		calls:    calls,
	}
}

func (u *upstreamServer) TargetURL() string {
	if u == nil || u.listener == nil {
		return ""
	}
	addr := u.listener.Addr().(*net.TCPAddr)
	return fmt.Sprintf("http://%s:%d", resolveHostForClients(addr.IP), addr.Port)
}

func (u *upstreamServer) WaitForCall(t *testing.T) upstreamCall {
	t.Helper()
	select {
	case call := <-u.calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for upstream request")
		return upstreamCall{}
	}
}

func (u *upstreamServer) Close() {
	if u == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = u.server.Shutdown(ctx)
}

func resolveHostForClients(ip net.IP) string {
	if ip.IsUnspecified() || ip.IsLoopback() {
		return "127.0.0.1"
	}
	return ip.String()
}
