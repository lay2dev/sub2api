package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOpenRouterCryptoProfileDetectorDetect_Match(t *testing.T) {
	t.Helper()

	var capturedAuth string
	var capturedReferer string
	var capturedTitle string
	var capturedBody map[string]any

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:          true,
				Endpoint:         "https://openrouter.test/api/v1/chat/completions",
				OpenRouterAPIKey: "or-key-test",
				Model:            "openai/gpt-5.2",
				TimeoutSeconds:   3,
				HTTPReferer:      "https://sub2api.local",
				Title:            "sub2api-test",
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	detector.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedAuth = r.Header.Get("Authorization")
			capturedReferer = r.Header.Get("HTTP-Referer")
			capturedTitle = r.Header.Get("X-Title")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"choices": [
						{
							"message": {
								"content": "{\"match\":true,\"profile\":\"dex-routing\"}"
							}
						}
					]
				}`)),
			}, nil
		}),
	}
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

func TestOpenRouterCryptoProfileDetectorDetect_UsesStructuredOutputsAndRetriesOnInvalidJSON(t *testing.T) {
	t.Helper()

	var callCount int
	var capturedBodies []map[string]any

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:          true,
				Endpoint:         "https://openrouter.test/api/v1/chat/completions",
				OpenRouterAPIKey: "or-key-test",
				Model:            "openai/gpt-5.2",
				TimeoutSeconds:   3,
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	detector.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			callCount++
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			capturedBodies = append(capturedBodies, body)

			content := `{"match":true,"profile":"crypto-basic"}`
			if callCount == 1 {
				content = `{"match":true,"profile`
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"choices": [
						{
							"message": {
								"content": ` + strconv.Quote(content) + `
							}
						}
					]
				}`)),
			}, nil
		}),
	}
	result, err := detector.Detect(context.Background(), "xxx 地址的分析")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Matched)
	require.Equal(t, "crypto-basic", result.Profile)
	require.Equal(t, 2, callCount)
	require.Len(t, capturedBodies, 2)

	responseFormat, ok := capturedBodies[0]["response_format"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "json_schema", responseFormat["type"])

	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, jsonSchema["strict"])
}

func TestParseOpenRouterCryptoProfilePayload_NoneProfileMapsToEmpty(t *testing.T) {
	result, err := parseOpenRouterCryptoProfilePayload(`{"match":false,"profile":"none"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Matched)
	require.Empty(t, result.Profile)
}
