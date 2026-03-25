package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterCryptoProfileDetectorDetect_Match(t *testing.T) {
	t.Helper()

	var capturedAuth string
	var capturedReferer string
	var capturedTitle string
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedReferer = r.Header.Get("HTTP-Referer")
		capturedTitle = r.Header.Get("X-Title")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "{\"match\":true,\"profile\":\"dex-routing\"}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:        true,
				Endpoint:       server.URL,
				OpenRouterAPIKey: "or-key-test",
				Model:          "openai/gpt-5.2",
				TimeoutSeconds: 3,
				HTTPReferer:    "https://sub2api.local",
				Title:          "sub2api-test",
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	require.True(t, detector.Enabled())

	result, err := detector.Detect(context.Background(), "帮我分析一下 10 ETH 换成 USDC 的最优 dex 路由")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Matched)
	require.Equal(t, "dex-routing", result.Profile)
	require.Equal(t, "openai/gpt-5.2", result.Model)

	require.Equal(t, "Bearer or-key-test", capturedAuth)
	require.Equal(t, "https://sub2api.local", capturedReferer)
	require.Equal(t, "sub2api-test", capturedTitle)
	require.Equal(t, "openai/gpt-5.2", capturedBody["model"])

	messages, ok := capturedBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
}
