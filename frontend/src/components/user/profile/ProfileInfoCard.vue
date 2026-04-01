<template>
  <div class="card overflow-hidden">
    <div
      class="border-b border-gray-100 bg-gradient-to-r from-primary-500/10 to-primary-600/5 px-6 py-5 dark:border-dark-700 dark:from-primary-500/20 dark:to-primary-600/10"
    >
      <div class="flex items-center gap-4">
        <!-- Avatar -->
        <div
          class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 text-2xl font-bold text-white shadow-lg shadow-primary-500/20"
        >
          {{ user?.email?.charAt(0).toUpperCase() || 'U' }}
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
            {{ user?.email }}
          </h2>
          <div class="mt-1 flex items-center gap-2">
            <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
              {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
            </span>
            <span
              :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']"
            >
              {{ user?.status }}
            </span>
          </div>
        </div>
      </div>
    </div>
    <div class="px-6 py-4">
      <div class="space-y-3">
        <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-400">
          <Icon name="mail" size="sm" class="text-gray-400 dark:text-gray-500" />
          <span class="truncate">{{ user?.email }}</span>
        </div>
        <div
          v-if="user?.username"
          class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-400"
        >
          <Icon name="user" size="sm" class="text-gray-400 dark:text-gray-500" />
          <span class="truncate">{{ user.username }}</span>
        </div>
        <div
          v-if="user?.binding_address"
          class="flex items-start gap-3 text-sm text-gray-600 dark:text-gray-400"
        >
          <Icon name="link" size="sm" class="mt-0.5 text-gray-400 dark:text-gray-500" />
          <div class="min-w-0 flex-1">
            <div class="text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
              {{ t('profile.bindingAddress') }}
            </div>
            <div class="mt-1 flex items-center gap-2">
              <span class="min-w-0 break-all font-mono text-xs text-gray-700 dark:text-gray-300">
                {{ user.binding_address }}
              </span>
              <button
                type="button"
                class="inline-flex flex-shrink-0 items-center rounded-md p-1 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('profile.copyBindingAddress')"
                data-testid="copy-binding-address"
                @click="handleCopyBindingAddress"
              >
                <Icon name="copy" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { User } from '@/types'

const props = defineProps<{
  user: User | null
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

function handleCopyBindingAddress() {
  if (!props.user?.binding_address) return
  void copyToClipboard(props.user.binding_address, t('profile.bindingAddressCopied'))
}
</script>
