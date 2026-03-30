package service

import (
	"context"
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
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "stream").Bool())
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
