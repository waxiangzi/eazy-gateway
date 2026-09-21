<template>
  <div class="hosts-view">
    <div class="hosts-header">
      <h1>{{ t('host.title') }}</h1>
      <RouterLink to="/hosts/new" class="btn-primary">{{ t('host.create') }}</RouterLink>
    </div>

    <div v-if="store.loading && store.list.length === 0" class="state-message">
      {{ t('host.loading') }}
    </div>
    <div v-else-if="store.error" class="state-message error">{{ store.error }}</div>
    <div v-else-if="store.list.length === 0" class="empty-state">
      <p>{{ t('host.empty') }}</p>
      <RouterLink to="/hosts/new" class="btn-primary">{{ t('host.create') }}</RouterLink>
    </div>
    <div v-else class="host-list">
      <div
        v-for="host in store.list"
        :key="host.id"
        class="host-card"
      >
        <div class="host-info">
          <div class="host-row">
            <span class="host-name">{{ host.name }}</span>
            <span class="host-address">{{ host.user }}@{{ host.host }}{{ host.port !== 22 ? ':' + host.port : '' }}</span>
          </div>
        </div>
        <div class="host-actions">
          <RouterLink :to="`/hosts/${host.id}`" class="btn-edit">{{ t('common.edit') }}</RouterLink>
          <button class="btn-delete" @click="handleDelete(host.id)">{{ t('common.delete') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useHostsStore } from '../stores/hosts.js'

const { t } = useI18n()
const store = useHostsStore()

async function handleDelete(id) {
  if (!confirm(t('host.deleteConfirm'))) return
  try {
    await store.deleteHost(id)
    store.removeHost(id)
  } catch (err) {
    alert(err.response?.data?.error || t('errors.generic'))
  }
}

onMounted(() => {
  store.fetchHosts()
})
</script>

<style scoped>
.hosts-view {
  max-width: 64rem;
  margin: 0 auto;
}

.hosts-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  /* 窄屏下标题与操作按钮换行，避免挤压重叠 */
  flex-wrap: wrap;
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

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), var(--shadow-sm);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.empty-state p {
  margin: 0 0 1.5rem;
  color: var(--text-secondary);
  font-size: 1rem;
}

.host-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.host-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  background: rgba(var(--card-rgb), 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.05), var(--shadow-sm);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.host-card:hover {
  border-color: rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 0 18px rgba(var(--accent-rgb), 0.1), var(--shadow-md);
}

.host-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  flex: 1;
  min-width: 0;
}

.host-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.host-name {
  font-weight: 600;
  font-size: 1rem;
  color: var(--text-primary);
}

.host-address {
  font-size: 0.875rem;
  color: var(--accent);
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
}

.host-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  margin-left: 1rem;
}

.btn-edit,
.btn-delete {
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
  background: rgba(var(--neutral-rgb), 0.1);
  color: var(--text-primary);
  border: 1px solid rgba(var(--slate-rgb), 0.2);
}

.btn-edit:hover {
  background: rgba(var(--accent-rgb), 0.15);
  border-color: rgba(var(--accent-rgb), 0.35);
  color: var(--accent);
}

.btn-delete {
  background: linear-gradient(135deg, rgba(var(--error-rgb),0.85), rgba(var(--danger-rgb),0.85));
  color: var(--accent-fg);
  border: 1px solid rgba(var(--error-rgb), 0.3);
  box-shadow: 0 0 6px rgba(var(--error-rgb), 0.15);
}

.btn-delete:hover {
  background: linear-gradient(135deg, rgba(var(--error-rgb),1), rgba(var(--danger-rgb),1));
  box-shadow: 0 0 12px rgba(var(--error-rgb), 0.25);
}

@media (max-width: 480px) {
  h1 {
    font-size: 1.25rem;
  }
}
</style>
