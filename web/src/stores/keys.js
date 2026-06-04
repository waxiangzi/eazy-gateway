import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useKeysStore = defineStore('keys', () => {
  const list = ref([])

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

  return { list, setKeys, addKey, removeKey }
})
