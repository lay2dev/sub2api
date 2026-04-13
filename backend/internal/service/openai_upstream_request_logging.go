package service

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/tidwall/gjson"
)

const (
	openAIUpstreamAgentRequestMessage   = "openai.upstream_agent_request"
	openAIUpstreamAgentRequestComponent = "service.openai_gateway"
	openAIUpstreamRequestBodyLimitBytes = 128 << 10
)

type openAIUpstreamRequestLogOptions struct {
	RequestID         string
	ClientRequestID   string
	OpenAIPassthrough *bool
}

func logOpenAIUpstreamAgentRequest(
	account *Account,
	upstreamReq *http.Request,
	body []byte,
	reqStream bool,
	opts openAIUpstreamRequestLogOptions,
) {
	if upstreamReq == nil {
		return
	}

	fields := map[string]any{
		"method": strings.TrimSpace(upstreamReq.Method),
		"stream": reqStream,
	}

	if account != nil {
		fields["account_id"] = account.ID
		if accountName := strings.TrimSpace(account.Name); accountName != "" {
			fields["account_name"] = accountName
		}
		if platform := strings.TrimSpace(account.Platform); platform != "" {
			fields["platform"] = platform
		}
	}

	if upstreamReq.URL != nil {
		if upstreamURL := sanitizeOpenAIUpstreamURL(upstreamReq.URL); upstreamURL != "" {
			fields["upstream_url"] = upstreamURL
		}
		if upstreamPath := strings.TrimSpace(upstreamReq.URL.Path); upstreamPath != "" {
			fields["upstream_path"] = upstreamPath
		}
	}

	if requestID := strings.TrimSpace(opts.RequestID); requestID != "" {
		fields["request_id"] = requestID
	}
	if clientRequestID := strings.TrimSpace(opts.ClientRequestID); clientRequestID != "" {
		fields["client_request_id"] = clientRequestID
	}
	if opts.OpenAIPassthrough != nil {
		fields["openai_passthrough"] = *opts.OpenAIPassthrough
	}

	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); model != "" {
		fields["model"] = model
	}

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) > 0 {
		sanitized := logredact.RedactText(string(trimmedBody))
		if len(sanitized) > openAIUpstreamRequestBodyLimitBytes {
			fields["upstream_request_body"] = sanitized[:openAIUpstreamRequestBodyLimitBytes]
			fields["upstream_request_body_truncated"] = true
		} else {
			fields["upstream_request_body"] = sanitized
		}
	}

	logger.WriteSinkEvent("info", openAIUpstreamAgentRequestComponent, openAIUpstreamAgentRequestMessage, fields)
}

func sanitizeOpenAIUpstreamURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	path := strings.TrimSpace(u.EscapedPath())
	if path == "" {
		path = strings.TrimSpace(u.Path)
	}
	sanitized := (&url.URL{
		Scheme: strings.TrimSpace(u.Scheme),
		Host:   strings.TrimSpace(u.Host),
		Path:   path,
	}).String()
	return strings.TrimSpace(sanitized)
}
