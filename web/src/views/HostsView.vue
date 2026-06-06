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
  margin-bottom: 1.5rem;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: #e2e8f0;
  letter-spacing: 0.02em;
  text-shadow: 0 0 12px rgba(56, 189, 248, 0.2);
}

.btn-primary {
  display: inline-block;
  padding: 0.5rem 1rem;
  background: linear-gradient(135deg, rgba(56,189,248,0.9), rgba(14,165,233,0.9));
  color: #fff;
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  border: 1px solid rgba(56, 189, 248, 0.35);
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.15);
  transition: all 0.2s;
}

.btn-primary:hover {
  background: linear-gradient(135deg, rgba(56,189,248,1), rgba(14,165,233,1));
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.3);
  transform: translateY(-1px);
}

.state-message {
  padding: 2rem;
  text-align: center;
  color: #64748b;
  font-size: 1rem;
}

.state-message.error {
  color: #f87171;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  background: rgba(16, 24, 48, 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(56, 189, 248, 0.15);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.empty-state p {
  margin: 0 0 1.5rem;
  color: #94a3b8;
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
  background: rgba(16, 24, 48, 0.65);
  border-radius: 0.5rem;
  border: 1px solid rgba(56, 189, 248, 0.15);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.05), 0 4px 8px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.host-card:hover {
  border-color: rgba(56, 189, 248, 0.3);
  box-shadow: 0 0 18px rgba(56, 189, 248, 0.1), 0 4px 12px rgba(0, 0, 0, 0.4);
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
  color: #e2e8f0;
}

.host-address {
  font-size: 0.875rem;
  color: #38bdf8;
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
  background: rgba(226, 232, 240, 0.1);
  color: #e2e8f0;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.btn-edit:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.35);
  color: #38bdf8;
}

.btn-delete {
  background: linear-gradient(135deg, rgba(248,113,113,0.85), rgba(239,68,68,0.85));
  color: #fff;
  border: 1px solid rgba(248, 113, 113, 0.3);
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.15);
}

.btn-delete:hover {
  background: linear-gradient(135deg, rgba(248,113,113,1), rgba(239,68,68,1));
  box-shadow: 0 0 12px rgba(248, 113, 113, 0.25);
}
</style>
