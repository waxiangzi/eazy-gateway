<template>
  <div class="host-edit-view">
    <h1 class="page-title">{{ isEditMode ? t('host.edit') : t('host.new') }}</h1>

    <div v-if="isLoading" class="state-message">{{ t('common.loading') }}</div>
    <div v-else-if="loadError" class="state-message error">{{ loadError }}</div>

    <form v-else class="host-form" @submit.prevent="handleSubmit">
      <div class="form-group">
        <label for="name">
          {{ t('common.name') }}
          <TooltipIcon :text="t('host.form.nameTooltip')" />
        </label>
        <input id="name" v-model="form.name" type="text" :placeholder="t('host.form.namePlaceholder')" />
        <p v-if="errors.name" class="field-error">{{ errors.name }}</p>
      </div>

      <div class="form-group">
        <label for="address">
          {{ t('host.form.addressLabel') }}
          <TooltipIcon :text="t('host.form.addressTooltip')" />
        </label>
        <input id="address" v-model="form.address" type="text" :placeholder="t('host.form.addressPlaceholder')" />
        <p v-if="errors.address" class="field-error">{{ errors.address }}</p>
      </div>

      <div class="form-group">
        <label for="keyId">{{ t('host.form.keyLabel') }}</label>
        <select id="keyId" v-model="form.keyId">
          <option value="" disabled>{{ t('host.form.selectKey') }}</option>
          <option v-for="key in keysStore.list" :key="key.id" :value="key.id">
            {{ key.name }}
          </option>
        </select>
        <p v-if="errors.keyId" class="field-error">{{ errors.keyId }}</p>
      </div>

      <p v-if="submitError" class="error-message">{{ submitError }}</p>
      <p v-if="testResult" :class="['test-message', testSuccess ? 'success' : 'error']">{{ testResult }}</p>

      <div class="form-actions">
        <button type="button" class="btn-secondary" @click="handleCancel">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="isTesting || !canTest" @click="handleTest">
          {{ isTesting ? t('host.testing') : t('host.test') }}
        </button>
        <button type="submit" class="btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? t('common.saving') : (isEditMode ? t('common.update') : t('common.create')) }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import TooltipIcon from '../components/TooltipIcon.vue'
import { useHostsStore } from '../stores/hosts.js'
import { useKeysStore } from '../stores/keys.js'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const hostsStore = useHostsStore()
const keysStore = useKeysStore()

const isEditMode = computed(() => !!route.params.id && route.params.id !== 'new')
const isSubmitting = ref(false)
const isTesting = ref(false)
const testResult = ref('')
const testSuccess = ref(false)
const submitError = ref('')
const isLoading = ref(false)
const loadError = ref('')

const form = reactive({
  name: '',
  address: '',
  keyId: '',
})

const errors = reactive({
  name: '',
  address: '',
  keyId: '',
})

function clearErrors() {
  errors.name = ''
  errors.address = ''
  errors.keyId = ''
  testResult.value = ''
}

const canTest = computed(() => {
  if (!form.name.trim() || !form.address.trim() || !form.keyId) return false
  return parseAddress(form.address) !== null
})

/**
 * Parse address string in format "user@host:port".
 * Supports IPv6 bracket notation: user@[ipv6]:port.
 * Port is optional and defaults to 22.
 * Returns { user, host, port } or null on failure.
 */
function parseAddress(addr) {
  const trimmed = addr.trim()
  if (!trimmed) return null

  // IPv6 bracket notation: user@[ipv6]:port
  const ipv6Match = trimmed.match(/^([^@]+)@\[([^\]]+)\](?::(\d+))?$/)
  if (ipv6Match) {
    const user = ipv6Match[1]
    const host = `[${ipv6Match[2]}]`
    const port = ipv6Match[3] ? parseInt(ipv6Match[3], 10) : 22
    if (!user || !ipv6Match[2] || isNaN(port) || port <= 0 || port > 65535) {
      return null
    }
    return { user, host, port }
  }

  const match = trimmed.match(/^([^@]+)@([^:]+)(?::(\d+))?$/)
  if (!match) return null

  const user = match[1]
  const host = match[2]
  const port = match[3] ? parseInt(match[3], 10) : 22

  if (!user || !host || isNaN(port) || port <= 0 || port > 65535) {
    return null
  }

  return { user, host, port }
}

function formatAddress(host) {
  if (!host) return ''
  const port = host.port || 22
  if (port === 22) {
    return `${host.user}@${host.host}`
  }
  return `${host.user}@${host.host}:${port}`
}

function validate() {
  clearErrors()
  let valid = true

  if (!form.name.trim()) {
    errors.name = t('validation.nameRequired')
    valid = false
  }
  if (!form.address.trim()) {
    errors.address = t('validation.addressRequired')
    valid = false
  } else {
    const parsed = parseAddress(form.address)
    if (!parsed) {
      errors.address = t('validation.addressInvalid')
      valid = false
    }
  }
  if (!form.keyId) {
    errors.keyId = t('validation.keyRequired')
    valid = false
  }

  return valid
}

async function handleSubmit() {
  submitError.value = ''
  if (!validate()) return

  const parsed = parseAddress(form.address)

  isSubmitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      host: parsed.host,
      port: parsed.port,
      user: parsed.user,
      keyId: form.keyId,
    }

    if (isEditMode.value) {
      await hostsStore.updateHost(route.params.id, payload)
    } else {
      await hostsStore.createHost(payload)
    }
    router.push('/hosts')
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

async function handleTest() {
  testResult.value = ''
  testSuccess.value = false
  if (!canTest.value) return

  const parsed = parseAddress(form.address)
  if (!parsed) return

  isTesting.value = true
  try {
    await hostsStore.testHost({
      name: form.name.trim(),
      host: parsed.host,
      port: parsed.port,
      user: parsed.user,
      keyId: form.keyId,
    })
    testResult.value = t('host.testSuccess')
    testSuccess.value = true
  } catch (err) {
    testResult.value = err.response?.data?.error || t('host.testFailed')
    testSuccess.value = false
  } finally {
    isTesting.value = false
  }
}

function handleCancel() {
  router.push({ name: 'Dashboard' })
}

onMounted(async () => {
  isLoading.value = true
  loadError.value = ''

  try {
    if (keysStore.list.length === 0) {
      try {
        await keysStore.fetchKeys()
      } catch {
        // Keys are not strictly required to display the form; user may still
        // edit name/address. Validation on submit will enforce key selection.
        console.warn('Failed to load keys for host edit form')
      }
    }

    if (isEditMode.value) {
      const host = await hostsStore.getHost(route.params.id)
      form.name = host.name || ''
      form.address = formatAddress(host)
      form.keyId = host.keyId || ''
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
.host-edit-view {
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

.host-form {
  background: rgba(var(--card-rgb), 0.65);
  padding: 1.5rem;
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), var(--shadow-sm);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.form-group {
  margin-bottom: 1.25rem;
}

.form-group:last-of-type {
  margin-bottom: 0;
}

.form-group label {
  display: block;
  margin-bottom: 0.375rem;
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

.field-error {
  margin: 0.375rem 0 0;
  color: var(--error);
  font-size: 0.875rem;
}

.error-message {
  margin: 0 0 1rem;
  color: var(--error);
  font-size: 0.875rem;
}

.test-message {
  margin: 0 0 1rem;
  font-size: 0.875rem;
}

.test-message.success {
  color: var(--success);
}

.test-message.error {
  color: var(--error);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1.5rem;
  padding-top: 1.25rem;
  border-top: 1px solid rgba(var(--accent-rgb), 0.1);
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
</style>
