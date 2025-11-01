package accounts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestService(t *testing.T) Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	svc := NewService(db)
	require.NoError(t, svc.AutoMigrate(context.Background()))
	return svc
}

func TestService_UserAPIKeyLifecycle(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService(t)

	user, err := svc.CreateUser(ctx, CreateUserParams{Name: "alice"})
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)

	key, plain, err := svc.CreateUserAPIKey(ctx, CreateAPIKeyParams{UserID: user.ID, Label: "default"})
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	require.Len(t, key.Prefix, apiKeyPrefixLength)

	keys, err := svc.ListUserAPIKeys(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	cred, err := svc.CreateUpstreamCredential(ctx, CreateUpstreamCredentialParams{
		UserID:    user.ID,
		Provider:  "openai",
		Label:     "primary",
		Plaintext: "sk-test",
		Endpoints: []string{"https://api.openai.com/v1"},
	})
	require.NoError(t, err)
	require.Equal(t, "openai", cred.Provider)

	creds, err := svc.ListUpstreamCredentials(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, creds, 1)

	binding, err := svc.BindAPIKey(ctx, BindAPIKeyParams{
		UserID:               user.ID,
		UserAPIKeyID:         key.ID,
		UpstreamCredentialID: cred.ID,
	})
	require.NoError(t, err)
	require.Equal(t, key.ID, binding.UserAPIKeyID)

	resolvedKey, err := svc.ResolveAPIKey(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, key.ID, resolvedKey.ID)

	resolvedBinding, resolvedCred, err := svc.ResolveBindingByRawKey(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, binding.ID, resolvedBinding.ID)
	require.Equal(t, cred.ID, resolvedCred.ID)

	require.NoError(t, svc.RevokeUserAPIKey(ctx, key.ID))

	_, err = svc.ResolveAPIKey(ctx, plain)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestService_BindOwnershipValidation(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService(t)

	user1, err := svc.CreateUser(ctx, CreateUserParams{Name: "user1"})
	require.NoError(t, err)
	user2, err := svc.CreateUser(ctx, CreateUserParams{Name: "user2"})
	require.NoError(t, err)

	key1, _, err := svc.CreateUserAPIKey(ctx, CreateAPIKeyParams{UserID: user1.ID})
	require.NoError(t, err)
	cred2, err := svc.CreateUpstreamCredential(ctx, CreateUpstreamCredentialParams{
		UserID:    user2.ID,
		Provider:  "anthropic",
		Label:     "other",
		Plaintext: "sk-xxx",
	})
	require.NoError(t, err)

	_, err = svc.BindAPIKey(ctx, BindAPIKeyParams{
		UserID:               user1.ID,
		UserAPIKeyID:         key1.ID,
		UpstreamCredentialID: cred2.ID,
	})
	require.ErrorIs(t, err, ErrConflict)
}
