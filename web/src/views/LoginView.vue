<template>
  <div class="login-view">
    <form class="login-form" @submit.prevent="handleSubmit">
      <h1 class="login-title">{{ displayTitle }}</h1>
      <div class="form-group">
        <label for="password">{{ t('login.passwordLabel') }}</label>
        <input
          id="password"
          v-model="password"
          type="password"
          :placeholder="t('login.passwordPlaceholder')"
          required
          autocomplete="current-password"
        />
      </div>
      <p v-if="error" class="error-message">{{ error }}</p>
      <button type="submit" class="login-button" :disabled="auth.isLoading">
        {{ auth.isLoading ? t('login.loggingIn') : t('login.submit') }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth.js'
import { useSettingsStore } from '../stores/settings.js'

const { t } = useI18n()
const password = ref('')
const auth = useAuthStore()
const settings = useSettingsStore()
const router = useRouter()

const displayTitle = computed(() => settings.appName || t('login.title'))

// surface store error reactively
const error = ref(auth.error)
watch(() => auth.error, (val) => {
  error.value = val
})

async function handleSubmit() {
  const ok = await auth.login(password.value)
  if (ok) {
    router.push('/')
  }
}
</script>

<style scoped>
.login-view {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-page);
}

.login-form {
  width: 100%;
  max-width: 24rem;
  padding: 2rem;
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.75rem;
  border: 1px solid rgba(var(--accent-rgb), 0.18);
  box-shadow: 0 0 20px rgba(var(--accent-rgb), 0.08), var(--shadow-md);
  backdrop-filter: blur(12px) saturate(140%);
  -webkit-backdrop-filter: blur(12px) saturate(140%);
}

.login-title {
  margin: 0 0 1.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  text-align: center;
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(var(--accent-rgb), 0.25);
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-secondary);
}

.form-group input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: var(--text-primary);
  background: rgba(var(--page-rgb), 0.8);
  box-sizing: border-box;
  font-family: inherit;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-group input:focus {
  outline: none;
  border-color: rgba(var(--accent-rgb), 0.55);
  box-shadow: 0 0 0 3px rgba(var(--accent-rgb), 0.15), 0 0 8px rgba(var(--accent-rgb), 0.2);
}

.error-message {
  margin: 0 0 1rem;
  color: var(--error);
  font-size: 0.875rem;
  text-align: center;
}

.login-button {
  width: 100%;
  padding: 0.625rem;
  border: none;
  border-radius: 0.375rem;
  background: linear-gradient(135deg, rgba(var(--accent-rgb),0.9), rgba(var(--accent-strong-rgb),0.9));
  color: var(--accent-fg);
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid rgba(var(--accent-rgb), 0.35);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.15);
}

.login-button:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),1), rgba(var(--accent-strong-rgb),1));
  box-shadow: 0 0 16px rgba(var(--accent-rgb), 0.3);
  transform: translateY(-1px);
}

.login-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
