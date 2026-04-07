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

	expectedPrompt := "你在做一个分类任务。判断用户问题是否应该路由到一个加密货币助手，并判断具体属于哪个 profile。\n" +
		"可选的 profile 列表：\n" +
		"- token-research: token 研究、代币分析、团队背景、解锁信息\n" +
		"- crypto-basic: 加密市场分析、链上数据、whale 追踪、热点、资金流向\n" +
		"- defi-lending: DeFi 借贷风险、利率比较\n" +
		"- dex-routing: DEX swap 路由、交易路径优化\n" +
		"- uniswap: Uniswap 池子分析、LP 策略\n" +
		"请只返回 JSON，格式必须严格为 {\"match\": boolean, \"profile\": string}。\n" +
		"不要输出 markdown、代码块、解释文字或额外字段。\n" +
		"如果不属于 crypto，返回 {\"match\": false, \"profile\": \"none\"}。"

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
	require.Equal(t, "qwen/qwen3.5-122b-a10b", result.Model)

	require.Equal(t, "Bearer or-key-test", capturedAuth)
	require.Equal(t, "https://sub2api.local", capturedReferer)
	require.Equal(t, "sub2api-test", capturedTitle)
	require.Equal(t, "qwen/qwen3.5-122b-a10b", capturedBody["model"])
	require.Equal(t, true, capturedBody["stream"])

	messages, ok := capturedBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	systemMessage, ok := messages[0].(map[string]any)
	require.True(t, ok)
	systemContent, ok := systemMessage["content"].(string)
	require.True(t, ok)
	require.Equal(t, expectedPrompt, systemContent)
}

func TestOpenRouterCryptoProfileDetectorDetect_OpenAICompatibleParsesStreamChunks(t *testing.T) {
	t.Helper()

	var capturedBody map[string]any

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:        true,
				Endpoint:       "https://openai-compatible.test/v1/chat/completions",
				Provider:       "openai_compatible",
				APIKey:         "oa-key-test",
				TimeoutSeconds: 3,
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	detector.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
						"data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{\\\"\"},\"finish_reason\":null}]}\n\n" +
						"data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"match\"},\"finish_reason\":null}]}\n\n" +
						"data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\\\":true,\\\"profile\\\":\\\"crypto-basic\\\"}\"},\"finish_reason\":\"stop\"}]}\n\n" +
						"data: [DONE]\n\n",
				)),
			}, nil
		}),
	}

	result, err := detector.Detect(context.Background(), "给出btc价格")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Matched)
	require.Equal(t, "crypto-basic", result.Profile)
	require.Equal(t, "qwen/qwen3.5-122b-a10b", capturedBody["model"])
	require.Equal(t, true, capturedBody["stream"])
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

func TestOpenRouterCryptoProfileDetectorDetect_OpenAICompatibleSkipsOpenRouterHeaders(t *testing.T) {
	t.Helper()

	var capturedAuth string
	var capturedReferer string
	var capturedTitle string

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:        true,
				Endpoint:       "https://openai-compatible.test/v1/chat/completions",
				Provider:       "openai_compatible",
				APIKey:         "oa-key-test",
				TimeoutSeconds: 3,
				HTTPReferer:    "https://sub2api.local",
				Title:          "sub2api-test",
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	detector.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedAuth = r.Header.Get("Authorization")
			capturedReferer = r.Header.Get("HTTP-Referer")
			capturedTitle = r.Header.Get("X-Title")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"choices": [
						{
							"message": {
								"content": "{\"match\":true,\"profile\":\"crypto-basic\"}"
							}
						}
					]
				}`)),
			}, nil
		}),
	}

	result, err := detector.Detect(context.Background(), "最近 crypto 有什么热点")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Bearer oa-key-test", capturedAuth)
	require.Empty(t, capturedReferer)
	require.Empty(t, capturedTitle)
}

func TestOpenRouterCryptoProfileDetector_OpenRouterLegacyAPIKeyFallback(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Enabled:          true,
				Provider:         "openrouter",
				OpenRouterAPIKey: "legacy-or-key",
			},
		},
	}

	detector := NewOpenRouterCryptoProfileDetector(cfg)
	require.True(t, detector.Enabled())
	require.Equal(t, "legacy-or-key", detector.apiKey)
	require.Equal(t, "https://openrouter.ai/api/v1/chat/completions", detector.endpoint)
}

func TestParseOpenRouterCryptoProfilePayload_NoneProfileMapsToEmpty(t *testing.T) {
	result, err := parseOpenRouterCryptoProfilePayload(`{"match":false,"profile":"none"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Matched)
	require.Empty(t, result.Profile)
}
