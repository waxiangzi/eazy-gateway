<template>
  <div class="keys-view">
    <h1 class="page-title">SSH Keys</h1>

    <section class="generate-section">
      <h2 class="section-title">Generate Key</h2>
      <form class="generate-form" @submit.prevent="handleCreate">
        <div class="form-group">
          <label for="key-name">Name</label>
          <input
            id="key-name"
            v-model="form.name"
            type="text"
            placeholder="e.g. production-server"
            required
          />
        </div>
        <p v-if="keys.generateError" class="error-message">{{ keys.generateError }}</p>
        <button type="submit" class="btn-primary" :disabled="keys.isGenerating">
          {{ keys.isGenerating ? 'Generating…' : 'Generate Key' }}
        </button>
      </form>
    </section>

    <section class="list-section">
      <h2 class="section-title">Your Keys</h2>

      <div v-if="keys.isLoading" class="loading-text">Loading keys…</div>

      <div v-else-if="!keys.hasKeys" class="empty-state">
        <p>No SSH keys yet.</p>
        <p class="empty-hint">Generate a key above to get started.</p>
      </div>

      <div v-else class="key-list">
        <div v-if="keys.deleteError" class="error-message list-error">{{ keys.deleteError }}</div>
        <div
          v-for="key in keys.list"
          :key="key.id"
          class="key-card"
        >
          <div class="key-info">
            <span class="key-name">{{ key.name }}</span>
            <span v-if="key.publicKey" class="key-public">{{ key.publicKey.trim() }}</span>
            <span class="key-date">{{ formatDate(key.createdAt) }}</span>
          </div>
          <button
            class="btn-danger"
            :disabled="keys.isDeleting"
            @click="handleDelete(key.id, key.name)"
          >
            Delete
          </button>
        </div>
      </div>
    </section>

    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content">
        <h2 class="modal-title">Key Generated: {{ generatedKeyName }}</h2>
        <p class="modal-notice">
          This is the only time the private key will be displayed. Save it securely now.
        </p>
        <textarea
          class="modal-key-area"
          :value="generatedPrivateKey"
          readonly
          rows="10"
          @click="$event.target.select()"
        ></textarea>
        <div class="modal-actions">
          <button class="btn-primary" @click="copyPrivateKey">Copy to Clipboard</button>
          <button class="btn-secondary" @click="closeModal">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useKeysStore } from '../stores/keys.js'

const keys = useKeysStore()

const form = reactive({
  name: '',
})

const showModal = ref(false)
const generatedKeyName = ref('')
const generatedPrivateKey = ref('')

function formatDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString()
}

async function handleCreate() {
  const result = await keys.createKey({
    name: form.name.trim(),
  })
  if (result) {
    generatedKeyName.value = result.name
    generatedPrivateKey.value = result.privateKey
    showModal.value = true
    form.name = ''
    await keys.fetchKeys()
  }
}

function closeModal() {
  showModal.value = false
  generatedKeyName.value = ''
  generatedPrivateKey.value = ''
}

async function copyPrivateKey() {
  try {
    await navigator.clipboard.writeText(generatedPrivateKey.value)
  } catch {
    // fallback: select the textarea text
  }
}

async function handleDelete(id, name) {
  const confirmed = window.confirm(`Delete key "${name}"?`)
  if (!confirmed) return
  const ok = await keys.deleteKey(id)
  if (ok) {
    await keys.fetchKeys()
  }
}

onMounted(() => {
  keys.fetchKeys()
})
</script>

<style scoped>
.keys-view {
  max-width: 64rem;
  margin: 0 auto;
}

.page-title {
  margin: 0 0 1.5rem;
  font-size: 1.5rem;
  font-weight: 700;
  color: #0f172a;
}

.generate-section {
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: #ffffff;
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.list-section {
  margin-bottom: 2rem;
}

.section-title {
  margin: 0 0 1rem;
  font-size: 1.125rem;
  font-weight: 600;
  color: #0f172a;
}

.generate-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.form-group label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #334155;
}

.form-group input {
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-size: 0.9375rem;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
  font-family: inherit;
}

.form-group input:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}

.error-message {
  margin: 0;
  color: #dc2626;
  font-size: 0.875rem;
}

.list-error {
  margin-bottom: 0.75rem;
}

.btn-primary {
  align-self: flex-start;
  padding: 0.625rem 1.25rem;
  border: none;
  border-radius: 0.375rem;
  background: #0f172a;
  color: #fff;
  font-size: 0.9375rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
}

.btn-primary:hover:not(:disabled) {
  background: #1e293b;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  padding: 0.625rem 1.25rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  background: #fff;
  color: #334155;
  font-size: 0.9375rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
}

.btn-secondary:hover {
  background: #f1f5f9;
}

.loading-text {
  padding: 1rem 0;
  color: #64748b;
  font-size: 0.9375rem;
}

.empty-state {
  padding: 2rem;
  text-align: center;
  color: #64748b;
  background: #f8fafc;
  border-radius: 0.5rem;
  border: 1px dashed #cbd5e1;
}

.empty-state p {
  margin: 0;
}

.empty-hint {
  margin-top: 0.25rem;
  font-size: 0.875rem;
}

.key-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.key-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  background: #ffffff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.07);
}

.key-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}

.key-name {
  font-weight: 600;
  color: #0f172a;
  font-size: 0.9375rem;
}

.key-public {
  font-size: 0.8125rem;
  color: #475569;
  font-family: monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.key-date {
  font-size: 0.8125rem;
  color: #64748b;
}

.btn-danger {
  flex-shrink: 0;
  padding: 0.375rem 0.75rem;
  border: 1px solid #fecaca;
  border-radius: 0.375rem;
  background: #fef2f2;
  color: #dc2626;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.btn-danger:hover:not(:disabled) {
  background: #fee2e2;
}

.btn-danger:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-content {
  background: #fff;
  border-radius: 0.75rem;
  padding: 1.5rem 2rem;
  max-width: 40rem;
  width: 90%;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.modal-title {
  margin: 0 0 0.5rem;
  font-size: 1.125rem;
  font-weight: 700;
  color: #0f172a;
}

.modal-notice {
  margin: 0 0 1rem;
  font-size: 0.875rem;
  color: #dc2626;
  font-weight: 500;
}

.modal-key-area {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-family: monospace;
  font-size: 0.8125rem;
  color: #0f172a;
  background: #f8fafc;
  resize: none;
  box-sizing: border-box;
}

.modal-actions {
  margin-top: 1rem;
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}
</style>
