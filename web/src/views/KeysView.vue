<template>
  <div class="keys-view">
    <h1 class="page-title">SSH Keys</h1>

    <section class="upload-section">
      <h2 class="section-title">Upload Key</h2>
      <form class="upload-form" @submit.prevent="handleCreate">
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
        <div class="form-group">
          <label for="key-pem">Private Key (PEM)</label>
          <textarea
            id="key-pem"
            v-model="form.pem"
            rows="8"
            placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
            required
          ></textarea>
        </div>
        <p v-if="keys.uploadError" class="error-message">{{ keys.uploadError }}</p>
        <button type="submit" class="btn-primary" :disabled="keys.isUploading">
          {{ keys.isUploading ? 'Uploading…' : 'Upload Key' }}
        </button>
      </form>
    </section>

    <section class="list-section">
      <h2 class="section-title">Your Keys</h2>

      <div v-if="keys.isLoading" class="loading-text">Loading keys…</div>

      <div v-else-if="!keys.hasKeys" class="empty-state">
        <p>No SSH keys yet.</p>
        <p class="empty-hint">Upload a key above to get started.</p>
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
  </div>
</template>

<script setup>
import { reactive, onMounted } from 'vue'
import { useKeysStore } from '../stores/keys.js'

const keys = useKeysStore()

const form = reactive({
  name: '',
  pem: '',
})

function formatDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString()
}

async function handleCreate() {
  const ok = await keys.createKey({
    name: form.name.trim(),
    pem: form.pem.trim(),
  })
  if (ok) {
    form.name = ''
    form.pem = ''
    await keys.fetchKeys()
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

.upload-section {
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

.upload-form {
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

.form-group input,
.form-group textarea {
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  font-size: 0.9375rem;
  color: #0f172a;
  background: #fff;
  box-sizing: border-box;
  font-family: inherit;
}

.form-group textarea {
  resize: vertical;
  min-height: 6rem;
}

.form-group input:focus,
.form-group textarea:focus {
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
}

.key-name {
  font-weight: 600;
  color: #0f172a;
  font-size: 0.9375rem;
}

.key-date {
  font-size: 0.8125rem;
  color: #64748b;
}

.btn-danger {
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
</style>
