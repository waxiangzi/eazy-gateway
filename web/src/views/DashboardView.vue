<template>
  <div class="dashboard-view">
    <div class="dashboard-header">
      <h1>{{ t('tunnel.title') }}</h1>
      <RouterLink to="/tunnels" class="btn-primary">{{ t('tunnel.create') }}</RouterLink>
    </div>

    <div v-if="store.loading && store.list.length === 0" class="state-message">
      {{ t('tunnel.loading') }}
    </div>
    <div v-else-if="store.error" class="state-message error">{{ store.error }}</div>
    <div v-else-if="store.list.length === 0" class="onboarding">
      <div class="onboarding-header">
        <h2>{{ t('tunnel.onboarding.title') }}</h2>
        <p>{{ t('tunnel.onboarding.subtitle') }}</p>
      </div>

      <div class="steps">
        <div
          v-for="(step, idx) in steps"
          :key="idx"
          class="step-card"
          :class="{ active: step.active, done: step.done, disabled: step.disabled }"
        >
          <div class="step-number">{{ idx + 1 }}</div>
          <div class="step-body">
            <h3>{{ step.title }}</h3>
            <p>{{ step.desc }}</p>
            <RouterLink
              v-if="step.to && !step.disabled"
              :to="step.to"
              class="step-action"
            >
              {{ step.action }}
            </RouterLink>
            <span v-else-if="step.action" class="step-action disabled">
              {{ step.action }}
            </span>
          </div>
        </div>
      </div>
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
            <span class="tunnel-type">{{ t('tunnel.types.' + tunnel.type) }}</span>
            <span
              class="status-badge"
              :class="`status-${store.statuses[tunnel.id] || 'disconnected'}`"
            >
              {{ t('tunnel.status.' + (store.statuses[tunnel.id] || 'disconnected')) }}
            </span>
          </div>
          <div class="tunnel-row details">
            <span class="host-name">{{ hostName(tunnel.hostId) }}</span>
            <span class="address">
              {{ tunnel.bindExternal ? '0.0.0.0' : '127.0.0.1' }}:{{ tunnel.listenPort }}
            </span>
            <span v-if="tunnel.type !== 'dynamic'" class="address">
              <template v-if="tunnel.type === 'local'">→ remote </template>
              <template v-else>→ local </template>
              {{ tunnel.targetHost }}:{{ tunnel.targetPort }}
            </span>
          </div>
        </div>
        <div class="tunnel-actions">
          <RouterLink
            :to="`/tunnels/${tunnel.id}`"
            class="btn-edit"
          >
            {{ t('common.edit') }}
          </RouterLink>
          <button
            class="btn-start"
            :disabled="isActive(tunnel.id)"
            @click="handleStart(tunnel.id)"
          >
            {{ t('tunnel.start') }}
          </button>
          <button
            class="btn-stop"
            :disabled="isInactive(tunnel.id)"
            @click="handleStop(tunnel.id)"
          >
            {{ t('tunnel.stop') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTunnelsStore } from '../stores/tunnels.js'
import { useHostsStore } from '../stores/hosts.js'

const { t } = useI18n()
const store = useTunnelsStore()
const hostsStore = useHostsStore()

let pollInterval = null

const steps = computed(() => {
  const hasHost = hostsStore.list.length > 0
  const hasTunnel = store.list.length > 0
  const anyActive = store.list.some((t) => {
    const s = store.statuses[t.id]
    return s === 'connected' || s === 'connecting'
  })

  return [
    {
      title: t('tunnel.onboarding.step1Title'),
      desc: t('tunnel.onboarding.step1Desc'),
      action: hasHost ? t('common.done') : t('tunnel.onboarding.addHost'),
      to: hasHost ? null : '/hosts/new',
      done: hasHost,
      active: !hasHost,
      disabled: false,
    },
    {
      title: t('tunnel.onboarding.step2Title'),
      desc: t('tunnel.onboarding.step2Desc'),
      action: hasHost ? (hasTunnel ? t('common.done') : t('tunnel.onboarding.createTunnel')) : t('common.locked'),
      to: hasHost && !hasTunnel ? '/tunnels' : null,
      done: hasTunnel,
      active: hasHost && !hasTunnel,
      disabled: !hasHost,
    },
    {
      title: t('tunnel.onboarding.step3Title'),
      desc: t('tunnel.onboarding.step3Desc'),
      action: hasTunnel ? (anyActive ? t('tunnel.onboarding.connected') : t('tunnel.onboarding.startTunnel')) : t('common.locked'),
      to: hasTunnel && !anyActive ? '/' : null,
      done: anyActive,
      active: hasTunnel && !anyActive,
      disabled: !hasTunnel,
    },
  ]
})

function hostName(hostId) {
  const host = hostsStore.list.find((h) => h.id === hostId)
  return host ? host.name : hostId.slice(0, 8)
}

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

async function loadData() {
  await store.fetchTunnels()
  await hostsStore.fetchHosts()
}

onMounted(() => {
  loadData()
  pollInterval = setInterval(() => {
    loadData()
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

.onboarding {
  margin-top: 1rem;
}

.onboarding-header {
  text-align: center;
  margin-bottom: 2rem;
}

.onboarding-header h2 {
  margin: 0 0 0.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  color: #0f172a;
}

.onboarding-header p {
  margin: 0;
  color: #64748b;
  font-size: 1rem;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.step-card {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.25rem 1.5rem;
  background: #fff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border: 2px solid transparent;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.step-card.active {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

.step-card.done {
  border-color: #22c55e;
}

.step-card.disabled {
  opacity: 0.6;
}

.step-number {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #e2e8f0;
  color: #475569;
  font-weight: 700;
  font-size: 0.875rem;
}

.step-card.active .step-number {
  background: #3b82f6;
  color: #fff;
}

.step-card.done .step-number {
  background: #22c55e;
  color: #fff;
}

.step-body {
  flex: 1;
}

.step-body h3 {
  margin: 0 0 0.25rem;
  font-size: 1rem;
  font-weight: 600;
  color: #0f172a;
}

.step-body p {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  color: #475569;
  line-height: 1.5;
}

.step-action {
  display: inline-block;
  padding: 0.375rem 0.875rem;
  background: #0f172a;
  color: #fff;
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  transition: background 0.15s;
}

.step-action:hover:not(.disabled) {
  background: #1e293b;
}

.step-action.disabled {
  background: #e2e8f0;
  color: #94a3b8;
  cursor: not-allowed;
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

.host-name {
  font-weight: 500;
  color: #334155;
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

.btn-edit,
.btn-start,
.btn-stop {
  padding: 0.375rem 0.75rem;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  text-decoration: none;
}

.btn-edit {
  background: #3b82f6;
  color: #fff;
}

.btn-edit:hover {
  background: #2563eb;
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
