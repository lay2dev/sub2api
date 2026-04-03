import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'

import RedeemView from '../RedeemView.vue'

const { list, getAll } = vi.hoisted(() => ({
  list: vi.fn(),
  getAll: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list,
      generate: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      exportCodes: vi.fn(),
    },
    groups: {
      getAll,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showInfo: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true),
  }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
}))

const messages: Record<string, string> = {
  'admin.redeem.apiKeyTrial': 'Trial API Key',
  'admin.redeem.apiKeyTrialHint':
    'Generates 6-character code with backend-configured policy.',
  'admin.redeem.apiKeyTrialValueLabel': 'Trial API Key',
  'admin.redeem.apiKeyTrialUsageSummary': 'Used 3/100 · 97 remaining',
  'admin.redeem.status.partially_used': 'Partially Used',
  'common.delete': 'Delete',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const SelectStub = defineComponent({
  props: {
    options: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div class="select-stub">
      <span v-for="option in options" :key="option.value">{{ option.label }}</span>
    </div>
  `,
})

const DataTableStub = defineComponent({
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  computed: {
    firstRow() {
      return (this.data as any[])[0] ?? {}
    },
    firstValue() {
      return (this.firstRow as any).value ?? 0
    },
    firstStatus() {
      return (this.firstRow as any).status ?? ''
    },
    firstUsedBy() {
      return (this.firstRow as any).used_by ?? null
    },
    firstUsedAt() {
      return (this.firstRow as any).used_at ?? null
    },
  },
  template: `
    <div>
      <slot
        name="cell-value"
        :value="firstValue"
        :row="firstRow"
      />
      <slot
        name="cell-status"
        :value="firstStatus"
        :row="firstRow"
      />
      <slot
        name="cell-actions"
        :row="firstRow"
      />
    </div>
  `,
})

describe('admin RedeemView', () => {
  beforeEach(() => {
    list.mockReset()
    getAll.mockReset()

    list.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'ABC123',
          type: 'api_key_trial',
          value: 0,
          status: 'unused',
          used_by: null,
          used_at: null,
          max_uses: 100,
          used_count: 3,
          remaining_uses: 97,
          created_at: '2026-04-03T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
      page: 1,
      page_size: 20,
    })
    getAll.mockResolvedValue([])
  })

  it('shows api_key_trial in type selectors, hides amount input, and renders usage-aware display', async () => {
    const wrapper = mount(RedeemView, {
      global: {
        stubs: {
          teleport: true,
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    ;(wrapper.vm as any).showGenerateDialog = true
    ;(wrapper.vm as any).generateForm.type = 'api_key_trial'
    await nextTick()

    expect(wrapper.text()).toContain('Trial API Key')
    expect(wrapper.text()).toContain('Generates 6-character code with backend-configured policy.')
    expect(wrapper.text()).toContain('Used 3/100 · 97 remaining')
    expect(wrapper.text()).toContain('Partially Used')
    expect(wrapper.find('input[type="number"][step="0.01"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.redeem.selectGroup')
    expect(wrapper.text()).not.toContain('Delete')
  })
})
