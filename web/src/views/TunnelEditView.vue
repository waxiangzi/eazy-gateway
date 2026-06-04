<template>
  <div class="tunnel-edit-view">
    <h1 class="page-title">{{ isEditMode ? 'Edit Tunnel' : 'Create Tunnel' }}</h1>

    <form class="tunnel-form" @submit.prevent="handleSubmit">
      <div class="form-group">
        <label for="name">Name</label>
        <input
          id="name"
          v-model="form.name"
          type="text"
          placeholder="Tunnel name"
        />
        <p v-if="errors.name" class="field-error">{{ errors.name }}</p>
      </div>

      <div class="form-group">
        <label for="type">Type</label>
        <select id="type" v-model="form.type">
          <option value="local">Local</option>
          <option value="remote">Remote</option>
          <option value="dynamic">Dynamic</option>
        </select>
        <p v-if="errors.type" class="field-error">{{ errors.type }}</p>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="sshHost">SSH Host</label>
          <input
            id="sshHost"
            v-model="form.sshHost"
            type="text"
            placeholder="e.g. 192.168.1.1"
          />
          <p v-if="errors.sshHost" class="field-error">{{ errors.sshHost }}</p>
        </div>

        <div class="form-group">
          <label for="sshPort">SSH Port</label>
          <input
            id="sshPort"
            v-model.number="form.sshPort"
            type="number"
            min="1"
            max="65535"
          />
          <p v-if="errors.sshPort" class="field-error">{{ errors.sshPort }}</p>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="sshUser">SSH User</label>
          <input
            id="sshUser"
            v-model="form.sshUser"
            type="text"
            placeholder="e.g. root"
          />
          <p v-if="errors.sshUser" class="field-error">{{ errors.sshUser }}</p>
        </div>

        <div class="form-group">
          <label for="keyId">Key</label>
          <select id="keyId" v-model="form.keyId">
            <option value="" disabled>Select a key</option>
            <option v-for="key in keysStore.list" :key="key.id" :value="key.id">
              {{ key.name }}
            </option>
          </select>
          <p v-if="errors.keyId" class="field-error">{{ errors.keyId }}</p>
        </div>
      </div>

      <div v-if="form.type === 'local'" class="form-row">
        <div class="form-group">
          <label for="localAddr">Local Address</label>
          <input
            id="localAddr"
            v-model="form.localAddr"
            type="text"
            placeholder="e.g. 127.0.0.1:8080"
          />
        </div>
        <div class="form-group">
          <label for="remoteAddr">Remote Address</label>
          <input
            id="remoteAddr"
            v-model="form.remoteAddr"
            type="text"
            placeholder="e.g. 127.0.0.1:80"
          />
        </div>
      </div>

      <div v-if="form.type === 'remote'" class="form-row">
        <div class="form-group">
          <label for="remoteAddr">Remote Address</label>
          <input
            id="remoteAddr"
            v-model="form.remoteAddr"
            type="text"
            placeholder="e.g. 0.0.0.0:8080"
          />
        </div>
        <div class="form-group">
          <label for="localAddr">Local Address</label>
          <input
            id="localAddr"
            v-model="form.localAddr"
            type="text"
            placeholder="e.g. 127.0.0.1:80"
          />
        </div>
      </div>

      <div v-if="form.type === 'dynamic'" class="form-group">
        <label for="dynamicAddr">Dynamic Address</label>
        <input
          id="dynamicAddr"
          v-model="form.dynamicAddr"
          type="text"
          placeholder="e.g. 127.0.0.1:1080"
        />
      </div>

      <p v-if="submitError" class="error-message">{{ submitError }}</p>

      <div class="form-actions">
        <button type="button" class="btn-secondary" @click="handleCancel">
          Cancel
        </button>
        <button type="submit" class="btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Saving…' : (isEditMode ? 'Update' : 'Create') }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTunnelsStore } from '../stores/tunnels.js'
import { useKeysStore } from '../stores/keys.js'

const route = useRoute()
const router = useRouter()
const tunnelsStore = useTunnelsStore()
const keysStore = useKeysStore()

const isEditMode = computed(() => !!route.params.id)
const isSubmitting = ref(false)
const submitError = ref('')

const form = reactive({
  name: '',
  type: 'local',
  sshHost: '',
  sshPort: 22,
  sshUser: '',
  keyId: '',
  localAddr: '',
  remoteAddr: '',
  dynamicAddr: '',
})

const errors = reactive({
  name: '',
  type: '',
  sshHost: '',
  sshPort: '',
  sshUser: '',
  keyId: '',
})

function clearErrors() {
  errors.name = ''
  errors.type = ''
  errors.sshHost = ''
  errors.sshPort = ''
  errors.sshUser = ''
  errors.keyId = ''
}

function validate() {
  clearErrors()
  let valid = true

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    valid = false
  }

  if (!form.sshHost.trim()) {
    errors.sshHost = 'SSH Host is required'
    valid = false
  }

  if (!form.sshPort || form.sshPort <= 0) {
    errors.sshPort = 'SSH Port must be greater than 0'
    valid = false
  }

  if (!form.sshUser.trim()) {
    errors.sshUser = 'SSH User is required'
    valid = false
  }

  if (!form.keyId) {
    errors.keyId = 'Key is required'
    valid = false
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
      sshHost: form.sshHost.trim(),
      sshPort: form.sshPort,
      sshUser: form.sshUser.trim(),
      keyId: form.keyId,
      localAddr: form.localAddr.trim() || undefined,
      remoteAddr: form.remoteAddr.trim() || undefined,
      dynamicAddr: form.dynamicAddr.trim() || undefined,
    }

    if (isEditMode.value) {
      await tunnelsStore.updateTunnel(route.params.id, payload)
    } else {
      await tunnelsStore.createTunnel(payload)
    }
    router.push('/')
  } catch (err) {
    if (!err.response) {
      submitError.value = 'Network error'
    } else {
      submitError.value = err.response.data?.error || err.response.data?.message || 'Save failed'
    }
  } finally {
    isSubmitting.value = false
  }
}

function handleCancel() {
  router.push('/')
}

// Reset conditional fields when type changes
watch(() => form.type, (newType, oldType) => {
  if (newType !== oldType) {
    form.localAddr = ''
    form.remoteAddr = ''
    form.dynamicAddr = ''
  }
})

onMounted(async () => {
  // Load keys for dropdown
  if (keysStore.list.length === 0) {
    try {
      await keysStore.fetchKeys()
    } catch {
      // ignore; dropdown will be empty
    }
  }

  // Load tunnel data in edit mode
  if (isEditMode.value) {
    try {
      const tunnel = await tunnelsStore.getTunnel(route.params.id)
      form.name = tunnel.name || ''
      form.type = tunnel.type || 'local'
      form.sshHost = tunnel.sshHost || ''
      form.sshPort = tunnel.sshPort || 22
      form.sshUser = tunnel.sshUser || ''
      form.keyId = tunnel.keyId || ''
      form.localAddr = tunnel.localAddr || ''
      form.remoteAddr = tunnel.remoteAddr || ''
      form.dynamicAddr = tunnel.dynamicAddr || ''
    } catch (err) {
      if (!err.response) {
        submitError.value = 'Network error'
      } else {
        submitError.value = err.response.data?.error || err.response.data?.message || 'Failed to load tunnel'
      }
    }
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
  color: #0f172a;
}

.tunnel-form {
  background: #ffffff;
  padding: 1.5rem;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
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
  color: #334155;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-size: 1rem;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}

.form-row {
  display: flex;
  gap: 1rem;
}

.field-error {
  margin: 0.25rem 0 0;
  color: #dc2626;
  font-size: 0.875rem;
}

.error-message {
  margin: 0 0 1rem;
  color: #dc2626;
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
  transition: background 0.15s;
}

.btn-primary {
  background: #0f172a;
  color: #fff;
}

.btn-primary:hover:not(:disabled) {
  background: #1e293b;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: #e2e8f0;
  color: #0f172a;
}

.btn-secondary:hover {
  background: #cbd5e1;
}
</style>
