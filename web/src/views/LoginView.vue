<template>
  <div class="login-view">
    <form class="login-form" @submit.prevent="handleSubmit">
      <h1 class="login-title">{{ t('login.title') }}</h1>
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
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth.js'

const { t } = useI18n()
const password = ref('')
const auth = useAuthStore()
const router = useRouter()

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
  background: #f1f5f9;
}

.login-form {
  width: 100%;
  max-width: 24rem;
  padding: 2rem;
  background: #ffffff;
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.login-title {
  margin: 0 0 1.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  text-align: center;
  color: #0f172a;
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
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}

.error-message {
  margin: 0 0 1rem;
  color: #dc2626;
  font-size: 0.875rem;
  text-align: center;
}

.login-button {
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

.login-button:hover:not(:disabled) {
  background: #1e293b;
}

.login-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
