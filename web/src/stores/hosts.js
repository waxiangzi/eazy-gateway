import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '../api/client.js'

export const useHostsStore = defineStore('hosts', () => {
  const list = ref([])
  const loading = ref(false)
  const error = ref(null)

  function setHosts(hosts) {
    list.value = hosts
  }

  function addHost(host) {
    list.value.push(host)
  }

  function removeHost(id) {
    const idx = list.value.findIndex((h) => h.id === id)
    if (idx !== -1) {
      list.value.splice(idx, 1)
    }
  }

  async function fetchHosts() {
    loading.value = true
    error.value = null
    try {
      const res = await client.get('/hosts')
      list.value = res.data || []
    } catch (err) {
      error.value =
        err.response?.data?.message ||
        err.response?.data?.error ||
        'Failed to fetch hosts'
    } finally {
      loading.value = false
    }
  }

  async function createHost(data) {
    const res = await client.post('/hosts', data)
    return res.data
  }

  async function updateHost(id, data) {
    const res = await client.put(`/hosts/${id}`, data)
    return res.data
  }

  async function getHost(id) {
    const res = await client.get(`/hosts/${id}`)
    return res.data
  }

  async function deleteHost(id) {
    await client.delete(`/hosts/${id}`)
  }

  async function testHost(data) {
    const res = await client.post('/hosts/test', data)
    return res.data
  }

  return {
    list,
    loading,
    error,
    setHosts,
    addHost,
    removeHost,
    fetchHosts,
    createHost,
    updateHost,
    getHost,
    deleteHost,
    testHost,
  }
})
