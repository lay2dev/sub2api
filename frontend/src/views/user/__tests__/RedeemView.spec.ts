import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const {
  redeem,
  getHistory,
  getPublicSettings,
  refreshUser,
  fetchActiveSubscriptions,
  showError,
  showWarning,
  showSuccess,
  copyToClipboard,
} = vi.hoisted(() => ({
  redeem: vi.fn(),
  getHistory: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    redeem,
    getHistory,
  },
  authAPI: {
    getPublicSettings,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: {
      balance: 0,
      concurrency: 1,
    },
    refreshUser,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showWarning,
    showSuccess,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

const messages: Record<string, string> = {
  'redeem.currentBalance': 'Current Balance',
  'redeem.concurrency': 'Concurrency',
  'redeem.requests': 'requests',
  'redeem.redeemCodeLabel': 'Redeem Code',
  'redeem.redeemCodePlaceholder': 'Enter your redeem code',
  'redeem.redeemCodeHint': 'Redeem codes are case-sensitive',
  'redeem.redeeming': 'Redeeming...',
  'redeem.redeemButton': 'Redeem Code',
  'redeem.redeemSuccess': 'Code Redeemed Successfully!',
  'redeem.codeRedeemSuccess': 'Code redeemed successfully!',
  'redeem.redeemFailed': 'Redemption Failed',
  'redeem.aboutCodes': 'About Redeem Codes',
  'redeem.codeRule1': 'rule1',
  'redeem.codeRule2': 'rule2',
  'redeem.codeRule3': 'rule3',
  'redeem.codeRule4': 'rule4',
  'redeem.recentActivity': 'Recent Activity',
  'redeem.historyWillAppear': 'history',
  'redeem.trialApiKeyTitle': 'Your Trial API Key',
  'redeem.trialApiKeyHint': 'Shown only once',
  'redeem.trialApiKeyLabel': 'API Key',
  'redeem.trialQuotaLabel': 'Quota',
  'redeem.trialExpiresAtLabel': 'Expires At',
  'redeem.trialApiKeyCopied': 'API key copied',
  'redeem.copyTrialApiKey': 'Copy',
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

describe('user RedeemView', () => {
  beforeEach(() => {
    redeem.mockReset()
    getHistory.mockReset()
    getPublicSettings.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()

    getHistory.mockResolvedValue([])
    getPublicSettings.mockResolvedValue({ contact_info: '' })
    refreshUser.mockResolvedValue(undefined)
    copyToClipboard.mockResolvedValue(true)
  })

  it('renders one-time trial api key details when redeem succeeds with api_key_trial', async () => {
    redeem.mockResolvedValue({
      message: 'Trial key issued.',
      type: 'api_key_trial',
      value: 0,
      issued_api_key: {
        key: 'sk-trial-once',
        quota: 20,
        expires_at: '2026-04-10T00:00:00Z',
      },
    })

    const wrapper = mount(RedeemView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('#code').setValue('TRIAL-CODE')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(redeem).toHaveBeenCalledWith('TRIAL-CODE')
    expect(wrapper.text()).toContain('Trial key issued.')
    expect(wrapper.text()).toContain('Your Trial API Key')
    expect(wrapper.text()).toContain('sk-trial-once')
    expect(wrapper.text()).toContain('20 USD')
    expect(wrapper.text()).toContain('2026')

    await wrapper.get('[data-testid="redeem-trial-copy"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('sk-trial-once', 'API key copied')
  })
})
