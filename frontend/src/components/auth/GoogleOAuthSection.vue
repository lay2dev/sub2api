<template>
  <div class="space-y-4">
    <div v-if="needsInvitation" class="space-y-4">
      <p class="text-sm text-gray-700 dark:text-gray-300">
        {{ t('auth.google.invitationRequired') }}
      </p>
      <div>
        <input
          v-model="invitationCode"
          type="text"
          class="input w-full"
          :placeholder="t('auth.invitationCodePlaceholder')"
          :disabled="isSubmitting"
          @keyup.enter="handleCompleteRegistration"
        />
      </div>
      <transition name="fade">
        <p v-if="invitationError" class="text-sm text-red-600 dark:text-red-400">
          {{ invitationError }}
        </p>
      </transition>
      <button
        type="button"
        class="btn btn-primary w-full"
        :disabled="isSubmitting || !invitationCode.trim()"
        @click="handleCompleteRegistration"
      >
        {{ isSubmitting ? t('auth.google.completing') : t('auth.google.completeRegistration') }}
      </button>
    </div>

    <div v-else class="space-y-3">
      <div
        v-if="disabled"
        class="h-[44px] rounded-xl border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-800"
      ></div>
      <div
        v-else
        ref="buttonHost"
        class="google-oauth-host min-h-[44px] overflow-hidden rounded-xl"
      ></div>
      <transition name="fade">
        <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">
          {{ errorMessage }}
        </p>
      </transition>
    </div>

    <div v-if="showDivider" class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.google.orContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { completeGoogleOAuthRegistration, exchangeGoogleOAuth, isGoogleOAuthPendingResponse } from '@/api/auth'
import { useAppStore, useAuthStore } from '@/stores'
import { loadGoogleIdentityScript } from '@/utils/googleIdentity'

const props = withDefaults(
  defineProps<{
    clientId: string
    disabled?: boolean
    showDivider?: boolean
  }>(),
  {
    disabled: false,
    showDivider: true,
  }
)

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const buttonHost = ref<HTMLDivElement | null>(null)
const errorMessage = ref('')
const invitationError = ref('')
const invitationCode = ref('')
const pendingOAuthToken = ref('')
const isSubmitting = ref(false)
const needsInvitation = ref(false)

function sanitizeRedirectPath(path: string | null | undefined): string {
  if (!path) return '/dashboard'
  if (!path.startsWith('/')) return '/dashboard'
  if (path.startsWith('//')) return '/dashboard'
  if (path.includes('://')) return '/dashboard'
  if (path.includes('\n') || path.includes('\r')) return '/dashboard'
  return path
}

function getRedirectTarget(): string {
  return sanitizeRedirectPath((route.query.redirect as string | undefined) || '/dashboard')
}

async function finishLogin(response: Awaited<ReturnType<typeof completeGoogleOAuthRegistration>>) {
  await authStore.applyAuthResponse(response)
  appStore.showSuccess(t('auth.loginSuccess'))
  await router.replace(getRedirectTarget())
}

async function handleGoogleCredential(response: { credential?: string }): Promise<void> {
  errorMessage.value = ''
  invitationError.value = ''

  const credential = response.credential?.trim()
  if (!credential) {
    errorMessage.value = t('auth.google.missingCredential')
    appStore.showError(errorMessage.value)
    return
  }

  try {
    const result = await exchangeGoogleOAuth({
      google_token: credential,
    })

    if (isGoogleOAuthPendingResponse(result)) {
      pendingOAuthToken.value = result.pending_oauth_token
      needsInvitation.value = true
      return
    }

    await finishLogin(result)
  } catch (error: any) {
    errorMessage.value = error?.message || t('auth.google.signInFailed')
    appStore.showError(errorMessage.value)
  }
}

async function renderGoogleButton(): Promise<void> {
  if (props.disabled || !props.clientId.trim()) {
    return
  }

  const host = buttonHost.value
  if (!host) return

  try {
    await loadGoogleIdentityScript()
    await nextTick()

    if (!window.google?.accounts?.id) {
      throw new Error(t('auth.google.loadFailed'))
    }

    host.innerHTML = ''
    window.google.accounts.id.initialize({
      client_id: props.clientId,
      callback: (response) => {
        void handleGoogleCredential(response)
      },
      ux_mode: 'popup',
    })
    window.google.accounts.id.renderButton(host, {
      type: 'standard',
      theme: 'outline',
      size: 'large',
      text: 'continue_with',
      shape: 'pill',
      width: host.clientWidth || 360,
      logo_alignment: 'left',
    })
  } catch (error: any) {
    errorMessage.value = error?.message || t('auth.google.loadFailed')
  }
}

async function handleCompleteRegistration(): Promise<void> {
  invitationError.value = ''
  if (!pendingOAuthToken.value || !invitationCode.value.trim()) {
    return
  }

  isSubmitting.value = true
  try {
    const response = await completeGoogleOAuthRegistration(
      pendingOAuthToken.value,
      invitationCode.value.trim()
    )
    await finishLogin(response)
  } catch (error: any) {
    invitationError.value = error?.message || t('auth.google.completeRegistrationFailed')
  } finally {
    isSubmitting.value = false
  }
}

watch(
  [buttonHost, () => props.clientId, () => props.disabled],
  () => {
    void renderGoogleButton()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  window.google?.accounts?.id?.cancel?.()
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
