package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"github.com/prehisle/yapi/pkg/accounts"
)

const (
	userContextKey      = "auth_user"
	apiKeyContextKey    = "auth_api_key"
	bindingContextKey   = "auth_binding"
	upstreamContextKey  = "auth_upstream"
	rawAPIKeyContextKey = "auth_raw_api_key"
)

// UpstreamInfo carries upstream credential and endpoints.
type UpstreamInfo struct {
	Credential accounts.UpstreamCredential
	Endpoints  []string
}

// Authenticator resolves API keys and associated upstream bindings.
type Authenticator interface {
	ResolveBindingByRawKey(ctx context.Context, rawKey string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error)
	ResolveAPIKey(ctx context.Context, rawKey string) (accounts.APIKey, error)
	GetUser(ctx context.Context, id string) (accounts.User, error)
}

// APIKeyAuth verifies client API key and loads associated binding.
func APIKeyAuth(auth Authenticator) gin.HandlerFunc {
	if auth == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		rawKey := extractAPIKey(c.Request)
		if rawKey == "" {
			c.Next()
			return
		}
		binding, upstream, err := auth.ResolveBindingByRawKey(c.Request.Context(), rawKey)
		if err != nil {
			apiKey, apiErr := auth.ResolveAPIKey(c.Request.Context(), rawKey)
			if apiErr != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
				return
			}
			c.Set(apiKeyContextKey, apiKey)
			c.Set(rawAPIKeyContextKey, rawKey)
			if user, userErr := auth.GetUser(c.Request.Context(), apiKey.UserID); userErr == nil {
				c.Set(userContextKey, user)
			}
			c.Next()
			return
		}
		if apiKey, apiErr := auth.ResolveAPIKey(c.Request.Context(), rawKey); apiErr == nil {
			c.Set(apiKeyContextKey, apiKey)
		}
		c.Set(bindingContextKey, binding)
		c.Set(upstreamContextKey, UpstreamInfo{
			Credential: upstream,
			Endpoints:  decodeEndpoints(upstream.Endpoints),
		})
		c.Set(rawAPIKeyContextKey, rawKey)
		if user, err := auth.GetUser(c.Request.Context(), binding.UserID); err == nil {
			c.Set(userContextKey, user)
		}
		c.Next()
	}
}

func extractAPIKey(req *http.Request) string {
	value := req.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		candidate := strings.TrimSpace(value[7:])
		if strings.HasPrefix(candidate, "yapi_") {
			return candidate
		}
	}
	if key := req.Header.Get("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}
	if key := req.Header.Get("X-User-Api-Key"); key != "" {
		return strings.TrimSpace(key)
	}
	return ""
}

// CurrentUser returns authenticated user info if available.
func CurrentUser(c *gin.Context) (accounts.User, bool) {
	if value, ok := c.Get(userContextKey); ok {
		if user, ok := value.(accounts.User); ok {
			return user, true
		}
	}
	return accounts.User{}, false
}

// CurrentAPIKey returns API key resolved from request.
func CurrentAPIKey(c *gin.Context) (accounts.APIKey, bool) {
	if value, ok := c.Get(apiKeyContextKey); ok {
		if key, ok := value.(accounts.APIKey); ok {
			return key, true
		}
	}
	return accounts.APIKey{}, false
}

// CurrentBinding returns API key binding if available.
func CurrentBinding(c *gin.Context) (accounts.UserAPIKeyBinding, bool) {
	if value, ok := c.Get(bindingContextKey); ok {
		if binding, ok := value.(accounts.UserAPIKeyBinding); ok {
			return binding, true
		}
	}
	return accounts.UserAPIKeyBinding{}, false
}

// CurrentUpstreamCredential returns upstream credential associated with request.
func CurrentUpstreamInfo(c *gin.Context) (UpstreamInfo, bool) {
	if value, ok := c.Get(upstreamContextKey); ok {
		if info, ok := value.(UpstreamInfo); ok {
			return info, true
		}
	}
	return UpstreamInfo{}, false
}

// RawAPIKey returns the raw API key string used for the request.
func RawAPIKey(c *gin.Context) (string, bool) {
	value, ok := c.Get(rawAPIKeyContextKey)
	if !ok {
		return "", false
	}
	if raw, ok := value.(string); ok && raw != "" {
		return raw, true
	}
	return "", false
}

func decodeEndpoints(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return nil
	}
	var endpoints []string
	if err := json.Unmarshal(raw, &endpoints); err != nil {
		return nil
	}
	return endpoints
}
