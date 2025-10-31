package admin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthenticator_IssueAndValidate(t *testing.T) {
	auth := NewAuthenticator("admin", "secret", "sign-key", time.Minute)
	token, err := auth.IssueToken("admin", "secret")
	require.NoError(t, err)
	require.NoError(t, auth.ValidateToken(token))
}

func TestAuthenticator_IssueToken_Invalid(t *testing.T) {
	auth := NewAuthenticator("admin", "secret", "sign-key", time.Minute)
	_, err := auth.IssueToken("admin", "wrong")
	require.ErrorIs(t, err, ErrInvalidCredential)
}

func TestAuthenticator_TokenDisabled(t *testing.T) {
	auth := NewAuthenticator("admin", "secret", "", time.Minute)
	_, err := auth.IssueToken("admin", "secret")
	require.ErrorIs(t, err, ErrTokenNotConfigured)
	require.Error(t, auth.ValidateToken(""))
}
