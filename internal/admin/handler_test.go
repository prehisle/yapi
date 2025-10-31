package admin

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

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

func TestHandler_ListRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expected := []rules.Rule{{ID: "rule-1"}}
	svc := &serviceStub{
		listFn: func(ctx context.Context) ([]rules.Rule, error) {
			return expected, nil
		},
	}
	router := gin.New()
	RegisterRoutes(router.Group("/admin"), NewHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/admin/rules", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"rule-1"`)
}

func TestHandler_CreateRule_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/admin"), NewHandler(&serviceStub{}))

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
	RegisterRoutes(router.Group("/admin"), NewHandler(svc))

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
	RegisterRoutes(router.Group("/admin"), NewHandler(svc))

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
	RegisterRoutes(router.Group("/admin"), NewHandler(svc))

	req := httptest.NewRequest(http.MethodDelete, "/admin/rules/rule-x", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
