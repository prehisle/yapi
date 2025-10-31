package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prehisle/yapi/pkg/rules"
)

func TestParseJSONPath(t *testing.T) {
	tokens, err := rules.ParseJSONPath("metadata.trace_id")
	require.NoError(t, err)
	require.Len(t, tokens, 2)
	require.True(t, tokens[0].IsKey())
	require.Equal(t, "metadata", tokens[0].Key)
	require.True(t, tokens[1].IsKey())
	require.Equal(t, "trace_id", tokens[1].Key)

	tokens, err = rules.ParseJSONPath("messages[0].role")
	require.NoError(t, err)
	require.Len(t, tokens, 3)
	require.True(t, tokens[0].IsKey())
	require.True(t, tokens[1].IsIndex())
	require.Equal(t, 0, tokens[1].IndexValue())
	require.True(t, tokens[2].IsKey())

	tokens, err = rules.ParseJSONPath("choices.1.delta")
	require.NoError(t, err)
	require.True(t, tokens[1].IsIndex())
	require.Equal(t, 1, tokens[1].IndexValue())
}

func TestParseJSONPath_Invalid(t *testing.T) {
	_, err := rules.ParseJSONPath("")
	require.Error(t, err)
	_, err = rules.ParseJSONPath("messages[]")
	require.Error(t, err)
	_, err = rules.ParseJSONPath(".metadata")
	require.Error(t, err)
}
