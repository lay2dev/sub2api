import { describe, expect, it } from 'vitest'
import type { OpsSystemLog } from '@/api/admin/ops'
import { buildSystemLogDetail, getSystemLogRequestBody } from '../systemLogDetail'

describe('buildSystemLogDetail', () => {
  it('renders crypto prefetch fallback fields for ops system logs', () => {
    const detail = buildSystemLogDetail({
      id: 1,
      created_at: '2026-04-08T10:00:00Z',
      level: 'warn',
      component: 'handler.openai_gateway.chat_completions',
      message: 'openai_chat_completions.crypto_provider_prefetch_fallback',
      request_id: 'req-1',
      client_request_id: 'creq-1',
      account_id: 88,
      platform: 'openai',
      model: 'gpt-5.2',
      extra: {
        crypto_prefetch: true,
        fallback_to_upstream: true,
        prefetch_transport: 'responses',
        account_name: 'crypto-oauth',
        error: 'prefetch failed',
      },
    } satisfies OpsSystemLog)

    expect(detail).toContain('openai_chat_completions.crypto_provider_prefetch_fallback')
    expect(detail).toContain('crypto_prefetch=true')
    expect(detail).toContain('fallback_to_upstream=true')
    expect(detail).toContain('prefetch_transport=responses')
    expect(detail).toContain('account_name=crypto-oauth')
    expect(detail).toContain('error=prefetch failed')
    expect(detail).toContain('req=req-1')
    expect(detail).toContain('acc=88')
  })

  it('renders crypto adapter names and upstream request id from extra fields', () => {
    const detail = buildSystemLogDetail({
      id: 2,
      created_at: '2026-04-10T10:00:00Z',
      level: 'info',
      component: 'handler.openai_gateway.chat_completions',
      message: 'openai_chat_completions.crypto_provider_response_prepared',
      request_id: 'req-2',
      client_request_id: 'creq-2',
      user_id: 7,
      account_id: 42,
      platform: 'openai',
      model: 'gpt-5.4',
      extra: {
        upstream_request_id: 'rid_crypto_prefetch',
        crypto_adapter_names: ['dexscreener', 'coinglass'],
      },
    } satisfies OpsSystemLog)

    expect(detail).toContain('upstream_request_id=rid_crypto_prefetch')
    expect(detail).toContain('crypto_adapter_names=dexscreener,coinglass')
  })

  it('renders structured tool calls from crypto provider logs', () => {
    const detail = buildSystemLogDetail({
      id: 3,
      created_at: '2026-04-10T10:00:00Z',
      level: 'info',
      component: 'handler.openai_gateway.chat_completions',
      message: 'openai_chat_completions.crypto_provider_response_prepared',
      extra: {
        tool_calls: ['crypto-market.fetch_price'],
      },
    } satisfies OpsSystemLog)

    expect(detail).toContain('tool_calls=["crypto-market.fetch_price"]')
  })

  it('renders outbound upstream request metadata and body for agent requests', () => {
    const row = {
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
    } satisfies OpsSystemLog
    const detail = buildSystemLogDetail(row)

    expect(detail).toContain('upstream_url=https://crypto-provider.example.com/v1/chat/completions')
    expect(detail).toContain('upstream_path=/v1/chat/completions')
    expect(detail).toContain('account_name=owlia-crypto-provider-log')
    expect(detail).toContain('method=POST')
    expect(detail).toContain('openai_passthrough=true')
    expect(detail).not.toContain('upstream_request_body=')
    expect(getSystemLogRequestBody(row)).toBe('{"messages":[{"role":"user","content":"btc"}]}')
    expect(detail.match(/account_name=owlia-crypto-provider-log/g)).toHaveLength(1)
    expect(detail.match(/method=POST/g)).toHaveLength(1)
  })

  it('renders account_name only once when crypto and outbound fields coexist', () => {
    const detail = buildSystemLogDetail({
      id: 5,
      created_at: '2026-04-13T10:00:00Z',
      level: 'info',
      component: 'service.openai_gateway',
      message: 'openai.upstream_agent_request',
      extra: {
        account_name: 'shared-account',
        crypto_prefetch: true,
        upstream_request_id: 'rid-crypto',
        upstream_url: 'https://crypto-provider.example.com/v1/chat/completions',
        upstream_request_body: '{"messages":[{"role":"user","content":"btc"}]}',
      },
    } satisfies OpsSystemLog)

    expect(detail).toContain('account_name=shared-account')
    expect(detail.match(/account_name=shared-account/g)).toHaveLength(1)
  })
})
