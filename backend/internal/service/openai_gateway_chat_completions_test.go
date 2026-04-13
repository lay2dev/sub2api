package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_UsesProviderChatEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_provider"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_provider_1",
			"object":"chat.completion",
			"model":"gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer inbound-should-not-forward")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77001,
		Name:        "owlia-crypto-provider",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	_, err := svc.ForwardChatCompletionsPassthrough(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`),
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://crypto-provider.example.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-provider", upstream.lastReq.Header.Get("Authorization"))
}

func TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_EmitsIndexedOutboundRequestLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logSink, restoreSink := captureStructuredLog(t)
	defer restoreSink()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_provider_log"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_provider_log_1",
			"object":"chat.completion",
			"model":"gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-5.2",
		"messages":[{"role":"user","content":"btc"}],
		"access_token":"secret-token"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Request-Id", "req-upstream-agent")
	c.Request.Header.Set("X-Client-Request-Id", "creq-upstream-agent")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          77010,
		Name:        "owlia-crypto-provider-log",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	_, err := svc.ForwardChatCompletionsPassthrough(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}],"access_token":"secret-token"}`),
		"",
	)
	require.NoError(t, err)
	require.True(t, logSink.ContainsMessageAtLevel("openai.upstream_agent_request", "info"))
	require.True(t, logSink.ContainsFieldValue("upstream_url", "https://crypto-provider.example.com/v1/chat/completions"))
	require.True(t, logSink.ContainsFieldValue("upstream_request_body", `"access_token":"***"`))
	require.True(t, logSink.ContainsFieldValue("upstream_request_body", `"content":"btc"`))
}

func TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_AppliesModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_provider_map"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_provider_map_1",
			"object":"chat.completion",
			"model":"owlia-gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77002,
		Name:        "owlia-crypto-provider-map",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":       "sk-provider",
			"base_url":      "https://crypto-provider.example.com",
			"model_mapping": map[string]any{"gpt-5.2": "owlia-gpt-5.2"},
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	_, err := svc.ForwardChatCompletionsPassthrough(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`),
		"",
	)
	require.NoError(t, err)
	require.Equal(t, "owlia-gpt-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_StreamWrapsProviderJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_provider_stream"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_provider_stream_1",
			"object":"chat.completion",
			"model":"gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.2","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"btc"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77003,
		Name:        "owlia-crypto-provider-stream",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	result, err := svc.ForwardChatCompletionsPassthrough(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"btc"}]}`),
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Contains(t, rec.Body.String(), `"object":"chat.completion.chunk"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_CryptoRouterAllowsMissingAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_provider_noauth"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_provider_noauth_1",
			"object":"chat.completion",
			"model":"gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77004,
		Name:        "owlia-crypto-provider-noauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"base_url": "http://127.0.0.1:8080",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	_, err := svc.ForwardChatCompletionsPassthrough(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`),
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Empty(t, upstream.lastReq.Header.Get("Authorization"))
}

func TestDeriveCryptoEnhancedPromptCacheKey_ChangesWhenEnhancedBodyChanges(t *testing.T) {
	first := deriveCryptoEnhancedPromptCacheKey(
		"prompt-cache-seed",
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"}]}`),
	)
	second := deriveCryptoEnhancedPromptCacheKey(
		"prompt-cache-seed",
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc"},{"role":"system","content":"<crypto_data>{\"price\":1}</crypto_data>"}]}`),
	)

	require.NotEmpty(t, first)
	require.NotEmpty(t, second)
	require.NotEqual(t, first, second)
	require.Empty(t, deriveCryptoEnhancedPromptCacheKey("", []byte(`{"model":"gpt-5.2"}`)))
}

func TestResolveOpenAICryptoPrefetchModel_PrefersProviderOwnedDefaultModel(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model":         "owlia-crypto-default",
			"model_mapping": map[string]any{"gpt-5.2": "mapped-user-facing-model"},
		},
	}

	got := resolveOpenAICryptoPrefetchModel(account, "gpt-5.2")
	require.Equal(t, "owlia-crypto-default", got)
}

func TestOpenAIGatewayService_PrepareCryptoEnhancedChatRequest_PrependsCryptoDataSystemMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-ID": []string{"rid_crypto_data"}},
		Body: io.NopCloser(strings.NewReader(`{
			"crypto":{
				"crypto_data":{
					"intent":"token_analysis",
					"sources":[
						{"name":"coinglass","status":"success","meta":{"adapter_names":["dexscreener","coinglass"],"tool_calls":["crypto-market.fetch_price"]}}
					]
				}
			}
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-5.2",
		"stream":true,
		"messages":[
			{"role":"system","content":"Answer in Chinese."},
			{"role":"user","content":"btc funding rate"}
		]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77005,
		Name:        "owlia-crypto-provider-enhance",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	prepared, err := svc.PrepareCryptoEnhancedChatRequest(
		context.Background(),
		c,
		account,
		[]byte(`{
			"model":"gpt-5.2",
			"stream":true,
			"messages":[
				{"role":"system","content":"Answer in Chinese."},
				{"role":"user","content":"btc funding rate"}
			]
		}`),
	)
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://crypto-provider.example.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-provider", upstream.lastReq.Header.Get("Authorization"))
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Empty(t, upstream.lastReq.Header.Get("Accept"))

	enhancedBody := prepared.EnhancedBody
	require.True(t, gjson.GetBytes(enhancedBody, "stream").Bool())
	require.Equal(t, "system", gjson.GetBytes(enhancedBody, "messages.0.role").String())
	require.Equal(t, "Answer in Chinese.", gjson.GetBytes(enhancedBody, "messages.0.content").String())
	require.Equal(t, "system", gjson.GetBytes(enhancedBody, "messages.1.role").String())
	require.Equal(t, "user", gjson.GetBytes(enhancedBody, "messages.2.role").String())
	require.Equal(t, "btc funding rate", gjson.GetBytes(enhancedBody, "messages.2.content").String())

	injected := gjson.GetBytes(enhancedBody, "messages.1.content").String()
	require.Contains(t, injected, "<crypto_data>")
	require.Contains(t, injected, `"intent":"token_analysis"`)
	require.Contains(t, injected, `"name":"coinglass"`)
	require.Contains(t, injected, "You have access to live crypto market data fetched for this request.")
	require.Contains(t, injected, "Available data sources:")
	require.Contains(t, injected, "crypto-market.fetch_price (dexscreener)")
	require.NotNil(t, prepared.PrefetchResult)
	require.Equal(t, []string{"dexscreener", "coinglass"}, prepared.AdapterNames)
	require.Len(t, prepared.ToolCalls, 1)
	require.Equal(t, "crypto-market.fetch_price", prepared.ToolCalls[0])
}

func TestOpenAIGatewayService_PrepareCryptoEnhancedChatRequest_ParsesCryptoDataFromStreamPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-ID": []string{"rid_crypto_data_stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_crypto_stream_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
				"data: {\"id\":\"chatcmpl_crypto_stream_1\",\"object\":\"chat.completion\",\"model\":\"gpt-5.2\",\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"crypto\":{\"crypto_data\":{\"intent\":\"token_analysis\",\"sources\":[{\"name\":\"coinglass\",\"status\":\"success\",\"meta\":{\"adapter_names\":[\"coinglass-stream\"]}}]}}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-5.2",
		"stream":true,
		"messages":[{"role":"user","content":"btc funding rate"}]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77008,
		Name:        "owlia-crypto-provider-enhance-stream",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	prepared, err := svc.PrepareCryptoEnhancedChatRequest(
		context.Background(),
		c,
		account,
		[]byte(`{
			"model":"gpt-5.2",
			"stream":true,
			"messages":[{"role":"user","content":"btc funding rate"}]
		}`),
	)
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Contains(t, gjson.GetBytes(prepared.EnhancedBody, "messages.0.content").String(), "<crypto_data>")
	require.Contains(t, gjson.GetBytes(prepared.EnhancedBody, "messages.0.content").String(), `"intent":"token_analysis"`)
	require.Equal(t, []string{"coinglass-stream"}, prepared.AdapterNames)
}

func TestOpenAIGatewayService_PrepareCryptoEnhancedChatRequest_UsesAccountDefaultModelWithMinimalCryptoEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-ID": []string{"rid_crypto_data_usage"}},
		Body: io.NopCloser(strings.NewReader(`{
			"crypto":{
				"crypto_data":{"intent":"token_analysis"}
			}
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.4","messages":[{"role":"user","content":"btc funding rate"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77006,
		Name:        "owlia-crypto-provider-default-model",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
			"model":    "owlia-crypto-default",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	prepared, err := svc.PrepareCryptoEnhancedChatRequest(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"btc funding rate"}]}`),
	)
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.Equal(t, "owlia-crypto-default", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotNil(t, prepared.PrefetchResult)
	require.Equal(t, 0, prepared.PrefetchResult.Usage.InputTokens)
	require.Equal(t, 0, prepared.PrefetchResult.Usage.OutputTokens)
	require.Equal(t, "owlia-crypto-default", prepared.PrefetchResult.BillingModel)
	require.Equal(t, "owlia-crypto-default", prepared.PrefetchResult.UpstreamModel)
}

func TestOpenAIGatewayService_PrepareCryptoEnhancedChatRequest_ErrorsWhenCryptoDataMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-ID": []string{"rid_crypto_data_missing"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_crypto_data_missing_1",
			"object":"chat.completion",
			"model":"gpt-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Crypto data ready."},"finish_reason":"stop"}],
			"crypto":{"detected":true}
		}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc funding rate"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          77007,
		Name:        "owlia-crypto-provider-missing-data",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-provider",
			"base_url": "https://crypto-provider.example.com",
		},
		Extra: map[string]any{
			"crypto_router": true,
		},
	}

	_, err := svc.PrepareCryptoEnhancedChatRequest(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.2","messages":[{"role":"user","content":"btc funding rate"}]}`),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "crypto.crypto_data")
}

func TestFormatCryptoDataSystemMessage_KnownIntentAndSources(t *testing.T) {
	raw := json.RawMessage(`{
		"intent": "uniswap",
		"sources": [
			{
				"name": "pool_data",
				"status": "success",
				"meta": {
					"adapter_names": ["geckoterminal"],
					"tool_calls": ["crypto-dex.fetch_pool_discovery"]
				}
			},
			{
				"name": "lp_data",
				"status": "success",
				"meta": {
					"adapter_names": ["revert"],
					"tool_calls": ["crypto-lp.fetch_positions"]
				}
			}
		]
	}`)

	msg := formatCryptoDataSystemMessage(raw)

	require.Contains(t, msg, "You have access to live crypto market data fetched for this request.")
	require.Contains(t, msg, "liquidity-weighted tick distribution")
	require.Contains(t, msg, "Available data sources:")
	require.Contains(t, msg, "crypto-dex.fetch_pool_discovery (geckoterminal): DEX pool discovery")
	require.Contains(t, msg, "crypto-lp.fetch_positions (revert): LP positions with tick ranges")
	require.Contains(t, msg, "<crypto_data>")
	require.Contains(t, msg, `"intent":"uniswap"`)
	require.Contains(t, msg, "</crypto_data>")

	// Header must appear before the data block
	require.Less(t, strings.Index(msg, "Available data sources:"), strings.Index(msg, "<crypto_data>"))
}

func TestFormatCryptoDataSystemMessage_UnknownIntentFallsBack(t *testing.T) {
	raw := json.RawMessage(`{
		"intent": "some-future-intent",
		"sources": [
			{
				"status": "success",
				"meta": {
					"adapter_names": ["foo"],
					"tool_calls": ["crypto-market.fetch_price"]
				}
			}
		]
	}`)

	msg := formatCryptoDataSystemMessage(raw)

	require.Contains(t, msg, "You have access to live crypto market data fetched for this request.")
	require.Contains(t, msg, "Available data sources:")
	require.Contains(t, msg, "crypto-market.fetch_price (foo)")
	require.Contains(t, msg, "<crypto_data>")
	// No intent guidance for unknown intent
	require.NotContains(t, msg, "Focus on")
}

func TestFormatCryptoDataSystemMessage_UnknownToolCallGetsGenericDescription(t *testing.T) {
	raw := json.RawMessage(`{
		"intent": "crypto-basic",
		"sources": [
			{
				"status": "success",
				"meta": {
					"adapter_names": ["new-adapter"],
					"tool_calls": ["crypto-future.fetch_something_new"]
				}
			}
		]
	}`)

	msg := formatCryptoDataSystemMessage(raw)

	require.Contains(t, msg, "crypto-future.fetch_something_new (new-adapter): live crypto data")
}

func TestFormatCryptoDataSystemMessage_FailedSourcesExcluded(t *testing.T) {
	raw := json.RawMessage(`{
		"intent": "defi-lending",
		"sources": [
			{
				"status": "success",
				"meta": {
					"adapter_names": ["aave"],
					"tool_calls": ["crypto-yield.fetch_snapshot"]
				}
			},
			{
				"status": "error",
				"meta": {
					"adapter_names": ["compound"],
					"tool_calls": ["crypto-dex-volume.fetch_snapshot"]
				}
			}
		]
	}`)

	msg := formatCryptoDataSystemMessage(raw)

	// Only the header portion should be checked; the raw JSON blob always contains all sources.
	header := msg[:strings.Index(msg, "<crypto_data>")]
	require.Contains(t, header, "crypto-yield.fetch_snapshot")
	require.NotContains(t, header, "crypto-dex-volume.fetch_snapshot")
}

func TestFormatCryptoDataSystemMessage_IntentOnlyNoSources(t *testing.T) {
	raw := json.RawMessage(`{"intent":"token_analysis"}`)

	msg := formatCryptoDataSystemMessage(raw)

	require.Contains(t, msg, "You have access to live crypto market data fetched for this request.")
	require.Contains(t, msg, "token-level analysis")
	require.NotContains(t, msg, "Available data sources:")
	require.Contains(t, msg, "<crypto_data>")
	require.Contains(t, msg, `"intent":"token_analysis"`)
}

func TestFormatCryptoDataSystemMessage_EmptyData(t *testing.T) {
	msg := formatCryptoDataSystemMessage(json.RawMessage(nil))

	require.Contains(t, msg, "You have access to live crypto market data fetched for this request.")
	require.Contains(t, msg, "<crypto_data>")
	require.Contains(t, msg, "{}")
}

func TestFormatCryptoDataSystemMessage_SourceWithNoAdapterName(t *testing.T) {
	raw := json.RawMessage(`{
		"intent": "crypto-basic",
		"sources": [
			{
				"status": "success",
				"meta": {
					"adapter_names": [],
					"tool_calls": ["crypto-news.fetch_newsflash"]
				}
			}
		]
	}`)

	msg := formatCryptoDataSystemMessage(raw)

	require.Contains(t, msg, "- crypto-news.fetch_newsflash: Recent crypto news")
	require.NotContains(t, msg, "crypto-news.fetch_newsflash ()")
}
