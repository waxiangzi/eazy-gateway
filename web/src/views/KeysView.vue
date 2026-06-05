<template>
  <div class="keys-view">
    <h1 class="page-title">{{ t('key.title') }}</h1>

    <section class="list-section">
      <h2 class="section-title">
        {{ t('key.listTitle') }}
        <TooltipIcon :text="t('key.tooltip')" />
      </h2>

      <div v-if="keys.isLoading" class="loading-text">{{ t('key.loading') }}</div>

      <div v-else-if="keys.error" class="empty-state error">
        <p>{{ keys.error }}</p>
      </div>

      <div v-else-if="!keys.hasKeys" class="empty-state">
        <p>{{ t('key.empty') }}</p>
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
          <div class="key-actions">
            <button
              v-if="key.publicKey"
              class="btn-copy"
              @click="copyPublicKey(key.publicKey)"
            >
              {{ t('common.copy') }}
            </button>
            <button
              class="btn-danger"
              :disabled="keys.isDeleting"
              @click="handleDelete(key.id, key.name)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import TooltipIcon from '../components/TooltipIcon.vue'
import { useKeysStore } from '../stores/keys.js'

const { t } = useI18n()
const keys = useKeysStore()

function formatDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString()
}

async function handleDelete(id, name) {
  const confirmed = window.confirm(t('key.deleteConfirm', { name }))
  if (!confirmed) return
  const ok = await keys.deleteKey(id)
  if (ok) {
    await keys.fetchKeys()
  }
}

async function copyPublicKey(text) {
  const value = text.trim()
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value)
    } else {
      // Fallback for non-secure contexts (http:// LAN IPs)
      const ta = document.createElement('textarea')
      ta.value = value
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      if (!ok) throw new Error('execCommand copy failed')
    }
    alert(t('key.copySuccess'))
  } catch {
    alert(t('key.copyError'))
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
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
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

.empty-state.error {
  background: #fef2f2;
  border-color: #fecaca;
  color: #dc2626;
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

.key-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  margin-left: 1rem;
}

.btn-copy {
  padding: 0.375rem 0.75rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.375rem;
  background: #f8fafc;
  color: #334155;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.btn-copy:hover {
  background: #e2e8f0;
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
