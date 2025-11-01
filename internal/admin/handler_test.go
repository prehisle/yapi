package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/prehisle/yapi/pkg/accounts"
	"github.com/prehisle/yapi/pkg/rules"
)

type serviceStub struct {
	listFn           func(ctx context.Context) ([]rules.Rule, error)
	upsertFn         func(ctx context.Context, rule rules.Rule) error
	deleteFn         func(ctx context.Context, id string) error
	createUserFn     func(ctx context.Context, params accounts.CreateUserParams) (accounts.User, error)
	listUsersFn      func(ctx context.Context) ([]accounts.User, error)
	deleteUserFn     func(ctx context.Context, id string) error
	createAPIKeyFn   func(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error)
	listAPIKeysFn    func(ctx context.Context, userID string) ([]accounts.APIKey, error)
	revokeAPIKeyFn   func(ctx context.Context, apiKeyID string) error
	createUpstreamFn func(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error)
	listUpstreamFn   func(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error)
	deleteUpstreamFn func(ctx context.Context, credentialID string) error
	bindAPIKeyFn     func(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error)
	getBindingFn     func(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error)
}

func (s *serviceStub) ListRules(ctx context.Context) ([]rules.Rule, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s *serviceStub) GetRule(ctx context.Context, id string) (rules.Rule, error) {
	return rules.Rule{}, nil
}

func (s *serviceStub) CreateOrUpdateRule(ctx context.Context, rule rules.Rule) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, rule)
	}
	return nil
}

func (s *serviceStub) DeleteRule(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *serviceStub) CreateUser(ctx context.Context, params accounts.CreateUserParams) (accounts.User, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, params)
	}
	return accounts.User{}, ErrAccountsUnavailable
}

func (s *serviceStub) ListUsers(ctx context.Context) ([]accounts.User, error) {
	if s.listUsersFn != nil {
		return s.listUsersFn(ctx)
	}
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) DeleteUser(ctx context.Context, id string) error {
	if s.deleteUserFn != nil {
		return s.deleteUserFn(ctx, id)
	}
	return ErrAccountsUnavailable
}

func (s *serviceStub) CreateUserAPIKey(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error) {
	if s.createAPIKeyFn != nil {
		return s.createAPIKeyFn(ctx, params)
	}
	return accounts.APIKey{}, "", ErrAccountsUnavailable
}

func (s *serviceStub) ListUserAPIKeys(ctx context.Context, userID string) ([]accounts.APIKey, error) {
	if s.listAPIKeysFn != nil {
		return s.listAPIKeysFn(ctx, userID)
	}
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) RevokeUserAPIKey(ctx context.Context, apiKeyID string) error {
	if s.revokeAPIKeyFn != nil {
		return s.revokeAPIKeyFn(ctx, apiKeyID)
	}
	return ErrAccountsUnavailable
}

func (s *serviceStub) CreateUpstreamCredential(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error) {
	if s.createUpstreamFn != nil {
		return s.createUpstreamFn(ctx, params)
	}
	return accounts.UpstreamCredential{}, ErrAccountsUnavailable
}

func (s *serviceStub) ListUpstreamCredentials(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error) {
	if s.listUpstreamFn != nil {
		return s.listUpstreamFn(ctx, userID)
	}
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) DeleteUpstreamCredential(ctx context.Context, credentialID string) error {
	if s.deleteUpstreamFn != nil {
		return s.deleteUpstreamFn(ctx, credentialID)
	}
	return ErrAccountsUnavailable
}

func (s *serviceStub) BindAPIKey(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error) {
	if s.bindAPIKeyFn != nil {
		return s.bindAPIKeyFn(ctx, params)
	}
	return accounts.UserAPIKeyBinding{}, ErrAccountsUnavailable
}

func (s *serviceStub) GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error) {
	if s.getBindingFn != nil {
		return s.getBindingFn(ctx, apiKeyID)
	}
	return accounts.UserAPIKeyBinding{}, accounts.UpstreamCredential{}, ErrAccountsUnavailable
}

func TestHandler_ListRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expected := []rules.Rule{{ID: "rule-1"}}
	svc := &serviceStub{
		listFn: func(ctx context.Context) ([]rules.Rule, error) {
			return expected, nil
		},
	}
	router := gin.New()
	handler := NewHandler(svc, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/rules", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp listRulesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "rule-1", resp.Items[0].ID)
	require.Equal(t, 0, resp.EnabledTotal)
}

func TestHandler_ListRules_PaginationAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rule := func(id string, enabled bool, target string) rules.Rule {
		return rules.Rule{
			ID:      id,
			Enabled: enabled,
			Matcher: rules.Matcher{PathPrefix: "/" + id},
			Actions: rules.Actions{SetTargetURL: target},
		}
	}
	svc := &serviceStub{
		listFn: func(ctx context.Context) ([]rules.Rule, error) {
			return []rules.Rule{
				rule("rule-1", true, "https://foo"),
				rule("rule-2", false, "https://bar"),
				rule("rule-3", true, "https://baz"),
				rule("rule-4", true, "https://qux"),
				rule("rule-5", false, "https://foo"),
			}, nil
		},
	}
	router := gin.New()
	handler := NewHandler(svc, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/rules?page=1&page_size=2&enabled=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp listRulesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.PageSize)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 3, resp.Total)
	require.Len(t, resp.Items, 2)
	require.Equal(t, 3, resp.EnabledTotal)

	// keyword filtering narrows down results
	req = httptest.NewRequest(http.MethodGet, "/admin/rules?page=1&page_size=50&q=foo", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Total)
	require.Len(t, resp.Items, 2)
	require.Equal(t, 1, resp.EnabledTotal)

	// requesting a page beyond the last should clamp to last page
	req = httptest.NewRequest(http.MethodGet, "/admin/rules?page=10&page_size=2", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.PageSize)
	require.Equal(t, 3, resp.Page) // ceil(5/2)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "rule-5", resp.Items[0].ID)
	require.Equal(t, 3, resp.EnabledTotal)
}

func TestHandler_CreateRule_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(&serviceStub{}, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	req := httptest.NewRequest(http.MethodPost, "/admin/rules", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateRule_InvalidRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &serviceStub{
		upsertFn: func(ctx context.Context, rule rules.Rule) error {
			return rules.ErrInvalidRule
		},
	}
	router := gin.New()
	handler := NewHandler(svc, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	body := `{"id":"rule-1"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeleteRule_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &serviceStub{
		deleteFn: func(ctx context.Context, id string) error {
			return rules.ErrRuleNotFound
		},
	}
	router := gin.New()
	handler := NewHandler(svc, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	req := httptest.NewRequest(http.MethodDelete, "/admin/rules/rule-x", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_DeleteRule_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &serviceStub{
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("db error")
		},
	}
	router := gin.New()
	handler := NewHandler(svc, nil)
	RegisterProtectedRoutes(router.Group("/admin"), handler)

	req := httptest.NewRequest(http.MethodDelete, "/admin/rules/rule-x", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_WithAuth_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &serviceStub{}
	auth := NewAuthenticator("admin", "secret", "sign-key", time.Minute)
	router := gin.New()
	group := router.Group("/admin")
	group.Use(auth.Middleware())
	RegisterProtectedRoutes(group, NewHandler(svc, auth))

	req := httptest.NewRequest(http.MethodGet, "/admin/rules", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_WithAuth_Bearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &serviceStub{}
	auth := NewAuthenticator("admin", "secret", "sign-key", time.Minute)
	token, err := auth.IssueToken("admin", "secret")
	require.NoError(t, err)
	router := gin.New()
	group := router.Group("/admin")
	group.Use(auth.Middleware())
	RegisterProtectedRoutes(group, NewHandler(svc, auth))

	req := httptest.NewRequest(http.MethodGet, "/admin/rules", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Login_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthenticator("admin", "secret", "sign-key", time.Minute)
	handler := NewHandler(&serviceStub{}, auth)
	router := gin.New()
	RegisterPublicRoutes(router.Group("/admin"), handler)

	body := bytes.NewBufferString(`{"username":"admin","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "access_token")
}

func TestHandler_Login_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&serviceStub{}, nil)
	router := gin.New()
	RegisterPublicRoutes(router.Group("/admin"), handler)

	body := bytes.NewBufferString(`{"username":"admin","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestHandler_CreateUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 10, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		createUserFn: func(ctx context.Context, params accounts.CreateUserParams) (accounts.User, error) {
			require.Equal(t, "new-user", params.Name)
			require.Equal(t, "example", params.Description)
			require.Equal(t, "gold", params.Metadata["tier"])
			return accounts.User{
				ID:          "user-1",
				Name:        params.Name,
				Description: params.Description,
				Metadata:    datatypes.JSONMap{"tier": "gold"},
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}
	router := newTestRouter(svc)

	body := bytes.NewBufferString(`{"name":"new-user","description":"example","metadata":{"tier":"gold"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/users", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp userResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "user-1", resp.ID)
	require.Equal(t, "new-user", resp.Name)
	require.Equal(t, "example", resp.Description)
	require.Contains(t, resp.Metadata, "tier")
	require.Equal(t, "gold", resp.Metadata["tier"])
	require.False(t, resp.CreatedAt.IsZero())
	require.False(t, resp.UpdatedAt.IsZero())
}

func TestHandler_CreateUser_InvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&serviceStub{})

	body := bytes.NewBufferString(`{"description":"missing name"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/users", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListUsers_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 11, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		listUsersFn: func(ctx context.Context) ([]accounts.User, error) {
			return []accounts.User{
				{
					ID:        "user-1",
					Name:      "alice",
					Metadata:  datatypes.JSONMap{"tier": "gold"},
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:        "user-2",
					Name:      "bob",
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []userResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	require.Equal(t, "alice", resp.Items[0].Name)
	require.Equal(t, "gold", resp.Items[0].Metadata["tier"])
	require.Equal(t, "bob", resp.Items[1].Name)
}

func TestHandler_CreateUserAPIKey_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 12, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		createAPIKeyFn: func(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error) {
			require.Equal(t, "user-1", params.UserID)
			require.Equal(t, "primary", params.Label)
			return accounts.APIKey{
				ID:        "key-1",
				UserID:    params.UserID,
				Label:     params.Label,
				Prefix:    "abcd1234",
				CreatedAt: now,
				UpdatedAt: now,
			}, "secret-value", nil
		},
	}
	router := newTestRouter(svc)

	body := bytes.NewBufferString(`{"label":"primary"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/user-1/api-keys", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp struct {
		APIKey apiKeyResponse `json:"api_key"`
		Secret string         `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "key-1", resp.APIKey.ID)
	require.Equal(t, "abcd1234", resp.APIKey.Prefix)
	require.Equal(t, "secret-value", resp.Secret)
}

func TestHandler_ListUserAPIKeys_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 13, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		listAPIKeysFn: func(ctx context.Context, userID string) ([]accounts.APIKey, error) {
			require.Equal(t, "user-1", userID)
			return []accounts.APIKey{
				{
					ID:        "key-1",
					UserID:    userID,
					Label:     "primary",
					Prefix:    "abcd1234",
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:        "key-2",
					UserID:    userID,
					Label:     "backup",
					Prefix:    "wxyz5678",
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/user-1/api-keys", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []apiKeyResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	require.Equal(t, "primary", resp.Items[0].Label)
	require.Equal(t, "backup", resp.Items[1].Label)
}

func TestHandler_DeleteUserAPIKey_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var called bool
	svc := &serviceStub{
		revokeAPIKeyFn: func(ctx context.Context, apiKeyID string) error {
			require.Equal(t, "key-1", apiKeyID)
			called = true
			return nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api-keys/key-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, called, "expected revokeUserAPIKey to be invoked")
}

func TestHandler_CreateUpstreamCredential_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 14, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		createUpstreamFn: func(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error) {
			require.Equal(t, "user-1", params.UserID)
			require.Equal(t, "mock-provider", params.Provider)
			require.Equal(t, "primary", params.Label)
			require.Equal(t, "secret-token", params.Plaintext)
			require.ElementsMatch(t, []string{"https://chat.example.com"}, params.Endpoints)
			require.Equal(t, "prod", params.Metadata["env"])
			return accounts.UpstreamCredential{
				ID:        "cred-1",
				UserID:    params.UserID,
				Provider:  params.Provider,
				Label:     params.Label,
				Endpoints: datatypes.JSON([]byte(`["https://chat.example.com"]`)),
				Metadata:  datatypes.JSONMap{"env": "prod"},
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
	router := newTestRouter(svc)

	body := bytes.NewBufferString(`{
		"provider":"mock-provider",
		"label":"primary",
		"plaintext":"secret-token",
		"endpoints":["https://chat.example.com"],
		"metadata":{"env":"prod"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/user-1/upstreams", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp upstreamCredentialResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "cred-1", resp.ID)
	require.Equal(t, "mock-provider", resp.Provider)
	require.Equal(t, []string{"https://chat.example.com"}, resp.Endpoints)
	require.Equal(t, "prod", resp.Metadata["env"])
}

func TestHandler_ListUpstreamCredentials_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 15, 0, 0, 0, time.UTC)
	svc := &serviceStub{
		listUpstreamFn: func(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error) {
			require.Equal(t, "user-1", userID)
			return []accounts.UpstreamCredential{
				{
					ID:        "cred-1",
					UserID:    userID,
					Provider:  "mock-provider",
					Label:     "primary",
					Endpoints: datatypes.JSON([]byte(`["https://chat.example.com","https://backup.example.com"]`)),
					Metadata:  datatypes.JSONMap{"env": "prod"},
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/user-1/upstreams", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []upstreamCredentialResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	require.ElementsMatch(t, []string{"https://chat.example.com", "https://backup.example.com"}, resp.Items[0].Endpoints)
	require.Equal(t, "prod", resp.Items[0].Metadata["env"])
}

func TestHandler_DeleteUpstreamCredential_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var called bool
	svc := &serviceStub{
		deleteUpstreamFn: func(ctx context.Context, credentialID string) error {
			require.Equal(t, "cred-1", credentialID)
			called = true
			return nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/admin/upstreams/cred-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, called, "expected deleteUpstreamCredential to be invoked")
}

func TestHandler_BindAPIKey_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 16, 0, 0, 0, time.UTC)
	binding := accounts.UserAPIKeyBinding{
		ID:                   "binding-1",
		UserID:               "user-1",
		UserAPIKeyID:         "key-1",
		UpstreamCredentialID: "cred-1",
		Metadata:             datatypes.JSONMap{"strategy": "primary"},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	upstream := accounts.UpstreamCredential{
		ID:        "cred-1",
		UserID:    "user-1",
		Provider:  "mock-provider",
		Label:     "primary",
		Endpoints: datatypes.JSON([]byte(`["https://chat.example.com"]`)),
		Metadata:  datatypes.JSONMap{"env": "prod"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	svc := &serviceStub{
		bindAPIKeyFn: func(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error) {
			require.Equal(t, "user-1", params.UserID)
			require.Equal(t, "key-1", params.UserAPIKeyID)
			require.Equal(t, "cred-1", params.UpstreamCredentialID)
			return binding, nil
		},
		getBindingFn: func(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error) {
			require.Equal(t, "key-1", apiKeyID)
			return binding, upstream, nil
		},
	}
	router := newTestRouter(svc)

	body := bytes.NewBufferString(`{"user_id":"user-1","upstream_credential_id":"cred-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys/key-1/binding", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp apiKeyBindingResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "binding-1", resp.ID)
	require.Equal(t, "cred-1", resp.UpstreamCredentialID)
	require.Equal(t, "mock-provider", resp.Upstream.Provider)
	require.Equal(t, "primary", resp.Upstream.Label)
	require.Equal(t, "primary", resp.Metadata["strategy"])
}

func TestHandler_GetAPIKeyBinding_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2025, time.November, 2, 17, 0, 0, 0, time.UTC)
	binding := accounts.UserAPIKeyBinding{
		ID:                   "binding-1",
		UserID:               "user-1",
		UserAPIKeyID:         "key-1",
		UpstreamCredentialID: "cred-1",
		Metadata:             datatypes.JSONMap{"strategy": "primary"},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	upstream := accounts.UpstreamCredential{
		ID:        "cred-1",
		UserID:    "user-1",
		Provider:  "mock-provider",
		Label:     "primary",
		Endpoints: datatypes.JSON([]byte(`["https://chat.example.com"]`)),
		Metadata:  datatypes.JSONMap{"env": "prod"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	svc := &serviceStub{
		getBindingFn: func(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error) {
			require.Equal(t, "key-1", apiKeyID)
			return binding, upstream, nil
		},
	}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/api-keys/key-1/binding", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp apiKeyBindingResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "binding-1", resp.ID)
	require.Equal(t, []string{"https://chat.example.com"}, resp.Upstream.Endpoints)
	require.Equal(t, "prod", resp.Upstream.Metadata["env"])
}

func newTestRouter(svc Service) *gin.Engine {
	router := gin.New()
	RegisterProtectedRoutes(router.Group("/admin"), NewHandler(svc, nil))
	return router
}
