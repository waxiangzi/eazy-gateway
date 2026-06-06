import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '../api/client.js'

export const useTunnelsStore = defineStore('tunnels', () => {
  const list = ref([])
  const loading = ref(false)
  const error = ref(null)
  // Map tunnel id -> 'connected' | 'connecting' | 'disconnected' | 'error'
  const statuses = ref({})
  // Map tunnel id -> { bytesIn: number, bytesOut: number }
  const traffic = ref({})

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
    delete traffic.value[id]
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
        statuses.value[t.id] = t.status || 'disconnected'
        if (t.traffic) {
          traffic.value[t.id] = t.traffic
        }
      }
      return data.hosts || {}
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to fetch connections'
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
        'Failed to start connection'
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
        'Failed to stop connection'
      return false
    }
  }

  async function createTunnel(data) {
    error.value = null
    try {
      const res = await client.post('/tunnels', data)
      return res.data
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to create connection'
      throw err
    }
  }

  async function updateTunnel(id, data) {
    error.value = null
    try {
      const res = await client.put(`/tunnels/${id}`, data)
      return res.data
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to update connection'
      throw err
    }
  }

  async function getTunnel(id) {
    const res = await client.get(`/tunnels/${id}`)
    return res.data || {}
  }

  async function fetchTrafficTrend(id, hours) {
    try {
      const res = await client.get(`/tunnels/${id}/traffic/trend`, {
        params: { hours },
      })
      return res.data?.points || []
    } catch (err) {
      return []
    }
  }

  return {
    list,
    loading,
    error,
    statuses,
    traffic,
    setTunnels,
    addTunnel,
    removeTunnel,
    fetchTunnels,
    startTunnel,
    stopTunnel,
    createTunnel,
    updateTunnel,
    getTunnel,
    fetchTrafficTrend,
  }
})
