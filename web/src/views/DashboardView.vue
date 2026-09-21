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
        v-for="tunnel in sortedList"
        :key="tunnel.id"
        class="tunnel-card"
      >
        <div class="tunnel-header">
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
              <span v-if="tunnel.type !== 'httpToSocks5'" class="host-name">{{ hostName(tunnel.hostId) }}</span>
              <span class="address">
                {{ tunnel.bindExternal ? '0.0.0.0' : '127.0.0.1' }}:{{ tunnel.listenPort }}
              </span>
              <span v-if="tunnel.type !== 'dynamic' && tunnel.type !== 'httpToSocks5'" class="address">
                <template v-if="tunnel.type === 'local'">→ remote </template>
                <template v-else>→ local </template>
                {{ tunnel.targetHost }}:{{ tunnel.targetPort }}
              </span>
              <span v-if="tunnel.type === 'httpToSocks5'" class="address">→ SOCKS5</span>
            </div>
            <div class="tunnel-row traffic">
              <span class="traffic-label">{{ t('tunnel.traffic.in') }}</span>
              <span class="traffic-value">{{ formatBytes(store.traffic[tunnel.id]?.bytesIn || 0) }}</span>
              <span class="traffic-label">{{ t('tunnel.traffic.out') }}</span>
              <span class="traffic-value">{{ formatBytes(store.traffic[tunnel.id]?.bytesOut || 0) }}</span>
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
        <div class="tunnel-row trend-toggle">
          <button class="btn-trend" @click.stop="toggleTrend(tunnel.id)">
            {{ expandedTrendId === tunnel.id ? t('tunnel.trend.hide') : t('tunnel.trend.show') }}
          </button>
        </div>
        <div v-if="expandedTrendId === tunnel.id" class="trend-panel">
          <div v-if="trendLoading[tunnel.id]" class="trend-loading">{{ t('common.loading') }}</div>
          <TrafficChart v-else :points="trendData[tunnel.id] || []" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTunnelsStore } from '../stores/tunnels.js'
import { useHostsStore } from '../stores/hosts.js'
import { useSettingsStore } from '../stores/settings.js'
import TrafficChart from '../components/TrafficChart.vue'

const { t } = useI18n()
const store = useTunnelsStore()
const hostsStore = useHostsStore()
const settingsStore = useSettingsStore()

let pollInterval = null

const expandedTrendId = ref(null)
const trendData = ref({})
const trendLoading = ref({})

async function toggleTrend(id) {
  if (expandedTrendId.value === id) {
    expandedTrendId.value = null
    return
  }
  expandedTrendId.value = id
  if (!trendData.value[id]) {
    trendLoading.value[id] = true
    const points = await store.fetchTrafficTrend(id, settingsStore.trafficTrendHours)
    trendData.value[id] = points
    trendLoading.value[id] = false
  }
}

const sortedList = computed(() => {
  return [...store.list].sort((a, b) => {
    const ta = a.createdAt ? new Date(a.createdAt).getTime() : 0
    const tb = b.createdAt ? new Date(b.createdAt).getTime() : 0
    return ta - tb
  })
})

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

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

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
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(var(--accent-rgb), 0.2);
}

.btn-primary {
  display: inline-block;
  padding: 0.5rem 1rem;
  background: linear-gradient(135deg, rgba(var(--accent-rgb),0.9), rgba(var(--accent-strong-rgb),0.9));
  color: var(--accent-fg);
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  border: 1px solid rgba(var(--accent-rgb), 0.35);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.15);
  transition: all 0.2s;
}

.btn-primary:hover {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),1), rgba(var(--accent-strong-rgb),1));
  box-shadow: 0 0 16px rgba(var(--accent-rgb), 0.3);
  transform: translateY(-1px);
}

.state-message {
  padding: 2rem;
  text-align: center;
  color: var(--text-muted);
  font-size: 1rem;
}

.state-message.error {
  color: var(--error);
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
  color: var(--text-primary);
}

.onboarding-header p {
  margin: 0;
  color: var(--text-muted);
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
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.step-card.active {
  border-color: rgba(var(--accent-rgb), 0.45);
  box-shadow: 0 0 16px rgba(var(--accent-rgb), 0.15), 0 4px 8px rgba(0, 0, 0, 0.3);
}

.step-card.done {
  border-color: rgba(var(--success-rgb), 0.45);
}

.step-card.disabled {
  opacity: 0.5;
}

.step-number {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(var(--slate-rgb), 0.15);
  color: var(--text-secondary);
  font-weight: 700;
  font-size: 0.875rem;
  border: 1px solid rgba(var(--slate-rgb), 0.2);
}

.step-card.active .step-number {
  background: rgba(var(--accent-rgb), 0.2);
  color: var(--accent);
  border-color: rgba(var(--accent-rgb), 0.4);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.2);
}

.step-card.done .step-number {
  background: rgba(var(--success-rgb), 0.2);
  color: var(--success);
  border-color: rgba(var(--success-rgb), 0.4);
  box-shadow: 0 0 8px rgba(var(--success-rgb), 0.2);
}

.step-body {
  flex: 1;
}

.step-body h3 {
  margin: 0 0 0.25rem;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.step-body p {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  color: var(--text-secondary);
  line-height: 1.5;
}

.step-action {
  display: inline-block;
  padding: 0.375rem 0.875rem;
  background: linear-gradient(135deg, rgba(var(--accent-rgb),0.9), rgba(var(--accent-strong-rgb),0.9));
  color: var(--accent-fg);
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  border: 1px solid rgba(var(--accent-rgb), 0.35);
  box-shadow: 0 0 6px rgba(var(--accent-rgb), 0.15);
  transition: all 0.2s;
}

.step-action:hover:not(.disabled) {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),1), rgba(var(--accent-strong-rgb),1));
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.25);
  transform: translateY(-1px);
}

.step-action.disabled {
  background: rgba(var(--slate-rgb), 0.1);
  color: var(--text-muted);
  border-color: rgba(var(--slate-rgb), 0.15);
  cursor: not-allowed;
  box-shadow: none;
}

.tunnel-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.tunnel-card {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 1rem 1.25rem;
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.tunnel-card:hover {
  border-color: rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 0 18px rgba(var(--accent-rgb), 0.1), 0 4px 12px rgba(0, 0, 0, 0.4);
}

.tunnel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
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
  color: var(--text-primary);
}

.tunnel-type {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  background: rgba(var(--slate-rgb), 0.1);
  padding: 0.125rem 0.5rem;
  border-radius: 0.25rem;
  border: 1px solid rgba(var(--slate-rgb), 0.15);
}

.status-badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  text-transform: capitalize;
  border: 1px solid transparent;
}

.status-connected {
  background: rgba(var(--success-rgb), 0.12);
  color: var(--success);
  border-color: rgba(var(--success-rgb), 0.25);
  box-shadow: 0 0 6px rgba(var(--success-rgb), 0.12);
}

.status-connecting {
  background: rgba(var(--warning-rgb), 0.12);
  color: var(--warning);
  border-color: rgba(var(--warning-rgb), 0.25);
  box-shadow: 0 0 6px rgba(var(--warning-rgb), 0.12);
}

.status-disconnected {
  background: rgba(var(--slate-rgb), 0.08);
  color: var(--text-muted);
  border-color: rgba(var(--slate-rgb), 0.15);
}

.status-error {
  background: rgba(var(--error-rgb), 0.1);
  color: var(--error);
  border-color: rgba(var(--error-rgb), 0.2);
  box-shadow: 0 0 6px rgba(var(--error-rgb), 0.1);
}

.tunnel-row.details {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.host-name {
  font-weight: 500;
  color: var(--text-soft);
}

.address {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.8125rem;
  color: var(--accent);
}

.tunnel-row.traffic {
  font-size: 0.75rem;
  color: var(--text-muted);
  gap: 0.375rem;
}

.traffic-label {
  color: var(--text-muted);
}

.traffic-value {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  color: var(--success);
  margin-right: 0.5rem;
  text-shadow: 0 0 4px rgba(var(--success-rgb), 0.25);
}

.trend-toggle {
  margin-top: 0.25rem;
}

.btn-trend {
  padding: 0.25rem 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  border-radius: 0.25rem;
  background: rgba(var(--page-rgb), 0.6);
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.2s;
}

.btn-trend:hover {
  background: rgba(var(--accent-rgb), 0.1);
  border-color: rgba(var(--accent-rgb), 0.4);
  color: var(--accent);
}

.trend-panel {
  margin-top: 0.5rem;
  padding-top: 0.5rem;
  border-top: 1px solid rgba(var(--accent-rgb), 0.1);
}

.trend-loading {
  padding: 1rem;
  text-align: center;
  color: var(--text-muted);
  font-size: 0.875rem;
}

.tunnel-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  margin-top: 0.25rem;
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
  transition: all 0.2s;
  text-decoration: none;
}

.btn-edit {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),0.85), rgba(var(--accent-strong-rgb),0.85));
  color: var(--accent-fg);
  border: 1px solid rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 0 6px rgba(var(--accent-rgb), 0.15);
}

.btn-edit:hover {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),1), rgba(var(--accent-strong-rgb),1));
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.25);
}

.btn-start {
  background: linear-gradient(135deg, rgba(var(--success-rgb),0.85), rgba(var(--success-strong-rgb), 0.85));
  color: var(--accent-fg);
  border: 1px solid rgba(var(--success-rgb), 0.3);
  box-shadow: 0 0 6px rgba(var(--success-rgb), 0.15);
}

.btn-start:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(var(--success-rgb),1), rgba(var(--success-strong-rgb), 1));
  box-shadow: 0 0 12px rgba(var(--success-rgb), 0.25);
}

.btn-stop {
  background: linear-gradient(135deg, rgba(var(--error-rgb),0.85), rgba(var(--danger-rgb),0.85));
  color: var(--accent-fg);
  border: 1px solid rgba(var(--error-rgb), 0.3);
  box-shadow: 0 0 6px rgba(var(--error-rgb), 0.15);
}

.btn-stop:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(var(--error-rgb),1), rgba(var(--danger-rgb),1));
  box-shadow: 0 0 12px rgba(var(--error-rgb), 0.25);
}

.btn-start:disabled,
.btn-stop:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  box-shadow: none;
}
</style>
