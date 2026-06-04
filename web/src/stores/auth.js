import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '../api/client.js'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || null)
  const error = ref(null)
  const isLoading = ref(false)

  const isLoggedIn = computed(() => !!token.value)

  async function login(password) {
    error.value = null
    isLoading.value = true
    try {
      const res = await client.post('/login', { password })
      token.value = res.data.token
      localStorage.setItem('token', res.data.token)
      return true
    } catch (err) {
      if (!err.response) {
        error.value = 'Network error'
      } else {
        error.value = err.response.data?.message || 'Login failed'
      }
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
      token.value = null
      localStorage.removeItem('token')
    }
  }

  async function checkAuth() {
    if (!token.value) return false
    try {
      await client.get('/me')
      return true
    } catch {
      token.value = null
      localStorage.removeItem('token')
      return false
    }
  }

  return { token, isLoggedIn, error, isLoading, login, logout, checkAuth }
})
