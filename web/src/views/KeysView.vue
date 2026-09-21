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
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(var(--accent-rgb), 0.2);
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
  color: var(--text-primary);
}

.error-message {
  margin: 0;
  color: var(--error);
  font-size: 0.875rem;
}

.list-error {
  margin-bottom: 0.75rem;
}

.loading-text {
  padding: 1rem 0;
  color: var(--text-muted);
  font-size: 0.9375rem;
}

.empty-state {
  padding: 2rem;
  text-align: center;
  color: var(--text-secondary);
  background: rgba(var(--card-rgb), 0.5);
  border-radius: 0.5rem;
  border: 1px dashed rgba(var(--accent-rgb), 0.2);
}

.empty-state p {
  margin: 0;
}

.empty-state.error {
  background: rgba(var(--error-rgb), 0.08);
  border-color: rgba(var(--error-rgb), 0.25);
  color: var(--error);
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
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.key-card:hover {
  border-color: rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 0 18px rgba(var(--accent-rgb), 0.1), 0 4px 12px rgba(0, 0, 0, 0.4);
}

.key-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}

.key-name {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.9375rem;
}

.key-public {
  font-size: 0.8125rem;
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.key-date {
  font-size: 0.8125rem;
  color: var(--text-muted);
}

.key-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  margin-left: 1rem;
}

.btn-copy {
  padding: 0.375rem 0.75rem;
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  border-radius: 0.375rem;
  background: rgba(var(--page-rgb), 0.6);
  color: var(--text-secondary);
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.2s;
}

.btn-copy:hover {
  background: rgba(var(--accent-rgb), 0.15);
  border-color: rgba(var(--accent-rgb), 0.4);
  color: var(--accent);
}

.btn-danger {
  padding: 0.375rem 0.75rem;
  border: 1px solid rgba(var(--error-rgb), 0.25);
  border-radius: 0.375rem;
  background: rgba(var(--error-rgb), 0.08);
  color: var(--error);
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.2s;
}

.btn-danger:hover:not(:disabled) {
  background: rgba(var(--error-rgb), 0.15);
  border-color: rgba(var(--error-rgb), 0.4);
  box-shadow: 0 0 8px rgba(var(--error-rgb), 0.15);
}

.btn-danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
