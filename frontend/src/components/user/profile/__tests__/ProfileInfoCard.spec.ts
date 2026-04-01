import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'

const copyToClipboard = vi.fn()

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: { value: false },
    copyToClipboard
  })
}))

describe('ProfileInfoCard', () => {
  beforeEach(() => {
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)
  })

  it('renders binding address and copies it on click', async () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: {
          id: 1,
          email: 'user@example.com',
          username: 'tester',
          role: 'user',
          balance: 0,
          concurrency: 1,
          status: 'active',
          binding_address: '0x98B9eF5be5A14Bb890F68E6f3FF80213F8138b60',
          allowed_groups: null,
          created_at: '2026-04-01T00:00:00Z',
          updated_at: '2026-04-01T00:00:00Z'
        }
      }
    })

    expect(wrapper.text()).toContain('profile.bindingAddress')
    expect(wrapper.text()).toContain('0x98B9eF5be5A14Bb890F68E6f3FF80213F8138b60')

    const copyButton = wrapper.get('[data-testid="copy-binding-address"]')
    await copyButton.trigger('click')

    expect(copyToClipboard).toHaveBeenCalledWith(
      '0x98B9eF5be5A14Bb890F68E6f3FF80213F8138b60',
      'profile.bindingAddressCopied'
    )
  })

  it('does not render binding address row when empty', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: {
          id: 1,
          email: 'user@example.com',
          username: 'tester',
          role: 'user',
          balance: 0,
          concurrency: 1,
          status: 'active',
          binding_address: '',
          allowed_groups: null,
          created_at: '2026-04-01T00:00:00Z',
          updated_at: '2026-04-01T00:00:00Z'
        }
      }
    })

    expect(wrapper.text()).not.toContain('profile.bindingAddress')
    expect(wrapper.find('[data-testid="copy-binding-address"]').exists()).toBe(false)
  })
})
