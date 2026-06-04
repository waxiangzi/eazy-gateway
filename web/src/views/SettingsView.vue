<template>
  <div class="settings-view">
    <h1 class="settings-title">Settings</h1>

    <form class="settings-form" @submit.prevent="handleSubmit">
      <h2 class="form-section-title">Change Password</h2>

      <div class="form-group">
        <label for="old-password">Old Password</label>
        <input
          id="old-password"
          v-model="oldPassword"
          type="password"
          placeholder="Enter current password"
          required
          autocomplete="current-password"
        />
      </div>

      <div class="form-group">
        <label for="new-password">New Password</label>
        <input
          id="new-password"
          v-model="newPassword"
          type="password"
          placeholder="Enter new password"
          required
          autocomplete="new-password"
        />
        <p v-if="newPasswordError" class="field-error">{{ newPasswordError }}</p>
      </div>

      <div class="form-group">
        <label for="confirm-password">Confirm New Password</label>
        <input
          id="confirm-password"
          v-model="confirmPassword"
          type="password"
          placeholder="Confirm new password"
          required
          autocomplete="new-password"
        />
        <p v-if="confirmError" class="field-error">{{ confirmError }}</p>
      </div>

      <p v-if="serverError" class="error-message">{{ serverError }}</p>
      <p v-if="successMessage" class="success-message">{{ successMessage }}</p>

      <button type="submit" class="submit-button" :disabled="isLoading">
        {{ isLoading ? 'Updating…' : 'Update Password' }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import client from '../api/client.js'

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const serverError = ref('')
const successMessage = ref('')

const auth = useAuthStore()
const router = useRouter()

const newPasswordError = computed(() => {
  if (newPassword.value && newPassword.value.length < 8) {
    return 'Password must be at least 8 characters'
  }
  return ''
})

const confirmError = computed(() => {
  if (confirmPassword.value && confirmPassword.value !== newPassword.value) {
    return 'Passwords do not match'
  }
  return ''
})

async function handleSubmit() {
  serverError.value = ''
  successMessage.value = ''

  if (!oldPassword.value || !newPassword.value || !confirmPassword.value) {
    serverError.value = 'All fields are required'
    return
  }

  if (newPassword.value.length < 8) {
    serverError.value = 'New password must be at least 8 characters'
    return
  }

  if (newPassword.value !== confirmPassword.value) {
    serverError.value = 'New password and confirmation do not match'
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
    successMessage.value = 'Password updated successfully. Logging out…'

    await auth.logout()
    router.push('/login')
  } catch (err) {
    if (!err.response) {
      serverError.value = 'Network error, please try again'
    } else if (err.response.status === 403) {
      serverError.value = 'Current password is incorrect'
    } else if (err.response.status === 400) {
      serverError.value = err.response.data?.message || 'Invalid request'
    } else if (err.response.status === 500) {
      serverError.value = 'Server error, please try again'
    } else {
      serverError.value = err.response.data?.message || 'An unexpected error occurred'
    }
  } finally {
    isLoading.value = false
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

.settings-form {
  max-width: 28rem;
  padding: 1.5rem;
  background: #ffffff;
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.form-section-title {
  margin: 0 0 1.25rem;
  font-size: 1.125rem;
  font-weight: 600;
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
</style>
