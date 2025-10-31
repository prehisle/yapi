package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
			"model":             "gpt-4.1",
			"metadata.trace_id": "abc123",
		},
		RemoveJSON: []string{"unused"},
	}

	applyRuleActions(req, actions)

	require.Equal(t, "Bearer xyz", req.Header.Get("Authorization"))
	require.Equal(t, "abc123", req.Header.Get("X-Trace-ID"))

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

}
