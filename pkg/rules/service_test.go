package rules_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prehisle/yapi/pkg/rules"
)

func TestService_UpsertAndList(t *testing.T) {
	store := rules.NewMemoryStore()
	svc := rules.NewService(store)

	ruleA := rules.Rule{
		ID:       "rule-a",
		Priority: 10,
		Matcher: rules.Matcher{
			PathPrefix: "/v1",
		},
		Actions: rules.Actions{
			SetTargetURL: "https://example.com/v1",
		},
		Enabled: true,
	}
	ruleB := ruleA
	ruleB.ID = "rule-b"
	ruleB.Priority = 5

	ctx := context.Background()
	require.NoError(t, svc.UpsertRule(ctx, ruleA))
	require.NoError(t, svc.UpsertRule(ctx, ruleB))

	rulesList, err := svc.ListRules(ctx)
	require.NoError(t, err)
	require.Len(t, rulesList, 2)
	require.Equal(t, "rule-a", rulesList[0].ID)

	got, err := svc.GetRule(ctx, "rule-b")
	require.NoError(t, err)
	require.Equal(t, "rule-b", got.ID)

	require.NoError(t, svc.DeleteRule(ctx, "rule-b"))

	_, err = svc.GetRule(ctx, "rule-b")
	require.ErrorIs(t, err, rules.ErrRuleNotFound)
}
