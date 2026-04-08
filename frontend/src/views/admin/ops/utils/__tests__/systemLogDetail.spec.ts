import { describe, expect, it } from 'vitest'
import type { OpsSystemLog } from '@/api/admin/ops'
import { buildSystemLogDetail } from '../systemLogDetail'

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
})
