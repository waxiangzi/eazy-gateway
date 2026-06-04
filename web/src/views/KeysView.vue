<template>
  <div class="keys-view">
    <h1 class="page-title">SSH Keys</h1>

    <section class="list-section">
      <h2 class="section-title">Your Keys</h2>

      <div v-if="keys.isLoading" class="loading-text">Loading keys…</div>

      <div v-else-if="!keys.hasKeys" class="empty-state">
        <p>No SSH keys found.</p>
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
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useKeysStore } from '../stores/keys.js'

const keys = useKeysStore()

function formatDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString()
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

.list-section {
  margin-bottom: 2rem;
}

.section-title {
  margin: 0 0 1rem;
  font-size: 1.125rem;
  font-weight: 600;
  color: #0f172a;
}

.error-message {
  margin: 0;
  color: #dc2626;
  font-size: 0.875rem;
}

.list-error {
  margin-bottom: 0.75rem;
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
</style>
