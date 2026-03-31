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
	if reqLog == nil {
		reqLog = logger.L()
	}

	trimmed := strings.TrimSpace(message)
	detectorEnabled := detector != nil && detector.Enabled()
	if detector == nil || !detectorEnabled {
		reason := "detector_disabled"
		if detector == nil {
			reason = "detector_nil"
		}
		reqLog.Info("gateway.crypto_profile_detection_skipped",
			zap.String("entrypoint", entrypoint),
			zap.String("reason", reason),
			zap.Bool("detector_configured", detector != nil),
			zap.Bool("detector_enabled", detectorEnabled),
			zap.Int("message_chars", len(trimmed)),
		)
		return nil
	}

	if trimmed == "" {
		reqLog.Info("gateway.crypto_profile_detection_skipped",
			zap.String("entrypoint", entrypoint),
			zap.String("reason", "empty_message"),
			zap.Bool("detector_configured", true),
			zap.Bool("detector_enabled", true),
			zap.Int("message_chars", 0),
		)
		return nil
	}

	reqLog.Info("gateway.crypto_profile_detection_invoking",
		zap.String("entrypoint", entrypoint),
		zap.Bool("detector_configured", true),
		zap.Bool("detector_enabled", true),
		zap.Int("message_chars", len(trimmed)),
	)

	result, err := detector.Detect(ctx, trimmed)
	if err != nil {
		reqLog.Warn("gateway.crypto_profile_detection_failed",
			zap.String("entrypoint", entrypoint),
			zap.Int("message_chars", len(trimmed)),
			zap.Error(err),
		)
		return nil
	}
	if result == nil {
		reqLog.Info("gateway.crypto_profile_detection_empty_result",
			zap.String("entrypoint", entrypoint),
			zap.Int("message_chars", len(trimmed)),
		)
		return nil
	}
	if !result.Matched {
		reqLog.Info("gateway.crypto_profile_not_matched",
			zap.String("entrypoint", entrypoint),
			zap.String("crypto_profile", result.Profile),
			zap.String("detector_model", result.Model),
			zap.Int("message_chars", len(trimmed)),
		)
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
