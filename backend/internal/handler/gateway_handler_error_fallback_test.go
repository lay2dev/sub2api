package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type handlerBillingCacheStub struct {
	balance float64
	err     error
}

func (s handlerBillingCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.balance, nil
}

func (handlerBillingCacheStub) SetUserBalance(context.Context, int64, float64) error { return nil }
func (handlerBillingCacheStub) DeductUserBalance(context.Context, int64, float64) error { return nil }
func (handlerBillingCacheStub) InvalidateUserBalance(context.Context, int64) error { return nil }
func (handlerBillingCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*service.SubscriptionCacheData, error) {
	return nil, nil
}
func (handlerBillingCacheStub) SetSubscriptionCache(context.Context, int64, int64, *service.SubscriptionCacheData) error {
	return nil
}
func (handlerBillingCacheStub) UpdateSubscriptionUsage(context.Context, int64, int64, float64) error { return nil }
func (handlerBillingCacheStub) InvalidateSubscriptionCache(context.Context, int64, int64) error { return nil }
func (handlerBillingCacheStub) GetAPIKeyRateLimit(context.Context, int64) (*service.APIKeyRateLimitCacheData, error) {
	return nil, nil
}
func (handlerBillingCacheStub) SetAPIKeyRateLimit(context.Context, int64, *service.APIKeyRateLimitCacheData) error {
	return nil
}
func (handlerBillingCacheStub) UpdateAPIKeyRateLimitUsage(context.Context, int64, float64) error { return nil }
func (handlerBillingCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error { return nil }

func TestGatewayEnsureForwardErrorResponse_WritesFallbackWhenNotWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.True(t, wrote)
	require.Equal(t, http.StatusBadGateway, w.Code)

	var parsed map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "error", parsed["type"])
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errorObj["type"])
	assert.Equal(t, "Upstream request failed", errorObj["message"])
}

func TestGatewayEnsureForwardErrorResponse_DoesNotOverrideWrittenResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.String(http.StatusTeapot, "already written")

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.False(t, wrote)
	require.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "already written", w.Body.String())
}

func TestGatewayMessages_SkipsCryptoDetectionWhenBillingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude-3-5-sonnet-20241022",
		"messages":[{"role":"user","content":[{"type":"text","text":"analyze token price and dex routing"}]}]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(14)
	apiKey := &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User: &service.User{
			ID:          8,
			Concurrency: 1,
		},
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
		},
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	}
	detector := &stubCryptoProfileDetector{
		enabled: true,
		result: &service.CryptoProfileMatchResult{
			Matched: true,
			Profile: "token-research",
			Model:   "openai/gpt-5.2",
		},
	}

	handler := &GatewayHandler{
		gatewayService:        &service.GatewayService{},
		billingCacheService:   service.NewBillingCacheService(handlerBillingCacheStub{balance: 0}, nil, nil, nil, &config.Config{}),
		apiKeyService:         &service.APIKeyService{},
		cryptoProfileDetector: detector,
		concurrencyHelper:     NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	defer handler.billingCacheService.Stop()

	handler.Messages(c)

	require.Empty(t, detector.gotText)
	require.NotEqual(t, http.StatusOK, rec.Code)
}
