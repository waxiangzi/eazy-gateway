import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useTunnelsStore = defineStore('tunnels', () => {
  const list = ref([])

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
  }

  return { list, setTunnels, addTunnel, removeTunnel }
})
