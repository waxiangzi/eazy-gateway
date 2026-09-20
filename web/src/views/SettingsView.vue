<template>
  <div class="settings-view">
    <h1 class="settings-title">{{ t('settings.title') }}</h1>

    <!-- 系统配置：轻量行内编辑 -->
    <section class="setting-section">
      <h2 class="section-heading">{{ t('settings.system.title') }}</h2>

      <div class="inline-settings-card">
        <div class="inline-setting">
          <label for="app-name">{{ t('settings.appNameLabel') }}</label>
          <div class="inline-field">
            <input
              id="app-name"
              v-model="appName"
              type="text"
              :placeholder="t('settings.appNamePlaceholder')"
              @blur="handleAppNameBlur"
              @keydown.enter="handleAppNameBlur"
            />
            <span v-if="appNameLoading" class="save-hint saving">{{ t('settings.system.saving') }}</span>
            <span v-else-if="appNameSuccess && appNameSaved" class="save-hint saved">
              ✓ {{ t('settings.system.saved') }}
            </span>
            <span v-else-if="appNameError" class="save-hint error">{{ appNameError }}</span>
          </div>
        </div>

        <div class="inline-setting">
          <label for="trend-hours">{{ t('settings.trafficTrendHoursLabel') }}</label>
          <div class="inline-field">
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
        </div>

        <div class="inline-setting">
          <label>{{ t('settings.versionLabel') }}</label>
          <div class="inline-field">
            <span class="version-value">{{ appVersion || '—' }}</span>
          </div>
        </div>
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

// Reported by GET /api/settings; shows which release this build is.
const appVersion = computed(() => settings.version || '')

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
  color: #e2e8f0;
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(56, 189, 248, 0.2);
}

.section-heading {
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #64748b;
  margin: 0 0 1rem;
}

.section-divider {
  border: none;
  border-top: 1px solid rgba(56, 189, 248, 0.1);
  margin: 2rem 0;
}

/* 系统配置：轻量行内编辑 */
.inline-settings-card {
  background: rgba(16, 24, 48, 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(56, 189, 248, 0.15);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  padding: 1rem 1.25rem;
  max-width: 32rem;
}

.inline-setting {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 0.75rem 0;
}

.inline-setting + .inline-setting {
  border-top: 1px solid rgba(56, 189, 248, 0.08);
}

.inline-setting label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #94a3b8;
  width: 7rem;
  flex-shrink: 0;
  padding-top: 0.5rem;
  line-height: 1.5;
}

.inline-field {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
}

.inline-field input,
.inline-field select {
  flex: 1;
  min-width: 0;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(56, 189, 248, 0.2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: #e2e8f0;
  background: rgba(10, 14, 26, 0.8);
  box-sizing: border-box;
  font-family: inherit;
  transition: border-color 0.2s, box-shadow 0.2s;
  height: 2.25rem;
}

.inline-field input:focus,
.inline-field select:focus {
  outline: none;
  border-color: rgba(56, 189, 248, 0.55);
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.15), 0 0 8px rgba(56, 189, 248, 0.2);
}

.inline-field select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%2394a3b8' d='M6 8L1 3h10z'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 0.75rem center;
  padding-right: 2rem;
  cursor: pointer;
}

.version-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 1rem;
  color: #cbd5e1;
}

.save-hint {
  font-size: 0.875rem;
  white-space: nowrap;
  flex-shrink: 0;
}

.save-hint.saving {
  color: #94a3b8;
}

.save-hint.saved {
  color: #34d399;
}

.save-hint.error {
  color: #f87171;
}

/* 安全设置：完整表单 */
.settings-form {
  max-width: 28rem;
  padding: 1.5rem;
  background: rgba(16, 24, 48, 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(251, 191, 36, 0.2);
  box-shadow: 0 0 12px rgba(251, 191, 36, 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border-left: 4px solid rgba(251, 191, 36, 0.5);
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #94a3b8;
}

.form-group input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(56, 189, 248, 0.2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: #e2e8f0;
  background: rgba(10, 14, 26, 0.8);
  box-sizing: border-box;
  font-family: inherit;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-group input:focus {
  outline: none;
  border-color: rgba(251, 191, 36, 0.55);
  box-shadow: 0 0 0 3px rgba(251, 191, 36, 0.12), 0 0 8px rgba(251, 191, 36, 0.15);
}

.field-error {
  margin: 0.25rem 0 0;
  color: #f87171;
  font-size: 0.875rem;
}

.error-message {
  margin: 0 0 1rem;
  color: #f87171;
  font-size: 0.875rem;
  text-align: center;
}

.success-message {
  margin: 0 0 1rem;
  color: #34d399;
  font-size: 0.875rem;
  text-align: center;
}

.submit-button {
  width: 100%;
  padding: 0.625rem;
  border: none;
  border-radius: 0.375rem;
  background: linear-gradient(135deg, rgba(56,189,248,0.9), rgba(14,165,233,0.9));
  color: #fff;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid rgba(56, 189, 248, 0.35);
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.15);
}

.submit-button:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(56,189,248,1), rgba(14,165,233,1));
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.3);
  transform: translateY(-1px);
}

.submit-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .inline-setting {
    flex-direction: column;
    gap: 0.375rem;
  }
  .inline-setting label {
    width: auto;
    padding-top: 0;
  }
  .inline-field {
    width: 100%;
    flex-wrap: wrap;
  }
  .inline-field input,
  .inline-field select {
    width: 100%;
    flex: none;
  }
}
</style>
