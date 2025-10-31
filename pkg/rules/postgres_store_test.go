package rules_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/prehisle/yapi/pkg/rules"
)

func TestDBStore_CRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	store := rules.NewDBStore(db)
	ctx := context.Background()
	require.NoError(t, store.AutoMigrate(ctx))

	ruleHigh := rules.Rule{
		ID:       "high",
		Priority: 20,
		Matcher: rules.Matcher{
			PathPrefix: "/v1",
			Methods:    []string{"POST"},
		},
		Actions: rules.Actions{
			SetTargetURL: "https://example.com/high",
		},
		Enabled: true,
	}
	ruleLow := ruleHigh
	ruleLow.ID = "low"
	ruleLow.Priority = 5
	ruleLow.Actions.SetTargetURL = "https://example.com/low"

	require.NoError(t, store.Save(ctx, ruleHigh))
	require.NoError(t, store.Save(ctx, ruleLow))

	list, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "high", list[0].ID)

	got, err := store.Get(ctx, "low")
	require.NoError(t, err)
	require.Equal(t, ruleLow.Actions.SetTargetURL, got.Actions.SetTargetURL)

	ruleLow.Actions.SetHeaders = map[string]string{"Authorization": "Bearer token"}
	require.NoError(t, store.Save(ctx, ruleLow))

	gotUpdated, err := store.Get(ctx, "low")
	require.NoError(t, err)
	require.Contains(t, gotUpdated.Actions.SetHeaders, "Authorization")

	require.NoError(t, store.Delete(ctx, "high"))
	_, err = store.Get(ctx, "high")
	require.ErrorIs(t, err, rules.ErrRuleNotFound)
}
