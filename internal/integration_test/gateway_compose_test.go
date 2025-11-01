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
	"github.com/prehisle/yapi/pkg/accounts"
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

	store, db, dbCleanup := setupStore(t, ctx, cfg)
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

	server := startGateway(t, cfg, logger, ruleService, db)
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

	user := createUser(t, client, baseURL, token, "e2e-user-"+ruleID, "integration user")
	apiKey, clientSecret := createUserAPIKey(t, client, baseURL, token, user.ID, "primary")
	upstreamSecret := "sk-upstream-binding"
	credential := createUpstreamCredential(t, client, baseURL, token, user.ID, "mock-provider", "primary", upstreamSecret, []string{upstream.TargetURL()})
	_ = bindAPIKey(t, client, baseURL, token, apiKey.ID, user.ID, credential.ID)

	accountBody := `{"messages":[{"role":"user","content":"from api key"}]}`
	accountReq, err := http.NewRequest(http.MethodPost, baseURL+pathPrefix+"/chat", bytes.NewBufferString(accountBody))
	require.NoError(t, err)
	accountReq.Header.Set("Content-Type", "application/json")
	accountReq.Header.Set("Authorization", "Bearer "+clientSecret)
	accountResp, err := client.Do(accountReq)
	require.NoError(t, err)
	defer accountResp.Body.Close()
	require.Equal(t, http.StatusOK, accountResp.StatusCode)

	accountCall := upstream.WaitForCall(t)
	require.Equal(t, "/anything/chat", accountCall.Path)
	require.Equal(t, "Bearer "+upstreamSecret, accountCall.Headers.Get("Authorization"))
	require.Equal(t, "mock-provider", accountCall.Headers.Get("X-Upstream-Provider"))
	require.Equal(t, credential.ID, accountCall.Headers.Get("X-Upstream-Credential-ID"))
	require.Equal(t, user.ID, accountCall.Headers.Get("X-YAPI-User-ID"))

	t.Run("binding missing upstream returns not found", func(t *testing.T) {
		key, _ := createUserAPIKey(t, client, baseURL, token, user.ID, "missing-upstream")
		resp := adminJSONRequest(t, client, baseURL, token, http.MethodPost, "/admin/api-keys/"+key.ID+"/binding", map[string]any{
			"user_id":                user.ID,
			"upstream_credential_id": uuid.NewString(),
		})
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		resp.Body.Close()
		require.Equalf(t, http.StatusNotFound, resp.StatusCode, "unexpected response: %s", string(body))
	})

	t.Run("revoked api key rejects proxy calls", func(t *testing.T) {
		key, secret := createUserAPIKey(t, client, baseURL, token, user.ID, "revocation-check")
		_ = bindAPIKey(t, client, baseURL, token, key.ID, user.ID, credential.ID)

		validReqBody := `{"messages":[{"role":"user","content":"before revocation"}]}`
		successReq, err := http.NewRequest(http.MethodPost, baseURL+pathPrefix+"/chat", bytes.NewBufferString(validReqBody))
		require.NoError(t, err)
		successReq.Header.Set("Content-Type", "application/json")
		successReq.Header.Set("Authorization", "Bearer "+secret)
		successResp, err := client.Do(successReq)
		require.NoError(t, err)
		defer successResp.Body.Close()
		require.Equal(t, http.StatusOK, successResp.StatusCode)
		_, _ = io.ReadAll(successResp.Body)

		deleteUserAPIKey(t, client, baseURL, token, key.ID)

		failReq, err := http.NewRequest(http.MethodPost, baseURL+pathPrefix+"/chat", bytes.NewBufferString(validReqBody))
		require.NoError(t, err)
		failReq.Header.Set("Content-Type", "application/json")
		failReq.Header.Set("Authorization", "Bearer "+secret)
		failResp, err := client.Do(failReq)
		require.NoError(t, err)
		defer failResp.Body.Close()
		body, readErr := io.ReadAll(failResp.Body)
		require.NoError(t, readErr)
		require.Equalf(t, http.StatusUnauthorized, failResp.StatusCode, "expected unauthorized, got %d body=%s", failResp.StatusCode, string(body))
	})
}

func startGateway(t *testing.T, cfg config.Config, logger *slog.Logger, ruleService rules.Service, db *gorm.DB) *httptestServer {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID(), middleware.AccessLogger(logger), middleware.CORS(cfg.AdminAllowedOrigins))

	var accountService accounts.Service
	if db != nil {
		accountService = accounts.NewService(db)
		require.NoError(t, accountService.AutoMigrate(context.Background()))
		require.NoError(t, db.WithContext(context.Background()).Unscoped().Where("1 = 1").Delete(&accounts.User{}).Error)
		require.NoError(t, db.WithContext(context.Background()).Unscoped().Where("1 = 1").Delete(&accounts.APIKey{}).Error)
		require.NoError(t, db.WithContext(context.Background()).Unscoped().Where("1 = 1").Delete(&accounts.UpstreamCredential{}).Error)
		require.NoError(t, db.WithContext(context.Background()).Unscoped().Where("1 = 1").Delete(&accounts.UserAPIKeyBinding{}).Error)
	}
	if accountService != nil {
		router.Use(middleware.APIKeyAuth(accountService))
	}
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	adminAuth := admin.NewAuthenticator(cfg.AdminUsername, cfg.AdminPassword, cfg.AdminTokenSecret, cfg.AdminTokenTTL)
	adminService := admin.NewService(ruleService, accountService)
	adminHandler := admin.NewHandler(adminService, adminAuth, admin.WithLogger(logger))

	adminGroup := router.Group("/admin")
	admin.RegisterPublicRoutes(adminGroup, adminHandler)
	protected := adminGroup.Group("")
	protected.Use(adminAuth.Middleware())
	admin.RegisterProtectedRoutes(protected, adminHandler)

	defaultTarget := mustParseURL(cfg.UpstreamBaseURL)
	proxyOpts := []proxy.Option{proxy.WithLogger(logger), proxy.WithDefaultTarget(defaultTarget)}
	if accountService != nil {
		proxyOpts = append(proxyOpts, proxy.WithAccountsService(accountService))
	}
	proxyHandler := proxy.NewHandler(ruleService, proxyOpts...)
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

func setupStore(t *testing.T, ctx context.Context, cfg config.Config) (rules.Store, *gorm.DB, func()) {
	t.Helper()
	if cfg.DatabaseDSN == "" {
		return rules.NewMemoryStore(), nil, func() {}
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
	return store, db, cleanup
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

func adminJSONRequest(t *testing.T, client *http.Client, baseURL, token, method, path string, body any) *http.Response {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func createUser(t *testing.T, client *http.Client, baseURL, token, name, description string) userDTO {
	resp := adminJSONRequest(t, client, baseURL, token, http.MethodPost, "/admin/users", map[string]any{
		"name":        name,
		"description": description,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var user userDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&user))
	return user
}

func createUserAPIKey(t *testing.T, client *http.Client, baseURL, token, userID, label string) (apiKeyDTO, string) {
	resp := adminJSONRequest(t, client, baseURL, token, http.MethodPost, "/admin/users/"+userID+"/api-keys", map[string]any{
		"label": label,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var result apiKeyCreateResponseDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result.APIKey, result.Secret
}

func createUpstreamCredential(t *testing.T, client *http.Client, baseURL, token, userID, provider, label, apiKey string, endpoints []string) upstreamCredentialDTO {
	resp := adminJSONRequest(t, client, baseURL, token, http.MethodPost, "/admin/users/"+userID+"/upstreams", map[string]any{
		"provider":  provider,
		"label":     label,
		"plaintext": apiKey,
		"endpoints": endpoints,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var result upstreamCredentialDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func bindAPIKey(t *testing.T, client *http.Client, baseURL, token, apiKeyID, userID, credentialID string) apiKeyBindingDTO {
	resp := adminJSONRequest(t, client, baseURL, token, http.MethodPost, "/admin/api-keys/"+apiKeyID+"/binding", map[string]any{
		"user_id":                userID,
		"upstream_credential_id": credentialID,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result apiKeyBindingDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func deleteUserAPIKey(t *testing.T, client *http.Client, baseURL, token, apiKeyID string) {
	resp := adminJSONRequest(t, client, baseURL, token, http.MethodDelete, "/admin/api-keys/"+apiKeyID, nil)
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

type userDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type userListResponseDTO struct {
	Items []userDTO `json:"items"`
}

type apiKeyDTO struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Label      string     `json:"label"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type apiKeyCreateResponseDTO struct {
	APIKey apiKeyDTO `json:"api_key"`
	Secret string    `json:"secret"`
}

type upstreamCredentialDTO struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Provider  string         `json:"provider"`
	Label     string         `json:"label"`
	Endpoints []string       `json:"endpoints"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type apiKeyBindingDTO struct {
	ID                   string                `json:"id"`
	UserID               string                `json:"user_id"`
	UserAPIKeyID         string                `json:"user_api_key_id"`
	UpstreamCredentialID string                `json:"upstream_credential_id"`
	Metadata             map[string]any        `json:"metadata"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	Upstream             upstreamCredentialDTO `json:"upstream"`
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
