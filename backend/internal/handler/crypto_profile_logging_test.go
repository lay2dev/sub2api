package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubCryptoProfileDetector struct {
	enabled bool
	result  *service.CryptoProfileMatchResult
	err     error
	gotText string
}

func (s *stubCryptoProfileDetector) Enabled() bool {
	return s.enabled
}

func (s *stubCryptoProfileDetector) Detect(_ context.Context, message string) (*service.CryptoProfileMatchResult, error) {
	s.gotText = message
	return s.result, s.err
}

func TestMaybeLogCryptoProfileMatch_LogsWhenDetectorMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureHandlerStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	detector := &stubCryptoProfileDetector{
		enabled: true,
		result: &service.CryptoProfileMatchResult{
			Matched: true,
			Profile: "dex-routing",
			Model:   "openai/gpt-5.2",
		},
	}

	reqLog := requestLogger(c, "handler.openai_gateway.chat_completions")
	maybeLogCryptoProfileMatch(c.Request.Context(), reqLog, detector, "swap 10 ETH to USDC", "/v1/chat/completions")

	require.Equal(t, "swap 10 ETH to USDC", detector.gotText)
	require.True(t, logSink.ContainsMessageAtLevel("gateway.crypto_profile_matched", "info"))
	require.True(t, logSink.ContainsFieldValue("crypto_profile", "dex-routing"))
	require.True(t, logSink.ContainsFieldValue("entrypoint", "/v1/chat/completions"))
	require.True(t, logSink.ContainsFieldValue("detector_model", "openai/gpt-5.2"))
}
