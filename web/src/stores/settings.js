import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '../api/client.js'

export const useSettingsStore = defineStore('settings', () => {
  const appName = ref('')
  const trafficTrendHours = ref(1)
  const loading = ref(false)
  const error = ref(null)

  async function fetchSettings() {
    loading.value = true
    error.value = null
    try {
      const res = await client.get('/settings')
      appName.value = res.data?.appName || ''
      const h = parseInt(res.data?.trafficTrendHours, 10)
      trafficTrendHours.value = (!isNaN(h) && h > 0) ? h : 1
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to load settings'
    } finally {
      loading.value = false
    }
  }

  async function updateAppName(name) {
    error.value = null
    try {
      await client.put('/settings', { appName: name })
      appName.value = name
      return true
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to update app name'
      return false
    }
  }

  async function updateTrafficTrendHours(hours) {
    error.value = null
    try {
      await client.put('/settings', { trafficTrendHours: hours })
      trafficTrendHours.value = hours
      return true
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to update traffic trend hours'
      return false
    }
  }

  return {
    appName,
    trafficTrendHours,
    loading,
    error,
    fetchSettings,
    updateAppName,
    updateTrafficTrendHours,
  }
})
