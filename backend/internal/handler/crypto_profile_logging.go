package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func maybeLogCryptoProfileMatch(
	ctx context.Context,
	reqLog *zap.Logger,
	detector service.CryptoProfileDetector,
	message string,
	entrypoint string,
) {
	_ = detectCryptoProfileMatch(ctx, reqLog, detector, message, entrypoint)
}

func detectCryptoProfileMatch(
	ctx context.Context,
	reqLog *zap.Logger,
	detector service.CryptoProfileDetector,
	message string,
	entrypoint string,
) *service.CryptoProfileMatchResult {
	if detector == nil || !detector.Enabled() {
		return nil
	}

	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil
	}

	if reqLog == nil {
		reqLog = logger.L()
	}

	result, err := detector.Detect(ctx, trimmed)
	if err != nil {
		reqLog.Warn("gateway.crypto_profile_detection_failed",
			zap.String("entrypoint", entrypoint),
			zap.Error(err),
		)
		return nil
	}
	if result == nil || !result.Matched {
		return result
	}

	reqLog.Info("gateway.crypto_profile_matched",
		zap.String("entrypoint", entrypoint),
		zap.String("crypto_profile", result.Profile),
		zap.String("detector_model", result.Model),
		zap.Int("message_chars", len(trimmed)),
	)
	return result
}

func extractCryptoProfileMessageTextFromParsedRequest(parsed *service.ParsedRequest) string {
	if parsed == nil {
		return ""
	}
	return flattenCryptoProfileMessageTexts(parsed.Messages)
}

func extractCryptoProfileMessageTextFromMessagesBody(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return ""
	}

	var decoded []any
	if err := json.Unmarshal([]byte(messages.Raw), &decoded); err != nil {
		return ""
	}
	return flattenCryptoProfileMessageTexts(decoded)
}

func flattenCryptoProfileMessageTexts(messages []any) string {
	if len(messages) == 0 {
		return ""
	}

	parts := make([]string, 0, len(messages))
	for _, rawMessage := range messages {
		messageMap, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		role, _ := messageMap["role"].(string)
		if strings.TrimSpace(role) != "" && !strings.EqualFold(role, "user") {
			continue
		}

		switch content := messageMap["content"].(type) {
		case string:
			content = strings.TrimSpace(content)
			if content != "" {
				parts = append(parts, content)
			}
		case []any:
			for _, rawPart := range content {
				partMap, ok := rawPart.(map[string]any)
				if !ok {
					continue
				}
				text, _ := partMap["text"].(string)
				text = strings.TrimSpace(text)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
	}

	return strings.Join(parts, "\n\n")
}
