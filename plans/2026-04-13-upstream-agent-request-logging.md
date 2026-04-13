# Upstream Agent Request Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Log outbound upstream agent requests into indexed ops system logs so operators can search and inspect full sanitized request bodies from the existing frontend system-log page.

**Architecture:** Add a dedicated outbound-request logging helper at the OpenAI gateway boundary, emit a single indexed sink event before each upstream HTTP send, and reuse the existing ops system-log query path that already searches `extra::text`. Extend the frontend system-log detail formatter to render the new outbound-request fields without introducing new APIs or schema changes.

**Tech Stack:** Go, Gin, Zap/logger sink, Vitest, Vue 3, existing ops system-log pipeline

---

## File Map

- Modify: `backend/internal/service/openai_gateway_service.go`
  - Reuse the shared helper from the main OpenAI HTTP forwarding path before `httpUpstream.Do(...)`.
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
  - Reuse the shared helper from chat-completions passthrough before `httpUpstream.Do(...)`.
- Create: `backend/internal/service/openai_upstream_request_logging.go`
  - Hold the dedicated outbound-request logging helper, sanitization/truncation logic, and message constants.
- Modify: `backend/internal/service/ops_system_log_sink.go`
  - Allow the new `info` event to be indexed.
- Modify: `backend/internal/service/ops_system_log_sink_test.go`
  - Cover indexing for the new message and redacted/truncated payload persistence.
- Modify: `backend/internal/service/openai_gateway_chat_completions_test.go`
  - Add a forwarding-path test proving the sink event is emitted with sanitized searchable payload.
- Modify: `frontend/src/views/admin/ops/utils/systemLogDetail.ts`
  - Render outbound-request URL/path/body/truncation fields in the system-log detail column.
- Modify: `frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts`
  - Cover rendering of the new fields.

## Task 1: Add backend TDD coverage for outbound request logging

**Files:**
- Modify: `backend/internal/service/openai_gateway_chat_completions_test.go`
- Modify: `backend/internal/service/ops_system_log_sink_test.go`
- Reference: `backend/internal/service/openai_oauth_passthrough_test.go`

- [ ] **Step 1: Write the failing forwarding-path test for emitted outbound request logs**

Add a new test near the existing chat-completions passthrough tests in `backend/internal/service/openai_gateway_chat_completions_test.go`:

```go
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
```

- [ ] **Step 2: Run the narrow Go test and verify it fails for the expected reason**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/backend
go test -tags=unit ./internal/service -run 'TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_EmitsIndexedOutboundRequestLog' -count=1
```

Expected:

- FAIL because `openai.upstream_agent_request` is not emitted yet
- assertions like `ContainsMessageAtLevel(...)=false`

- [ ] **Step 3: Write the failing sink-index test for the new info message**

Add this case to `TestOpsSystemLogSink_ShouldIndex` in `backend/internal/service/ops_system_log_sink_test.go`:

```go
{
	name: "openai upstream agent request message",
	event: &logger.LogEvent{
		Level:     "info",
		Component: "service.openai_gateway",
		Message:   "openai.upstream_agent_request",
	},
	want: true,
},
```

Add one persistence-focused test in the same file:

```go
func TestOpsSystemLogSink_FlushRedactsAndPersistsOutboundRequestBody(t *testing.T) {
	done := make(chan struct{}, 1)
	var captured []*OpsInsertSystemLogInput
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(_ context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			captured = append(captured, inputs...)
			select {
			case done <- struct{}{}:
			default:
			}
			return int64(len(inputs)), nil
		},
	}

	sink := NewOpsSystemLogSink(repo)
	sink.batchSize = 1
	sink.flushInterval = 10 * time.Millisecond
	sink.Start()
	defer sink.Stop()

	logger.WriteSinkEvent("info", "service.openai_gateway", "openai.upstream_agent_request", map[string]any{
		"request_id":            "req-ops-body",
		"client_request_id":     "creq-ops-body",
		"platform":              "openai",
		"model":                 "gpt-5.2",
		"upstream_url":          "https://crypto-provider.example.com/v1/chat/completions",
		"upstream_request_body": `{"access_token":"secret-token","messages":[{"role":"user","content":"btc"}]}`,
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for sink flush")
	}

	require.Len(t, captured, 1)
	require.Contains(t, captured[0].ExtraJSON, `"upstream_request_body":"{\"access_token\":\"***\"`)
	require.Contains(t, captured[0].ExtraJSON, `btc`)
}
```

- [ ] **Step 4: Run the sink tests and verify they fail**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/backend
go test -tags=unit ./internal/service -run 'TestOpsSystemLogSink_(ShouldIndex|FlushRedactsAndPersistsOutboundRequestBody)' -count=1
```

Expected:

- FAIL because `shouldIndex` does not yet include the new message
- FAIL because no emitted event path exists yet for the forwarding test

- [ ] **Step 5: Commit the failing-test checkpoint**

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
git add backend/internal/service/openai_gateway_chat_completions_test.go backend/internal/service/ops_system_log_sink_test.go
git commit -m "test: cover upstream agent request logging"
```

## Task 2: Implement the backend logging helper and indexing rules

**Files:**
- Create: `backend/internal/service/openai_upstream_request_logging.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
- Modify: `backend/internal/service/ops_system_log_sink.go`

- [ ] **Step 1: Add the shared outbound-request logging helper**

Create `backend/internal/service/openai_upstream_request_logging.go` with the helper and constants:

```go
package service

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIUpstreamAgentRequestMessage   = "openai.upstream_agent_request"
	openAIUpstreamAgentRequestComponent = "service.openai_gateway"
	openAIUpstreamRequestBodyLimitBytes = 128 << 10
)

func logOpenAIUpstreamAgentRequest(c *gin.Context, account *Account, upstreamReq *http.Request, body []byte, reqStream bool) {
	if upstreamReq == nil {
		return
	}

	fields := map[string]any{
		"method": strings.TrimSpace(upstreamReq.Method),
		"stream": reqStream,
	}

	if account != nil {
		fields["account_id"] = account.ID
		fields["account_name"] = strings.TrimSpace(account.Name)
		fields["platform"] = strings.TrimSpace(account.Platform)
	}

	if upstreamReq.URL != nil {
		fields["upstream_url"] = strings.TrimSpace(upstreamReq.URL.String())
		fields["upstream_path"] = strings.TrimSpace(upstreamReq.URL.Path)
	}

	if c != nil {
		if v, ok := c.Get("openai_passthrough"); ok {
			if passthrough, ok := v.(bool); ok {
				fields["openai_passthrough"] = passthrough
			}
		}
		if reqID := strings.TrimSpace(c.GetHeader("X-Request-Id")); reqID != "" {
			fields["request_id"] = reqID
		}
		if clientReqID := strings.TrimSpace(c.GetHeader("X-Client-Request-Id")); clientReqID != "" {
			fields["client_request_id"] = clientReqID
		}
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
```

- [ ] **Step 2: Wire the helper into the main OpenAI HTTP forwarding path**

In `backend/internal/service/openai_gateway_service.go`, call the helper after `proxyURL` is resolved and before `httpUpstream.Do(...)`:

```go
		// Get proxy URL
		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}

		logOpenAIUpstreamAgentRequest(c, account, upstreamReq, body, reqStream)

		// Send request
		upstreamStart := time.Now()
		resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
```

This keeps the provider transport log at the gateway boundary, not in the repository layer.

- [ ] **Step 3: Wire the helper into chat-completions passthrough**

In `backend/internal/service/openai_gateway_chat_completions.go`, add the same call before sending the passthrough request:

```go
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	setOpsUpstreamRequestBody(c, forwardBody)
	if c != nil {
		c.Set("openai_passthrough", true)
	}
	logOpenAIUpstreamAgentRequest(c, account, upstreamReq, forwardBody, reqStream)

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
```

- [ ] **Step 4: Allow the new message through the ops sink indexing gate**

Update `backend/internal/service/ops_system_log_sink.go`:

```go
	message := strings.TrimSpace(event.Message)
	if message == "openai_chat_completions.crypto_provider_response_prepared" {
		return true
	}
	if message == openAIUpstreamAgentRequestMessage {
		return true
	}
```

If the constant creates a package dependency loop or readability issue, use the literal string once in `shouldIndex`, but keep the message name identical.

- [ ] **Step 5: Run the narrow backend tests and verify they pass**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/backend
go test -tags=unit ./internal/service -run 'TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_EmitsIndexedOutboundRequestLog|TestOpsSystemLogSink_(ShouldIndex|FlushRedactsAndPersistsOutboundRequestBody)' -count=1
```

Expected:

- PASS
- emitted sink event includes the sanitized searchable request body
- sink indexing test accepts the new info-level message

- [ ] **Step 6: Commit the backend implementation**

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
git add backend/internal/service/openai_upstream_request_logging.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/ops_system_log_sink.go backend/internal/service/openai_gateway_chat_completions_test.go backend/internal/service/ops_system_log_sink_test.go
git commit -m "feat: log outbound upstream agent requests"
```

## Task 3: Render outbound request details in the frontend system-log view

**Files:**
- Modify: `frontend/src/views/admin/ops/utils/systemLogDetail.ts`
- Modify: `frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts`

- [ ] **Step 1: Write the failing frontend rendering test**

Add this case to `frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts`:

```ts
it('renders outbound upstream request body and target metadata', () => {
  const detail = buildSystemLogDetail({
    id: 4,
    created_at: '2026-04-13T10:00:00Z',
    level: 'info',
    component: 'service.openai_gateway',
    message: 'openai.upstream_agent_request',
    request_id: 'req-upstream-agent',
    client_request_id: 'creq-upstream-agent',
    account_id: 77,
    platform: 'openai',
    model: 'gpt-5.2',
    extra: {
      account_name: 'owlia-crypto-provider-log',
      upstream_url: 'https://crypto-provider.example.com/v1/chat/completions',
      upstream_path: '/v1/chat/completions',
      method: 'POST',
      stream: false,
      openai_passthrough: true,
      upstream_request_body: '{"messages":[{"role":"user","content":"btc"}]}',
      upstream_request_body_truncated: false,
    },
  } satisfies OpsSystemLog)

  expect(detail).toContain('upstream_url=https://crypto-provider.example.com/v1/chat/completions')
  expect(detail).toContain('upstream_path=/v1/chat/completions')
  expect(detail).toContain('method=POST')
  expect(detail).toContain('openai_passthrough=true')
  expect(detail).toContain('upstream_request_body={"messages":[{"role":"user","content":"btc"}]}')
})
```

- [ ] **Step 2: Run the Vitest file and verify it fails**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/frontend
pnpm test:run src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts
```

Expected:

- FAIL because `buildSystemLogDetail` does not yet include the new outbound-request fields

- [ ] **Step 3: Implement detail rendering for the new fields**

Update `frontend/src/views/admin/ops/utils/systemLogDetail.ts`:

```ts
  const upstreamURL = getExtraString(extra, 'upstream_url')
  const upstreamPath = getExtraString(extra, 'upstream_path')
  const accountName = getExtraString(extra, 'account_name')
  const upstreamMethod = getExtraString(extra, 'method')
  const openAIPassthrough = getExtraString(extra, 'openai_passthrough')
  const upstreamRequestBody = getExtraString(extra, 'upstream_request_body')
  const upstreamRequestBodyTruncated = getExtraString(extra, 'upstream_request_body_truncated')

  const outboundParts: string[] = []
  if (accountName) outboundParts.push(`account_name=${accountName}`)
  if (upstreamURL) outboundParts.push(`upstream_url=${upstreamURL}`)
  if (upstreamPath) outboundParts.push(`upstream_path=${upstreamPath}`)
  if (upstreamMethod) outboundParts.push(`method=${upstreamMethod}`)
  if (openAIPassthrough) outboundParts.push(`openai_passthrough=${openAIPassthrough}`)
  if (upstreamRequestBodyTruncated) outboundParts.push(`upstream_request_body_truncated=${upstreamRequestBodyTruncated}`)
  if (upstreamRequestBody) outboundParts.push(`upstream_request_body=${upstreamRequestBody}`)
  if (outboundParts.length > 0) parts.push(outboundParts.join(' '))
```

Keep the existing crypto-prefetch rendering intact; this new block should be additive.

- [ ] **Step 4: Run the frontend test and verify it passes**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/frontend
pnpm test:run src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts
```

Expected:

- PASS

- [ ] **Step 5: Commit the frontend update**

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
git add frontend/src/views/admin/ops/utils/systemLogDetail.ts frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts
git commit -m "feat: show upstream agent request logs in ops detail"
```

## Task 4: Run broader verification and finish the change

**Files:**
- Modify: none expected unless verification reveals gaps

- [ ] **Step 1: Run the focused backend package tests**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/backend
go test -tags=unit ./internal/service -run 'TestOpenAIGatewayService_ForwardChatCompletionsPassthrough_|TestOpsSystemLogSink_' -count=1
```

Expected:

- PASS for the touched service tests

- [ ] **Step 2: Run the focused frontend test file**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api/frontend
pnpm test:run src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts
```

Expected:

- PASS

- [ ] **Step 3: Run repository validation required by AGENTS.md**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
bash scripts/validate.sh
```

Expected:

- `PASS: validate.sh`

- [ ] **Step 4: Inspect the final diff before handoff**

Run:

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
git diff --stat HEAD~3..HEAD
git diff -- backend/internal/service/openai_upstream_request_logging.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/ops_system_log_sink.go frontend/src/views/admin/ops/utils/systemLogDetail.ts
```

Expected:

- only the planned observability files changed
- no usage-log schema or request-details API drift

- [ ] **Step 5: Create the final implementation commit**

```bash
cd /Users/zz/.codex/worktrees/a633/sub2api
git add backend/internal/service/openai_upstream_request_logging.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/ops_system_log_sink.go backend/internal/service/openai_gateway_chat_completions_test.go backend/internal/service/ops_system_log_sink_test.go frontend/src/views/admin/ops/utils/systemLogDetail.ts frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts plans/2026-04-13-upstream-agent-request-logging.md
git commit -m "feat: add searchable upstream agent request logs"
```

## Self-Review

### Spec coverage

- Outbound upstream request is printed into logs: covered by Task 1 and Task 2
- Request body is searchable from frontend system logs: covered by Task 2 and Task 3
- Operators can inspect routing context with the request: covered by Task 2 and Task 3
- Validation before handoff: covered by Task 4

No spec gaps found.

### Placeholder scan

- No `TBD`, `TODO`, or deferred implementation markers remain.
- Each code-changing step includes concrete code.
- Each verification step includes a concrete command and expected result.

### Type consistency

- Shared helper name is `logOpenAIUpstreamAgentRequest`
- Event message name is `openai.upstream_agent_request`
- Body field is `upstream_request_body`
- Truncation flag is `upstream_request_body_truncated`

These names are used consistently across backend logging, sink indexing, and frontend rendering.
