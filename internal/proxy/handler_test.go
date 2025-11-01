package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/prehisle/yapi/internal/middleware"
	"github.com/prehisle/yapi/pkg/accounts"
	"github.com/prehisle/yapi/pkg/rules"
)

func TestApplyRuleActions_ModifyJSONAndHeaders(t *testing.T) {
	body := `{"model":"gpt-4","unused":"legacy","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, "http://localhost/v1/chat", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}

	actions := rules.Actions{
		SetHeaders:       map[string]string{"X-Trace-ID": "abc123"},
		SetAuthorization: "Bearer xyz",
		OverrideJSON: map[string]any{
			"model":                 "gpt-4.1",
			"metadata.trace_id":     "abc123",
			"messages[0].role":      "system",
			"messages[1].role":      "assistant",
			"messages[1].content":   "hello",
			"messages[1].recipient": "user",
		},
		RemoveJSON: []string{"unused", "messages[0].content"},
	}

	h := &Handler{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	err = h.applyRuleActions(ctx, req, rules.Rule{ID: "rule-a", Actions: actions})
	require.NoError(t, err)

	require.Equal(t, "Bearer xyz", req.Header.Get("Authorization"))
	require.Equal(t, "abc123", req.Header.Get("X-Trace-ID"))
	require.Empty(t, req.Header.Values("X-YAPI-Body-Rewrite-Error"))

	modifiedBody, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	// 重置 Body 以供后续读取。
	req.Body = io.NopCloser(bytes.NewReader(modifiedBody))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(modifiedBody, &payload))
	require.Equal(t, "gpt-4.1", payload["model"])
	_, exists := payload["unused"]
	require.False(t, exists)

	metadata, ok := payload["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "abc123", metadata["trace_id"])

	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	first, ok := messages[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "system", first["role"])
	_, hasContent := first["content"]
	require.False(t, hasContent)

	second, ok := messages[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "assistant", second["role"])
	require.Equal(t, "hello", second["content"])
	require.Equal(t, "user", second["recipient"])
}

func TestApplyRuleActions_NonJSONBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost/v1/chat", strings.NewReader("plain text"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	actions := rules.Actions{
		OverrideJSON: map[string]any{"model": "gpt-4"},
	}

	h := &Handler{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	err = h.applyRuleActions(ctx, req, rules.Rule{ID: "rule-non-json", Actions: actions})
	require.Error(t, err)
}

func TestHandler_StreamPassthrough(t *testing.T) {
	chunks := []string{"data: first\n\n", "data: second\n\n", "data: [DONE]\n\n"}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		for _, chunk := range chunks {
			_, err := io.WriteString(w, chunk)
			require.NoError(t, err)
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	store := rules.NewMemoryStore()
	svc := rules.NewService(store)
	rule := rules.Rule{
		ID:       "stream-rule",
		Priority: 100,
		Enabled:  true,
		Matcher: rules.Matcher{
			PathPrefix: "/v1",
		},
		Actions: rules.Actions{
			SetTargetURL: upstream.URL,
		},
	}
	require.NoError(t, svc.UpsertRule(context.Background(), rule))

	h := NewHandler(svc)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	RegisterRoutes(router, h)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/chat")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, strings.Join(chunks, ""), string(body))
}

func TestHandler_MatchRule_WithAccountMatchers(t *testing.T) {
	accountRule := rules.Rule{
		ID:       "account-specific",
		Priority: 100,
		Enabled:  true,
		Matcher: rules.Matcher{
			PathPrefix:         "/v1",
			RequireBinding:     true,
			APIKeyPrefixes:     []string{"abcd1234"},
			UserIDs:            []string{"user-1"},
			UserMetadata:       map[string]string{"tier": "gold"},
			BindingUpstreamIDs: []string{"cred-1"},
			BindingProviders:   []string{"openai"},
		},
		Actions: rules.Actions{
			SetTargetURL: "https://upstream.example.com",
		},
	}
	fallbackRule := rules.Rule{
		ID:       "fallback",
		Priority: 10,
		Enabled:  true,
		Matcher: rules.Matcher{
			PathPrefix: "/v1",
		},
		Actions: rules.Actions{
			SetTargetURL: "https://fallback.example.com",
		},
	}
	svc := &ruleServiceStub{
		rules: []rules.Rule{accountRule, fallbackRule},
	}
	h := NewHandler(svc)

	t.Run("matches account aware rule when context satisfies", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		req, err := http.NewRequest(http.MethodGet, "/v1/chat", nil)
		require.NoError(t, err)
		ctx.Request = req
		ctx.Set("auth_api_key", accounts.APIKey{ID: "key-1", UserID: "user-1", Prefix: "abcd1234"})
		ctx.Set("auth_raw_api_key", "yapi_abcd1234_secretvalue")
		ctx.Set("auth_user", accounts.User{ID: "user-1", Metadata: datatypes.JSONMap{"tier": "gold"}})
		ctx.Set("auth_binding", accounts.UserAPIKeyBinding{
			ID:                   "binding-1",
			UserID:               "user-1",
			UserAPIKeyID:         "key-1",
			UpstreamCredentialID: "cred-1",
		})
		ctx.Set("auth_upstream", middleware.UpstreamInfo{
			Credential: accounts.UpstreamCredential{
				ID:       "cred-1",
				UserID:   "user-1",
				Provider: "openai",
			},
			Endpoints: []string{"https://upstream.example.com"},
		})

		rule, err := h.matchRule(ctx)
		require.NoError(t, err)
		require.Equal(t, "account-specific", rule.ID)
	})

	t.Run("falls back when account context missing", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		req, err := http.NewRequest(http.MethodGet, "/v1/chat", nil)
		require.NoError(t, err)
		ctx.Request = req

		rule, err := h.matchRule(ctx)
		require.NoError(t, err)
		require.Equal(t, "fallback", rule.ID)
	})
}

type ruleServiceStub struct {
	rules []rules.Rule
}

func (s *ruleServiceStub) ListRules(ctx context.Context) ([]rules.Rule, error) {
	return s.rules, nil
}

func (s *ruleServiceStub) GetRule(ctx context.Context, id string) (rules.Rule, error) {
	return rules.Rule{}, nil
}

func (s *ruleServiceStub) UpsertRule(ctx context.Context, rule rules.Rule) error {
	return nil
}

func (s *ruleServiceStub) DeleteRule(ctx context.Context, id string) error {
	return nil
}

func (s *ruleServiceStub) StartBackgroundSync(ctx context.Context) {}
