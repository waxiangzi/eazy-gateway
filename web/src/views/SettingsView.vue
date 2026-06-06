<template>
  <div class="settings-view">
    <h1 class="settings-title">{{ t('settings.title') }}</h1>

    <!-- 系统配置：轻量行内编辑 -->
    <section class="setting-section">
      <h2 class="section-heading">{{ t('settings.system.title') }}</h2>

      <div class="inline-setting">
        <label for="app-name">{{ t('settings.appNameLabel') }}</label>
        <input
          id="app-name"
          v-model="appName"
          type="text"
          :placeholder="t('settings.appNamePlaceholder')"
          @blur="handleAppNameBlur"
          @keydown.enter="handleAppNameBlur"
        />
        <span v-if="appNameLoading" class="save-hint saving">{{ t('settings.system.saving') }}</span>
        <span v-else-if="appNameSuccess" class="save-hint saved">
          {{ appNameSaved ? '✓ ' + t('settings.system.saved') : '' }}
        </span>
        <span v-else-if="appNameError" class="save-hint error">{{ appNameError }}</span>
      </div>

      <div class="inline-setting">
        <label for="trend-hours">{{ t('settings.trafficTrendHoursLabel') }}</label>
        <select
          id="trend-hours"
          v-model="trafficTrendHours"
          @change="handleTrendHoursChange"
        >
          <option v-for="h in [1, 2, 3, 6, 12, 24]" :key="h" :value="h">
            {{ h }} {{ t('settings.trafficTrendHoursUnit') }}
          </option>
        </select>
        <span v-if="trendHoursLoading" class="save-hint saving">{{ t('settings.system.saving') }}</span>
        <span v-else-if="trendHoursSuccess" class="save-hint saved">
          ✓ {{ t('settings.system.saved') }}
        </span>
        <span v-else-if="trendHoursError" class="save-hint error">{{ trendHoursError }}</span>
      </div>
    </section>

    <hr class="section-divider" />

    <!-- 安全设置：完整表单 -->
    <section class="setting-section">
      <h2 class="section-heading">{{ t('settings.security.title') }}</h2>

      <form class="settings-form" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="old-password">{{ t('settings.oldPassword') }}</label>
          <input
            id="old-password"
            v-model="oldPassword"
            type="password"
            :placeholder="t('settings.oldPasswordPlaceholder')"
            required
            autocomplete="current-password"
          />
        </div>

        <div class="form-group">
          <label for="new-password">{{ t('settings.newPassword') }}</label>
          <input
            id="new-password"
            v-model="newPassword"
            type="password"
            :placeholder="t('settings.newPasswordPlaceholder')"
            required
            autocomplete="new-password"
          />
          <p v-if="newPasswordError" class="field-error">{{ newPasswordError }}</p>
        </div>

        <div class="form-group">
          <label for="confirm-password">{{ t('settings.confirmPassword') }}</label>
          <input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            :placeholder="t('settings.confirmPasswordPlaceholder')"
            required
            autocomplete="new-password"
          />
          <p v-if="confirmError" class="field-error">{{ confirmError }}</p>
        </div>

        <p v-if="serverError" class="error-message">{{ serverError }}</p>
        <p v-if="successMessage" class="success-message">{{ successMessage }}</p>

        <button type="submit" class="submit-button" :disabled="isLoading">
          {{ isLoading ? t('settings.updating') : t('settings.update') }}
        </button>
      </form>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth.js'
import { useSettingsStore } from '../stores/settings.js'
import client from '../api/client.js'

const { t } = useI18n()
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const serverError = ref('')
const successMessage = ref('')

const appName = ref('')
const appNameLoading = ref(false)
const appNameError = ref('')
const appNameSuccess = ref(false)
const appNameSaved = ref(false)

const trafficTrendHours = ref(1)
const trendHoursLoading = ref(false)
const trendHoursSuccess = ref(false)
const trendHoursError = ref('')

const auth = useAuthStore()
const settings = useSettingsStore()
const router = useRouter()

onMounted(() => {
  appName.value = settings.appName || t('nav.brand')
  trafficTrendHours.value = settings.trafficTrendHours || 1
})

const newPasswordError = computed(() => {
  if (newPassword.value && newPassword.value.length < 8) {
    return t('settings.errors.passwordTooShort')
  }
  return ''
})

const confirmError = computed(() => {
  if (confirmPassword.value && confirmPassword.value !== newPassword.value) {
    return t('settings.errors.passwordMismatch')
  }
  return ''
})

async function handleSubmit() {
  serverError.value = ''
  successMessage.value = ''

  if (!oldPassword.value || !newPassword.value || !confirmPassword.value) {
    serverError.value = t('settings.errors.required')
    return
  }

  if (newPassword.value.length < 8) {
    serverError.value = t('settings.errors.tooShort')
    return
  }

  if (newPassword.value !== confirmPassword.value) {
    serverError.value = t('settings.errors.mismatch')
    return
  }

  isLoading.value = true

  try {
    await client.post('/admin/change-password', {
      oldPassword: oldPassword.value,
      newPassword: newPassword.value,
    })

    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    successMessage.value = t('settings.success')

    await auth.logout()
    router.push('/login')
  } catch (err) {
    if (!err.response) {
      serverError.value = t('settings.errors.network')
    } else if (err.response.status === 403) {
      serverError.value = t('settings.errors.wrongCurrent')
    } else if (err.response.status === 400) {
      serverError.value = err.response.data?.message || t('settings.errors.required')
    } else if (err.response.status === 500) {
      serverError.value = t('settings.errors.server')
    } else {
      serverError.value = err.response.data?.message || t('settings.errors.unexpected')
    }
  } finally {
    isLoading.value = false
  }
}

let debounceTimer = null
let isUpdatingAppName = false

async function handleAppNameBlur() {
  const name = appName.value.trim()
  if (!name) {
    appNameError.value = t('settings.errors.appNameRequired')
    appNameSuccess.value = false
    return
  }

  if (isUpdatingAppName) {
    return
  }

  appNameError.value = ''
  appNameLoading.value = true
  appNameSuccess.value = false

  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(async () => {
    isUpdatingAppName = true
    try {
      const ok = await settings.updateAppName(name)
      if (ok) {
        appNameSuccess.value = true
        appNameSaved.value = true
        setTimeout(() => { appNameSaved.value = false }, 3000)
      } else {
        appNameError.value = settings.error || t('settings.errors.unexpected')
      }
    } catch (err) {
      appNameError.value = err.response?.data?.message || t('settings.errors.unexpected')
    } finally {
      appNameLoading.value = false
      isUpdatingAppName = false
    }
  }, 300)
}

async function handleTrendHoursChange() {
  trendHoursLoading.value = true
  trendHoursSuccess.value = false
  trendHoursError.value = ''
  try {
    const ok = await settings.updateTrafficTrendHours(trafficTrendHours.value)
    if (ok) {
      trendHoursSuccess.value = true
      setTimeout(() => { trendHoursSuccess.value = false }, 3000)
    } else {
      trendHoursError.value = settings.error || t('settings.errors.unexpected')
    }
  } catch (err) {
    trendHoursError.value = err.response?.data?.message || t('settings.errors.unexpected')
  } finally {
    trendHoursLoading.value = false
  }
}
</script>

<style scoped>
.settings-view {
  max-width: 64rem;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.settings-title {
  margin: 0 0 1.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  color: #0f172a;
}

.section-heading {
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #64748b;
  margin: 0 0 1rem;
}

.section-divider {
  border: none;
  border-top: 1px solid #e2e8f0;
  margin: 2rem 0;
}

/* 系统配置：轻量行内编辑 */
.inline-setting {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  max-width: 28rem;
}

.inline-setting label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #334155;
  min-width: 5rem;
}

.inline-setting input {
  flex: 1;
  min-width: 12rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-size: 1rem;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
}

.inline-setting input:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}

.save-hint {
  font-size: 0.875rem;
  white-space: nowrap;
}

.save-hint.saving {
  color: #64748b;
}

.save-hint.saved {
  color: #16a34a;
}

.save-hint.error {
  color: #dc2626;
}

/* 安全设置：完整表单 */
.settings-form {
  max-width: 28rem;
  padding: 1.5rem;
  background: #ffffff;
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  border-left: 4px solid #f59e0b;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #334155;
}

.form-group input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-size: 1rem;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
}

.form-group input:focus {
  outline: none;
  border-color: #f59e0b;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.15);
}

.field-error {
  margin: 0.25rem 0 0;
  color: #dc2626;
  font-size: 0.875rem;
}

.error-message {
  margin: 0 0 1rem;
  color: #dc2626;
  font-size: 0.875rem;
  text-align: center;
}

.success-message {
  margin: 0 0 1rem;
  color: #16a34a;
  font-size: 0.875rem;
  text-align: center;
}

.submit-button {
  width: 100%;
  padding: 0.625rem;
  border: none;
  border-radius: 0.375rem;
  background: #0f172a;
  color: #fff;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
}

.submit-button:hover:not(:disabled) {
  background: #1e293b;
}

.submit-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .inline-setting {
    flex-direction: column;
    align-items: flex-start;
  }
  .inline-setting input {
    width: 100%;
  }
}
</style>
