import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import OpsSystemLogTable from '../OpsSystemLogTable.vue'

const mockListSystemLogs = vi.fn()
const mockGetSystemLogSinkHealth = vi.fn()
const mockGetRuntimeLogConfig = vi.fn()
const mockUpdateRuntimeLogConfig = vi.fn()
const mockResetRuntimeLogConfig = vi.fn()
const mockCleanupSystemLogs = vi.fn()
const mockCopyToClipboard = vi.fn()
const mockShowError = vi.fn()
const mockShowSuccess = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    listSystemLogs: (...args: any[]) => mockListSystemLogs(...args),
    getSystemLogSinkHealth: (...args: any[]) => mockGetSystemLogSinkHealth(...args),
    getRuntimeLogConfig: (...args: any[]) => mockGetRuntimeLogConfig(...args),
    updateRuntimeLogConfig: (...args: any[]) => mockUpdateRuntimeLogConfig(...args),
    resetRuntimeLogConfig: (...args: any[]) => mockResetRuntimeLogConfig(...args),
    cleanupSystemLogs: (...args: any[]) => mockCleanupSystemLogs(...args),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: mockShowError,
    showSuccess: mockShowSuccess,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: { value: false },
    copyToClipboard: (...args: any[]) => mockCopyToClipboard(...args),
  }),
}))

describe('OpsSystemLogTable', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()

    mockListSystemLogs.mockResolvedValue({
      items: [
        {
          id: 88,
          created_at: '2026-04-13T13:54:20Z',
          level: 'info',
          component: 'service.openai_gateway',
          message: 'openai.upstream_agent_request',
          request_id: 'req-1',
          client_request_id: 'creq-1',
          account_id: 2,
          platform: 'openai',
          model: 'gpt-5.2',
          extra: {
            method: 'POST',
            upstream_url: 'https://llm2api.owlia.ai/v1/responses',
            upstream_path: '/v1/responses',
            account_name: 'owlia-provider',
            openai_passthrough: false,
            upstream_request_body: '{"model":"gpt-5.2","input":[{"role":"user","content":"btc"}]}',
          },
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    mockGetSystemLogSinkHealth.mockResolvedValue({
      queue_depth: 0,
      queue_capacity: 5000,
      dropped_count: 0,
      write_failed_count: 0,
      written_count: 1,
      avg_write_delay_ms: 1,
    })
    mockGetRuntimeLogConfig.mockResolvedValue({
      level: 'info',
      enable_sampling: false,
      sampling_initial: 100,
      sampling_thereafter: 100,
      caller: true,
      stacktrace_level: 'error',
      retention_days: 30,
    })
    mockCopyToClipboard.mockResolvedValue(true)
  })

  it('renders request body in a dedicated section with copy and expand controls', async () => {
    const wrapper = mount(OpsSystemLogTable, {
      global: {
        stubs: {
          Pagination: { template: '<div />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Request Body')
    expect(wrapper.text()).not.toContain('upstream_request_body=')

    const buttons = wrapper.findAll('button')
    const copyButton = buttons.find((button) => button.text().includes('复制'))
    const expandButton = buttons.find((button) => button.text().includes('展开'))

    expect(copyButton).toBeTruthy()
    expect(expandButton).toBeTruthy()

    await copyButton!.trigger('click')
    expect(mockCopyToClipboard).toHaveBeenCalledWith(
      '{"model":"gpt-5.2","input":[{"role":"user","content":"btc"}]}',
      'Request body copied',
    )

    await expandButton!.trigger('click')
    expect(wrapper.text()).toContain('收起')
  })
})
