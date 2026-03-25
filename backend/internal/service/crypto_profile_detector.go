package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const openRouterCryptoProfilePromptMaxChars = 4000

// CryptoProfileDetector classifies whether a request should be treated as a crypto/web3 profile request.
type CryptoProfileDetector interface {
	Enabled() bool
	Detect(ctx context.Context, message string) (*CryptoProfileMatchResult, error)
}

// CryptoProfileMatchResult is the normalized detector output used by handler logs.
type CryptoProfileMatchResult struct {
	Matched bool
	Profile string
	Model   string
}

// OpenRouterCryptoProfileDetector calls OpenRouter Chat Completions for profile classification.
type OpenRouterCryptoProfileDetector struct {
	enabled     bool
	endpoint    string
	apiKey      string
	model       string
	httpReferer string
	title       string
	httpClient  *http.Client
}

type openRouterChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []openRouterChatMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens"`
}

type openRouterChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openRouterCryptoProfilePayload struct {
	Match   bool   `json:"match"`
	Profile string `json:"profile"`
}

// NewOpenRouterCryptoProfileDetector builds a fail-open detector.
// If disabled or missing required configuration, the detector becomes a no-op.
func NewOpenRouterCryptoProfileDetector(cfg *config.Config) *OpenRouterCryptoProfileDetector {
	detector := &OpenRouterCryptoProfileDetector{
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}
	if cfg == nil {
		return detector
	}

	detectCfg := cfg.Gateway.CryptoProfileDetection
	timeoutSeconds := detectCfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 3
	}

	endpoint := strings.TrimSpace(detectCfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1/chat/completions"
	}
	model := strings.TrimSpace(detectCfg.Model)
	if model == "" {
		model = "openai/gpt-5.2"
	}

	detector.endpoint = endpoint
	detector.apiKey = strings.TrimSpace(detectCfg.OpenRouterAPIKey)
	detector.model = model
	detector.httpReferer = strings.TrimSpace(detectCfg.HTTPReferer)
	detector.title = strings.TrimSpace(detectCfg.Title)
	detector.httpClient = &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	detector.enabled = detectCfg.Enabled && detector.apiKey != ""

	if detectCfg.Enabled && detector.apiKey == "" {
		logger.LegacyPrintf("service.crypto_profile_detector", "OpenRouter crypto profile detection enabled but openrouter_api_key is empty; detector disabled")
	}

	return detector
}

func (d *OpenRouterCryptoProfileDetector) Enabled() bool {
	return d != nil && d.enabled
}

func (d *OpenRouterCryptoProfileDetector) Detect(ctx context.Context, message string) (*CryptoProfileMatchResult, error) {
	if !d.Enabled() {
		return nil, nil
	}

	trimmedMessage := truncateCryptoProfilePrompt(strings.TrimSpace(message))
	if trimmedMessage == "" {
		return nil, nil
	}

	payload := openRouterChatCompletionRequest{
		Model: d.model,
		Messages: []openRouterChatMessage{
			{
				Role: "system",
				Content: "You classify whether a user request matches a crypto/web3 profile. " +
					"Return only compact JSON: " +
					"{\"match\":true|false,\"profile\":\"crypto-basic|token-research|uniswap|defi-lending|dex-routing|\"}. " +
					"Mark true for blockchain, tokens, wallets, swaps, DEX routing, DeFi, onchain analysis, protocols, stablecoins, bridges, NFT, gas or chain-specific requests. " +
					"Mark false for general coding, productivity, or non-crypto uses of words like token or wallet. " +
					"When true but no specialized profile is obvious, use crypto-basic.",
			},
			{
				Role:    "user",
				Content: trimmedMessage,
			},
		},
		Temperature: 0,
		MaxTokens:   64,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if d.httpReferer != "" {
		req.Header.Set("HTTP-Referer", d.httpReferer)
	}
	if d.title != "" {
		req.Header.Set("X-Title", d.title)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send openrouter request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter returned status %d", resp.StatusCode)
	}

	var completion openRouterChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("decode openrouter response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("openrouter response missing choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	match, err := parseOpenRouterCryptoProfilePayload(content)
	if err != nil {
		return nil, err
	}
	match.Model = d.model
	return match, nil
}

func truncateCryptoProfilePrompt(message string) string {
	if len(message) <= openRouterCryptoProfilePromptMaxChars {
		return message
	}
	return message[:openRouterCryptoProfilePromptMaxChars]
}

func parseOpenRouterCryptoProfilePayload(content string) (*CryptoProfileMatchResult, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var payload openRouterCryptoProfilePayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("parse openrouter classifier response: %w", err)
	}
	if payload.Match && strings.TrimSpace(payload.Profile) == "" {
		payload.Profile = "crypto-basic"
	}

	return &CryptoProfileMatchResult{
		Matched: payload.Match,
		Profile: strings.TrimSpace(payload.Profile),
	}, nil
}
