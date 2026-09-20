import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '../api/client.js'

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(localStorage.getItem('eazy-gateway-auth') === 'true')
  const error = ref(null)
  const isLoading = ref(false)
  const initialized = ref(false)

  async function login(password) {
    error.value = null
    isLoading.value = true
    try {
      await client.post('/login', { password })
      isLoggedIn.value = true
      localStorage.setItem('eazy-gateway-auth', 'true')
      return true
    } catch (err) {
      if (!err.response) {
        error.value = 'Network error'
      } else {
        error.value = err.response.data?.message || 'Login failed'
      }
      isLoggedIn.value = false
      localStorage.removeItem('eazy-gateway-auth')
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function logout() {
    try {
      await client.post('/logout')
    } catch {
      // ignore network errors on logout
    } finally {
      isLoggedIn.value = false
      localStorage.removeItem('eazy-gateway-auth')
    }
  }

  async function checkAuth() {
    try {
      await client.get('/me')
      isLoggedIn.value = true
      localStorage.setItem('eazy-gateway-auth', 'true')
      return true
    } catch {
      isLoggedIn.value = false
      localStorage.removeItem('eazy-gateway-auth')
      return false
    } finally {
      initialized.value = true
    }
  }

  return { isLoggedIn, error, isLoading, initialized, login, logout, checkAuth }
})
