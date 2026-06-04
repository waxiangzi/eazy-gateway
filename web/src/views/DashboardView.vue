<template>
  <div class="dashboard-view">
    <div class="dashboard-header">
      <h1>Tunnels</h1>
      <RouterLink to="/tunnels" class="btn-primary">Create Tunnel</RouterLink>
    </div>

    <div v-if="store.loading && store.list.length === 0" class="state-message">
      Loading tunnels…
    </div>
    <div v-else-if="store.error" class="state-message error">{{ store.error }}</div>
    <div v-else-if="store.list.length === 0" class="empty-state">
      <p>No tunnels configured yet.</p>
      <RouterLink to="/tunnels" class="btn-primary">Create your first tunnel</RouterLink>
    </div>
    <div v-else class="tunnel-list">
      <div
        v-for="tunnel in store.list"
        :key="tunnel.id"
        class="tunnel-card"
      >
        <div class="tunnel-info">
          <div class="tunnel-row">
            <span class="tunnel-name">{{ tunnel.name }}</span>
            <span class="tunnel-type">{{ tunnel.type }}</span>
            <span
              class="status-badge"
              :class="`status-${store.statuses[tunnel.id] || 'disconnected'}`"
            >
              {{ store.statuses[tunnel.id] || 'disconnected' }}
            </span>
          </div>
          <div class="tunnel-row details">
            <span>{{ tunnel.sshHost }}:{{ tunnel.sshPort }}</span>
            <span v-if="tunnel.type === 'local' && tunnel.localAddr" class="address">
              Local {{ tunnel.localAddr }} → {{ tunnel.remoteAddr }}
            </span>
            <span v-else-if="tunnel.type === 'remote' && tunnel.remoteAddr" class="address">
              Remote {{ tunnel.remoteAddr }} → {{ tunnel.localAddr }}
            </span>
            <span v-else-if="tunnel.type === 'dynamic' && tunnel.dynamicAddr" class="address">
              SOCKS5 {{ tunnel.dynamicAddr }}
            </span>
          </div>
        </div>
        <div class="tunnel-actions">
          <button
            class="btn-start"
            :disabled="isActive(tunnel.id)"
            @click="handleStart(tunnel.id)"
          >
            Start
          </button>
          <button
            class="btn-stop"
            :disabled="isInactive(tunnel.id)"
            @click="handleStop(tunnel.id)"
          >
            Stop
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import { useTunnelsStore } from '../stores/tunnels.js'

const store = useTunnelsStore()
let pollInterval = null

function isActive(id) {
  const s = store.statuses[id]
  return s === 'connected' || s === 'connecting'
}

function isInactive(id) {
  const s = store.statuses[id]
  return s === 'disconnected' || s === undefined
}

async function handleStart(id) {
  await store.startTunnel(id)
}

async function handleStop(id) {
  await store.stopTunnel(id)
}

onMounted(() => {
  store.fetchTunnels()
  pollInterval = setInterval(() => {
    store.fetchTunnels()
  }, 5000)
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
  }
})
</script>

<style scoped>
.dashboard-view {
  max-width: 64rem;
  margin: 0 auto;
}

.dashboard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1.5rem;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: #0f172a;
}

.btn-primary {
  display: inline-block;
  padding: 0.5rem 1rem;
  background: #0f172a;
  color: #fff;
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  transition: background 0.15s;
}

.btn-primary:hover {
  background: #1e293b;
}

.state-message {
  padding: 2rem;
  text-align: center;
  color: #64748b;
  font-size: 1rem;
}

.state-message.error {
  color: #dc2626;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  background: #fff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.empty-state p {
  margin: 0 0 1.5rem;
  color: #64748b;
  font-size: 1rem;
}

.tunnel-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.tunnel-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  background: #fff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.tunnel-info {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
}

.tunnel-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.tunnel-name {
  font-weight: 600;
  font-size: 1rem;
  color: #0f172a;
}

.tunnel-type {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #64748b;
  background: #f1f5f9;
  padding: 0.125rem 0.5rem;
  border-radius: 0.25rem;
}

.status-badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  text-transform: capitalize;
}

.status-connected {
  background: #dcfce7;
  color: #166534;
}

.status-connecting {
  background: #fef9c3;
  color: #854d0e;
}

.status-disconnected {
  background: #f1f5f9;
  color: #64748b;
}

.status-error {
  background: #fee2e2;
  color: #991b1b;
}

.tunnel-row.details {
  font-size: 0.875rem;
  color: #475569;
}

.address {
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.8125rem;
}

.tunnel-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  margin-left: 1rem;
}

.btn-start,
.btn-stop {
  padding: 0.375rem 0.75rem;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}

.btn-start {
  background: #16a34a;
  color: #fff;
}

.btn-start:hover:not(:disabled) {
  background: #15803d;
}

.btn-stop {
  background: #dc2626;
  color: #fff;
}

.btn-stop:hover:not(:disabled) {
  background: #b91c1c;
}

.btn-start:disabled,
.btn-stop:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
