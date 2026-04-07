package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const openRouterCryptoProfilePromptMaxChars = 4000
const openRouterCryptoProfileDetectMaxAttempts = 2
const (
	cryptoProfileDetectorProviderOpenRouter       = "openrouter"
	cryptoProfileDetectorProviderOpenAICompatible = "openai_compatible"
)

const cryptoProfileSystemPrompt = "你在做一个分类任务。判断用户问题是否应该路由到一个加密货币助手，并判断具体属于哪个 profile。\n" +
	"可选的 profile 列表：\n" +
	"- token-research: token 研究、代币分析、团队背景、解锁信息\n" +
	"- crypto-basic: 加密市场分析、链上数据、whale 追踪、热点、资金流向\n" +
	"- defi-lending: DeFi 借贷风险、利率比较\n" +
	"- dex-routing: DEX swap 路由、交易路径优化\n" +
	"- uniswap: Uniswap 池子分析、LP 策略\n" +
	"请只返回 JSON，格式必须严格为 {\"match\": boolean, \"profile\": string}。\n" +
	"不要输出 markdown、代码块、解释文字或额外字段。\n" +
	"如果不属于 crypto，返回 {\"match\": false, \"profile\": \"none\"}。"

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

// OpenRouterCryptoProfileDetector calls OpenRouter or OpenAI-compatible Chat Completions for profile classification.
type OpenRouterCryptoProfileDetector struct {
	enabled     bool
	provider    string
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
	Model          string                   `json:"model"`
	Stream         bool                     `json:"stream"`
	Messages       []openRouterChatMessage  `json:"messages"`
	Temperature    float64                  `json:"temperature"`
	MaxTokens      int                      `json:"max_tokens"`
	ResponseFormat openRouterResponseFormat `json:"response_format"`
}

type openRouterResponseFormat struct {
	Type       string                    `json:"type"`
	JSONSchema openRouterJSONSchemaField `json:"json_schema"`
}

type openRouterJSONSchemaField struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openRouterChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openRouterChatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
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

	provider, validProvider := normalizeCryptoProfileDetectorProvider(detectCfg.Provider)
	rawEndpoint := strings.TrimSpace(detectCfg.Endpoint)
	endpoint := rawEndpoint
	if provider == cryptoProfileDetectorProviderOpenRouter && endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1/chat/completions"
	}
	model := strings.TrimSpace(detectCfg.Model)
	if model == "" {
		model = "qwen/qwen3.5-122b-a10b"
	}

	apiKey := strings.TrimSpace(detectCfg.APIKey)
	if provider == cryptoProfileDetectorProviderOpenRouter && apiKey == "" {
		apiKey = strings.TrimSpace(detectCfg.OpenRouterAPIKey)
	}

	detector.provider = provider
	detector.endpoint = endpoint
	detector.apiKey = apiKey
	detector.model = model
	detector.httpReferer = strings.TrimSpace(detectCfg.HTTPReferer)
	detector.title = strings.TrimSpace(detectCfg.Title)
	detector.httpClient = &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	detector.enabled = detectCfg.Enabled && validProvider && detector.endpoint != "" && detector.apiKey != ""

	if detectCfg.Enabled && !validProvider {
		logger.LegacyPrintf("service.crypto_profile_detector", "crypto profile detection enabled but provider %q is unsupported; detector disabled", strings.TrimSpace(detectCfg.Provider))
	}
	if detectCfg.Enabled && detector.endpoint == "" {
		logger.LegacyPrintf("service.crypto_profile_detector", "crypto profile detection enabled but endpoint is empty for provider %q; detector disabled", detector.provider)
	}
	if detectCfg.Enabled && detector.apiKey == "" {
		logger.LegacyPrintf("service.crypto_profile_detector", "crypto profile detection enabled but api key is empty for provider %q; detector disabled", detector.provider)
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
		Model:  d.model,
		Stream: true,
		Messages: []openRouterChatMessage{
			{
				Role: "system",
				Content: cryptoProfileSystemPrompt,
			},
			{
				Role:    "user",
				Content: trimmedMessage,
			},
		},
		Temperature: 0,
		MaxTokens:   64,
		ResponseFormat: openRouterResponseFormat{
			Type: "json_schema",
			JSONSchema: openRouterJSONSchemaField{
				Name:   "crypto_profile_match",
				Strict: true,
				Schema: openRouterCryptoProfileJSONSchema(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openrouter request: %w", err)
	}
	var lastErr error
	for attempt := 1; attempt <= openRouterCryptoProfileDetectMaxAttempts; attempt++ {
		match, err := d.detectOnce(ctx, body)
		if err == nil {
			match.Model = d.model
			return match, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func truncateCryptoProfilePrompt(message string) string {
	if len(message) <= openRouterCryptoProfilePromptMaxChars {
		return message
	}
	return message[:openRouterCryptoProfilePromptMaxChars]
}

func normalizeCryptoProfileDetectorProvider(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", cryptoProfileDetectorProviderOpenRouter:
		return cryptoProfileDetectorProviderOpenRouter, true
	case cryptoProfileDetectorProviderOpenAICompatible, "openai-compatible":
		return cryptoProfileDetectorProviderOpenAICompatible, true
	default:
		return strings.ToLower(strings.TrimSpace(raw)), false
	}
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
	if strings.EqualFold(strings.TrimSpace(payload.Profile), "none") {
		payload.Profile = ""
	}
	if payload.Match && strings.TrimSpace(payload.Profile) == "" {
		payload.Profile = "crypto-basic"
	}

	return &CryptoProfileMatchResult{
		Matched: payload.Match,
		Profile: strings.TrimSpace(payload.Profile),
	}, nil
}

func (d *OpenRouterCryptoProfileDetector) detectOnce(ctx context.Context, body []byte) (*CryptoProfileMatchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if d.provider == cryptoProfileDetectorProviderOpenRouter && d.httpReferer != "" {
		req.Header.Set("HTTP-Referer", d.httpReferer)
	}
	if d.provider == cryptoProfileDetectorProviderOpenRouter && d.title != "" {
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

	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		content, err := decodeCryptoProfileStreamContent(resp.Body)
		if err != nil {
			return nil, err
		}
		return parseOpenRouterCryptoProfilePayload(content)
	}

	var completion openRouterChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("decode openrouter response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("openrouter response missing choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("openrouter response content is empty")
	}
	return parseOpenRouterCryptoProfilePayload(content)
}

func decodeCryptoProfileStreamContent(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		var chunk openRouterChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("decode openrouter stream: %w", err)
	}

	trimmed := strings.TrimSpace(content.String())
	if trimmed == "" {
		return "", fmt.Errorf("openrouter response content is empty")
	}
	return trimmed, nil
}

func openRouterCryptoProfileJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"match": map[string]any{
				"type": "boolean",
			},
			"profile": map[string]any{
				"type": "string",
				"enum": []string{
					"none",
					"crypto-basic",
					"token-research",
					"uniswap",
					"defi-lending",
					"dex-routing",
				},
			},
		},
		"required":             []string{"match", "profile"},
		"additionalProperties": false,
	}
}
