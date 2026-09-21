<template>
  <div class="tunnel-edit-view">
    <h1 class="page-title">{{ isEditMode ? t('tunnel.edit') : t('tunnel.new') }}</h1>

    <div v-if="isLoading" class="state-message">{{ t('common.loading') }}</div>
    <div v-else-if="loadError" class="state-message error">{{ loadError }}</div>

    <div v-else-if="hostsStore.loading && hostsStore.list.length === 0" class="state-message">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="hostsStore.list.length === 0" class="empty-state">
      <p>{{ t('tunnel.form.noHostDesc') }}</p>
      <RouterLink to="/hosts/new" class="btn-primary">{{ t('tunnel.form.goToHosts') }}</RouterLink>
    </div>

    <form v-else class="tunnel-form" @submit.prevent="handleSubmit">
      <div class="form-group">
        <label for="name">{{ t('common.name') }}</label>
        <input id="name" v-model="form.name" type="text" :placeholder="t('tunnel.form.namePlaceholder')" />
        <p v-if="errors.name" class="field-error">{{ errors.name }}</p>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="type">
            {{ t('common.type') }}
            <TooltipIcon :text="t('tunnel.typeTooltip')" />
          </label>
          <select id="type" v-model="form.type">
            <option value="local">{{ t('tunnel.types.local') }}</option>
            <option value="remote">{{ t('tunnel.types.remote') }}</option>
            <option value="dynamic">{{ t('tunnel.types.dynamic') }}</option>
            <option value="httpToSocks5">{{ t('tunnel.types.httpToSocks5') }}</option>
          </select>
          <p v-if="errors.type" class="field-error">{{ errors.type }}</p>
        </div>

        <div v-if="form.type !== 'httpToSocks5'" class="form-group">
          <label for="hostId">{{ t('common.host') }}</label>
          <select id="hostId" v-model="form.hostId">
            <option value="" disabled>{{ t('tunnel.form.selectHost') }}</option>
            <option v-for="host in hostsStore.list" :key="host.id" :value="host.id">
              {{ host.name }}
            </option>
          </select>
          <p v-if="errors.hostId" class="field-error">{{ errors.hostId }}</p>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group form-group-narrow">
          <label for="listenPort">
            <template v-if="form.type === 'local'">{{ t('tunnel.form.localPortLabel') }}</template>
            <template v-else-if="form.type === 'remote'">{{ t('tunnel.form.remotePortLabel') }}</template>
            <template v-else>{{ t('tunnel.form.listenPortLabel') }}</template>
            <TooltipIcon :text="portTooltipText" />
          </label>
          <input id="listenPort" v-model.number="form.listenPort" type="number" min="1" max="65535" :placeholder="form.type === 'remote' ? '9090' : '8080'" />
          <p v-if="errors.listenPort" class="field-error">{{ errors.listenPort }}</p>
        </div>

        <div class="form-group checkbox-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="form.bindExternal" />
            {{ t('tunnel.form.bindExternal') }}
            <TooltipIcon :text="t('tunnel.form.bindExternalTooltip')" />
          </label>
        </div>
      </div>

      <div v-if="form.type !== 'dynamic' && form.type !== 'httpToSocks5'" class="form-row">
        <div class="form-group">
          <label for="targetHost">
            <template v-if="form.type === 'local'">{{ t('tunnel.form.localTargetHostLabel') }}</template>
            <template v-else>{{ t('tunnel.form.remoteTargetHostLabel') }}</template>
            <TooltipIcon :text="targetHostTooltipText" />
          </label>
          <input id="targetHost" v-model="form.targetHost" type="text" :placeholder="targetHostPlaceholder" />
          <p v-if="errors.targetHost" class="field-error">{{ errors.targetHost }}</p>
        </div>

        <div class="form-group form-group-narrow">
          <label for="targetPort">
            <template v-if="form.type === 'local'">{{ t('tunnel.form.localTargetPortLabel') }}</template>
            <template v-else>{{ t('tunnel.form.remoteTargetPortLabel') }}</template>
          </label>
          <input id="targetPort" v-model.number="form.targetPort" type="number" min="1" max="65535" placeholder="80" />
          <p v-if="errors.targetPort" class="field-error">{{ errors.targetPort }}</p>
        </div>
      </div>

      <div v-if="form.type === 'httpToSocks5'" class="form-row">
        <div class="form-group">
          <label for="socks5Host">{{ t('tunnel.form.socks5HostLabel') }}</label>
          <input id="socks5Host" v-model="form.socks5Host" type="text" />
          <p v-if="errors.socks5Host" class="field-error">{{ errors.socks5Host }}</p>
        </div>
        <div class="form-group form-group-narrow">
          <label for="socks5Port">{{ t('tunnel.form.socks5PortLabel') }}</label>
          <input id="socks5Port" v-model.number="form.socks5Port" type="number" min="1" max="65535" />
          <p v-if="errors.socks5Port" class="field-error">{{ errors.socks5Port }}</p>
        </div>
      </div>

      <div v-if="form.type === 'httpToSocks5'" class="form-group">
        <label>{{ t('tunnel.form.proxyRulesLabel') }}</label>
        <div v-for="(rule, index) in form.proxyRules" :key="index" class="proxy-rule-row">
          <input v-model="rule.domainPattern" type="text" placeholder="*.example.com" />
          <input v-model="rule.socks5Host" type="text" />
          <input v-model.number="rule.socks5Port" type="number" min="1" max="65535" />
          <button type="button" class="btn-secondary" @click="removeProxyRule(index)">
            {{ t('tunnel.form.removeRule') }}
          </button>
        </div>
        <button type="button" class="btn-secondary" @click="addProxyRule">
          + {{ t('tunnel.form.addRule') }}
        </button>
      </div>

      <p v-if="submitError" class="error-message">{{ submitError }}</p>

      <div class="form-actions">
        <button type="button" class="btn-secondary" @click="handleCancel">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" class="btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? t('common.saving') : (isEditMode ? t('common.update') : t('common.create')) }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import TooltipIcon from '../components/TooltipIcon.vue'
import { useTunnelsStore } from '../stores/tunnels.js'
import { useHostsStore } from '../stores/hosts.js'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const tunnelsStore = useTunnelsStore()
const hostsStore = useHostsStore()

const isEditMode = computed(() => !!route.params.id)
const isSubmitting = ref(false)
const submitError = ref('')
const isLoading = ref(false)
const loadError = ref('')

const form = reactive({
  name: '',
  type: 'local',
  hostId: '',
  listenPort: null,
  targetHost: '',
  targetPort: null,
  bindExternal: false,
  socks5Host: '',
  socks5Port: null,
  proxyRules: [],
})

const errors = reactive({
  name: '',
  type: '',
  hostId: '',
  listenPort: '',
  targetHost: '',
  targetPort: '',
  socks5Host: '',
  socks5Port: '',
})

function clearErrors() {
  errors.name = ''
  errors.type = ''
  errors.hostId = ''
  errors.listenPort = ''
  errors.targetHost = ''
  errors.targetPort = ''
  errors.socks5Host = ''
  errors.socks5Port = ''
}

const portTooltipText = computed(() => {
  if (form.type === 'local') return t('tunnel.form.localPortTooltip')
  if (form.type === 'remote') return t('tunnel.form.remotePortTooltip')
  return t('tunnel.form.dynamicPortTooltip')
})

const targetHostTooltipText = computed(() => {
  if (form.type === 'local') return t('tunnel.form.localTargetHostTooltip')
  return t('tunnel.form.remoteTargetHostTooltip')
})

const targetHostPlaceholder = computed(() => {
  if (form.type === 'local') return t('tunnel.form.localTargetHostPlaceholder')
  return t('tunnel.form.remoteTargetHostPlaceholder')
})

function validate() {
  clearErrors()
  let valid = true

  if (!form.name.trim()) {
    errors.name = t('validation.nameRequired')
    valid = false
  }
  if (form.type !== 'httpToSocks5' && !form.hostId) {
    errors.hostId = t('validation.hostIdRequired')
    valid = false
  }
  if (!form.listenPort || form.listenPort <= 0) {
    errors.listenPort = t('validation.listenPortInvalid')
    valid = false
  }
  if (form.type !== 'dynamic' && form.type !== 'httpToSocks5') {
    if (!form.targetHost.trim()) {
      errors.targetHost = t('validation.targetHostRequired')
      valid = false
    }
    if (!form.targetPort || form.targetPort <= 0) {
      errors.targetPort = t('validation.targetPortInvalid')
      valid = false
    }
  }
  if (form.type === 'httpToSocks5') {
    if (form.proxyRules.length === 0) {
      if (!form.socks5Host.trim()) {
        errors.socks5Host = t('validation.socks5HostRequired')
        valid = false
      }
      if (!form.socks5Port || form.socks5Port <= 0 || form.socks5Port > 65535) {
        errors.socks5Port = t('validation.socks5PortInvalid')
        valid = false
      }
    }
    for (let i = 0; i < form.proxyRules.length; i++) {
      const rule = form.proxyRules[i]
      if (!rule.domainPattern.trim()) {
        valid = false
      }
      if (!rule.socks5Host.trim()) {
        valid = false
      }
      if (!rule.socks5Port || rule.socks5Port <= 0 || rule.socks5Port > 65535) {
        valid = false
      }
    }
  }

  return valid
}

async function handleSubmit() {
  submitError.value = ''
  if (!validate()) return

  isSubmitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      type: form.type,
      hostId: form.hostId,
      listenPort: Number(form.listenPort),
      targetHost: form.targetHost,
      targetPort: form.type === 'dynamic' || form.type === 'httpToSocks5' ? 0 : Number(form.targetPort),
      bindExternal: form.bindExternal,
      socks5Host: form.socks5Host || '',
      socks5Port: Number(form.socks5Port) || 0,
      proxyRules: form.proxyRules.map(r => ({ domainPattern: r.domainPattern, socks5Host: r.socks5Host, socks5Port: Number(r.socks5Port) })),
    }

    if (isEditMode.value) {
      await tunnelsStore.updateTunnel(route.params.id, payload)
    } else {
      await tunnelsStore.createTunnel(payload)
    }
    router.push({ name: 'Dashboard' })
  } catch (err) {
    if (!err.response) {
      submitError.value = t('errors.network')
    } else {
      submitError.value = err.response.data?.error || err.response.data?.message || t('errors.saveFailed')
    }
  } finally {
    isSubmitting.value = false
  }
}

function handleCancel() {
  router.push({ name: 'Dashboard' })
}

function addProxyRule() {
  form.proxyRules.push({ domainPattern: '', socks5Host: '', socks5Port: null })
}

function removeProxyRule(index) {
  form.proxyRules.splice(index, 1)
}

watch(() => form.type, (newType, oldType) => {
  if (newType !== oldType) {
    if (newType === 'dynamic') {
      form.targetHost = ''
      form.targetPort = null
    }
    if (newType === 'httpToSocks5') {
      form.targetHost = ''
      form.targetPort = null
      form.hostId = ''
    }
    if (oldType === 'httpToSocks5') {
      form.socks5Host = ''
      form.socks5Port = null
      form.proxyRules = []
    }
  }
})

onMounted(async () => {
  isLoading.value = true
  loadError.value = ''

  try {
    if (hostsStore.list.length === 0) {
      try {
        await hostsStore.fetchHosts()
      } catch {
        loadError.value = t('errors.loadFailed')
      }
    }

    if (isEditMode.value) {
      const response = await tunnelsStore.getTunnel(route.params.id)
      const tunnel = response.tunnel || response
      form.name = tunnel.name || ''
      form.type = tunnel.type || 'local'
      form.hostId = tunnel.hostId || ''
      form.listenPort = tunnel.listenPort || null
      form.targetHost = tunnel.targetHost || ''
      form.targetPort = tunnel.targetPort || null
      form.bindExternal = tunnel.bindExternal || false
      form.socks5Host = tunnel.socks5Host || ''
      form.socks5Port = tunnel.socks5Port || null
      form.proxyRules = Array.isArray(tunnel.proxyRules) ? tunnel.proxyRules.map(r => ({ ...r, socks5Port: r.socks5Port || null })) : []
    }
  } catch (err) {
    if (!err.response) {
      loadError.value = t('errors.network')
    } else {
      loadError.value = err.response.data?.error || err.response.data?.message || t('errors.loadFailed')
    }
  } finally {
    isLoading.value = false
  }
})
</script>

<style scoped>
.tunnel-edit-view {
  max-width: 48rem;
  margin: 0 auto;
  padding: 1.5rem 1rem;
}

.page-title {
  margin: 0 0 1.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(var(--accent-rgb), 0.2);
}

.tunnel-form {
  background: rgba(var(--card-rgb), 0.65);
  padding: 1.5rem;
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.form-group {
  margin-bottom: 1rem;
  flex: 1;
}

.form-group label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-secondary);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: var(--text-primary);
  background: rgba(var(--page-rgb), 0.8);
  box-sizing: border-box;
  font-family: inherit;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: rgba(var(--accent-rgb), 0.55);
  box-shadow: 0 0 0 3px rgba(var(--accent-rgb), 0.15), 0 0 8px rgba(var(--accent-rgb), 0.2);
}

.form-row {
  display: flex;
  gap: 1rem;
}

.proxy-rule-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.5rem;
}

.form-group-narrow {
  flex: 0 0 10rem;
}

.checkbox-group {
  display: flex;
  align-items: flex-end;
  padding-bottom: 0.5rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--text-secondary);
  cursor: pointer;
}

.checkbox-label input[type="checkbox"] {
  width: auto;
  margin: 0;
}

.field-error {
  margin: 0.25rem 0 0;
  color: var(--error);
  font-size: 0.875rem;
}

.error-message {
  margin: 0 0 1rem;
  color: var(--error);
  font-size: 0.875rem;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1.5rem;
}

.btn-primary,
.btn-secondary {
  padding: 0.625rem 1.25rem;
  border: none;
  border-radius: 0.375rem;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-primary {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),0.9), rgba(var(--accent-strong-rgb),0.9));
  color: var(--accent-fg);
  border: 1px solid rgba(var(--accent-rgb), 0.35);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.15);
}

.btn-primary:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(var(--accent-rgb),1), rgba(var(--accent-strong-rgb),1));
  box-shadow: 0 0 16px rgba(var(--accent-rgb), 0.3);
  transform: translateY(-1px);
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-secondary {
  background: rgba(var(--neutral-rgb), 0.1);
  color: var(--text-primary);
  border: 1px solid rgba(var(--accent-rgb), 0.2);
}

.btn-secondary:hover {
  background: rgba(var(--accent-rgb), 0.12);
  border-color: rgba(var(--accent-rgb), 0.4);
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

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.empty-state p {
  margin: 0 0 1.5rem;
  color: var(--text-secondary);
  font-size: 1rem;
}
</style>
