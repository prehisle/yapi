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

	"github.com/prehisle/yapi/pkg/accounts"
	"github.com/prehisle/yapi/pkg/rules"
)

type serviceStub struct {
	listFn   func(ctx context.Context) ([]rules.Rule, error)
	upsertFn func(ctx context.Context, rule rules.Rule) error
	deleteFn func(ctx context.Context, id string) error
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
	return accounts.User{}, ErrAccountsUnavailable
}

func (s *serviceStub) ListUsers(ctx context.Context) ([]accounts.User, error) {
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) DeleteUser(ctx context.Context, id string) error {
	return ErrAccountsUnavailable
}

func (s *serviceStub) CreateUserAPIKey(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error) {
	return accounts.APIKey{}, "", ErrAccountsUnavailable
}

func (s *serviceStub) ListUserAPIKeys(ctx context.Context, userID string) ([]accounts.APIKey, error) {
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) RevokeUserAPIKey(ctx context.Context, apiKeyID string) error {
	return ErrAccountsUnavailable
}

func (s *serviceStub) CreateUpstreamCredential(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error) {
	return accounts.UpstreamCredential{}, ErrAccountsUnavailable
}

func (s *serviceStub) ListUpstreamCredentials(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error) {
	return nil, ErrAccountsUnavailable
}

func (s *serviceStub) DeleteUpstreamCredential(ctx context.Context, credentialID string) error {
	return ErrAccountsUnavailable
}

func (s *serviceStub) BindAPIKey(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error) {
	return accounts.UserAPIKeyBinding{}, ErrAccountsUnavailable
}

func (s *serviceStub) GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error) {
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
