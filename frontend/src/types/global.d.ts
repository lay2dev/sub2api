import type { PublicSettings } from '@/types'

declare global {
  interface GoogleCredentialResponse {
    credential?: string
    select_by?: string
  }

  interface GoogleAccountsID {
    initialize(config: {
      client_id: string
      callback: (response: GoogleCredentialResponse) => void
      ux_mode?: 'popup' | 'redirect'
    }): void
    renderButton(
      parent: HTMLElement,
      options: {
        type?: string
        theme?: string
        size?: string
        text?: string
        shape?: string
        width?: number
        logo_alignment?: string
      }
    ): void
    cancel?: () => void
  }

  interface Window {
    __APP_CONFIG__?: PublicSettings
    google?: {
      accounts?: {
        id?: GoogleAccountsID
      }
    }
  }
}

export {}
