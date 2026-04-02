import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const exchangeGoogleOAuth = vi.fn()
const completeGoogleOAuthRegistration = vi.fn()
const applyAuthResponse = vi.fn()
const showSuccess = vi.fn()
const showError = vi.fn()
const routerReplace = vi.fn()

vi.mock('@/api/auth', () => ({
  exchangeGoogleOAuth: (...args: any[]) => exchangeGoogleOAuth(...args),
  completeGoogleOAuthRegistration: (...args: any[]) => completeGoogleOAuthRegistration(...args),
  isGoogleOAuthPendingResponse: (response: any) =>
    response?.requires_invitation === true && typeof response?.pending_oauth_token === 'string',
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    applyAuthResponse,
  }),
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {
      redirect: '/workspace',
    },
  }),
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/utils/googleIdentity', () => ({
  loadGoogleIdentityScript: vi.fn().mockResolvedValue(undefined),
}))

import GoogleOAuthSection from '@/components/auth/GoogleOAuthSection.vue'

describe('GoogleOAuthSection', () => {
  let googleCallback: ((response: { credential?: string }) => void) | undefined

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    googleCallback = undefined
    ;(window as any).google = {
      accounts: {
        id: {
          initialize: vi.fn((config: { callback: (response: { credential?: string }) => void }) => {
            googleCallback = config.callback
          }),
          renderButton: vi.fn(),
          cancel: vi.fn(),
        },
      },
    }
  })

  it('收到 Google 凭证后完成登录并跳转', async () => {
    exchangeGoogleOAuth.mockResolvedValue({
      access_token: 'google-access-token',
      refresh_token: 'google-refresh-token',
      expires_in: 3600,
      token_type: 'Bearer',
      user: {
        id: 1,
        username: 'alice',
        email: 'alice@example.com',
        role: 'user',
        balance: 0,
        concurrency: 1,
        status: 'active',
        binding_address: '',
        allowed_groups: [],
        created_at: '2026-04-02T00:00:00Z',
        updated_at: '2026-04-02T00:00:00Z',
      },
    })

    const wrapper = mount(GoogleOAuthSection, {
      props: {
        clientId: 'google-client-id',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(typeof googleCallback).toBe('function')

    googleCallback?.({ credential: 'google-id-token' })
    await flushPromises()

    expect(exchangeGoogleOAuth).toHaveBeenCalledWith({
      google_token: 'google-id-token',
    })
    expect(applyAuthResponse).toHaveBeenCalledTimes(1)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(routerReplace).toHaveBeenCalledWith('/workspace')
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it('邀请码必填时展示补全表单并可继续完成注册', async () => {
    exchangeGoogleOAuth.mockResolvedValue({
      requires_invitation: true,
      pending_oauth_token: 'pending-google-token',
    })
    completeGoogleOAuthRegistration.mockResolvedValue({
      access_token: 'google-access-token',
      refresh_token: 'google-refresh-token',
      expires_in: 3600,
      token_type: 'Bearer',
      user: {
        id: 2,
        username: 'new-user',
        email: 'new-user@example.com',
        role: 'user',
        balance: 0,
        concurrency: 1,
        status: 'active',
        binding_address: '',
        allowed_groups: [],
        created_at: '2026-04-02T00:00:00Z',
        updated_at: '2026-04-02T00:00:00Z',
      },
    })

    const wrapper = mount(GoogleOAuthSection, {
      props: {
        clientId: 'google-client-id',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()
    googleCallback?.({ credential: 'google-id-token' })
    await flushPromises()

    const input = wrapper.get('input')
    await input.setValue('invite-123')
    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(completeGoogleOAuthRegistration).toHaveBeenCalledWith(
      'pending-google-token',
      'invite-123'
    )
    expect(applyAuthResponse).toHaveBeenCalledTimes(1)
    expect(routerReplace).toHaveBeenCalledWith('/workspace')
  })
})
