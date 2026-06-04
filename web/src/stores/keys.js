import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '../api/client.js'

export const useKeysStore = defineStore('keys', () => {
  const list = ref([])
  const isLoading = ref(false)
  const error = ref(null)
  const isUploading = ref(false)
  const uploadError = ref(null)
  const isDeleting = ref(false)
  const deleteError = ref(null)

  const hasKeys = computed(() => list.value.length > 0)

  function setKeys(keys) {
    list.value = keys
  }

  function addKey(key) {
    list.value.push(key)
  }

  function removeKey(id) {
    const idx = list.value.findIndex((k) => k.id === id)
    if (idx !== -1) {
      list.value.splice(idx, 1)
    }
  }

  async function fetchKeys() {
    isLoading.value = true
    error.value = null
    try {
      const res = await client.get('/keys')
      setKeys(res.data || [])
      return true
    } catch (err) {
      error.value = err.response?.data?.message || 'Failed to load keys'
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function createKey(data) {
    isUploading.value = true
    uploadError.value = null
    try {
      const res = await client.post('/keys', data)
      addKey(res.data)
      return true
    } catch (err) {
      if (err.response?.status === 409) {
        uploadError.value = 'Key is in use by a tunnel'
      } else {
        uploadError.value = err.response?.data?.message || 'Failed to create key'
      }
      return false
    } finally {
      isUploading.value = false
    }
  }

  async function deleteKey(id) {
    isDeleting.value = true
    deleteError.value = null
    try {
      await client.delete(`/keys/${id}`)
      removeKey(id)
      return true
    } catch (err) {
      if (err.response?.status === 409) {
        deleteError.value = 'Key is in use by a tunnel'
      } else {
        deleteError.value = err.response?.data?.message || 'Failed to delete key'
      }
      return false
    } finally {
      isDeleting.value = false
    }
  }

  return {
    list,
    isLoading,
    error,
    isUploading,
    uploadError,
    isDeleting,
    deleteError,
    hasKeys,
    setKeys,
    addKey,
    removeKey,
    fetchKeys,
    createKey,
    deleteKey,
  }
})
