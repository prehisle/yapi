package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prehisle/yapi/pkg/rules"
)

func TestMatcherValidation_WithAccountFields(t *testing.T) {
	rule := rules.Rule{
		ID:       "ctx-rule",
		Priority: 1,
		Enabled:  true,
		Matcher: rules.Matcher{
			PathPrefix:         "/v1",
			APIKeyPrefixes:     []string{"abcd1234"},
			APIKeyIDs:          []string{"key-1"},
			UserIDs:            []string{"user-1"},
			UserMetadata:       map[string]string{"tier": "gold"},
			BindingUpstreamIDs: []string{"cred-1"},
			BindingProviders:   []string{"openai"},
		},
		Actions: rules.Actions{
			SetTargetURL: "https://example.com",
		},
	}
	require.NoError(t, rule.Validate())
}

func TestMatcherValidation_InvalidPrefix(t *testing.T) {
	rule := rules.Rule{
		ID:       "invalid-prefix",
		Priority: 1,
		Enabled:  true,
		Matcher: rules.Matcher{
			PathPrefix:     "/v1",
			APIKeyPrefixes: []string{"short"},
		},
		Actions: rules.Actions{
			SetTargetURL: "https://example.com",
		},
	}
	err := rule.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "api_key_prefixes")
}
