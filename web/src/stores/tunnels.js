import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '../api/client.js'

export const useTunnelsStore = defineStore('tunnels', () => {
  const list = ref([])
  const loading = ref(false)
  const error = ref(null)
  // Map tunnel id -> 'connected' | 'connecting' | 'disconnected' | 'error'
  const statuses = ref({})

  function setTunnels(tunnels) {
    list.value = tunnels
  }

  function addTunnel(tunnel) {
    list.value.push(tunnel)
  }

  function removeTunnel(id) {
    const idx = list.value.findIndex((t) => t.id === id)
    if (idx !== -1) {
      list.value.splice(idx, 1)
    }
    delete statuses.value[id]
  }

  async function fetchTunnels() {
    loading.value = true
    error.value = null
    try {
      const res = await client.get('/tunnels')
      const data = res.data || {}
      const tunnels = data.items || []
      list.value = tunnels
      for (const t of list.value) {
        if (statuses.value[t.id] === undefined) {
          statuses.value[t.id] = 'disconnected'
        }
      }
      return data.hosts || {}
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to fetch tunnels'
      return {}
    } finally {
      loading.value = false
    }
  }

  async function startTunnel(id) {
    error.value = null
    statuses.value[id] = 'connecting'
    try {
      await client.post(`/tunnels/${id}/start`)
      statuses.value[id] = 'connected'
      return true
    } catch (err) {
      statuses.value[id] = 'error'
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to start tunnel'
      return false
    }
  }

  async function stopTunnel(id) {
    error.value = null
    try {
      await client.post(`/tunnels/${id}/stop`)
      statuses.value[id] = 'disconnected'
      return true
    } catch (err) {
      statuses.value[id] = 'error'
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to stop tunnel'
      return false
    }
  }

  async function createTunnel(data) {
    const res = await client.post('/tunnels', data)
    return res.data
  }

  async function updateTunnel(id, data) {
    const res = await client.put(`/tunnels/${id}`, data)
    return res.data
  }

  async function getTunnel(id) {
    const res = await client.get(`/tunnels/${id}`)
    return res.data || {}
  }

  return {
    list,
    loading,
    error,
    statuses,
    setTunnels,
    addTunnel,
    removeTunnel,
    fetchTunnels,
    startTunnel,
    stopTunnel,
    createTunnel,
    updateTunnel,
    getTunnel,
  }
})
