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
  color: #0f172a;
}

.btn-primary {
  display: inline-block;
  padding: 0.5rem 1rem;
  background: #0f172a;
  color: #fff;
  text-decoration: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  transition: background 0.15s;
}

.btn-primary:hover {
  background: #1e293b;
}

.state-message {
  padding: 2rem;
  text-align: center;
  color: #64748b;
  font-size: 1rem;
}

.state-message.error {
  color: #dc2626;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  background: #fff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.empty-state p {
  margin: 0 0 1.5rem;
  color: #64748b;
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
  background: #fff;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
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
  color: #0f172a;
}

.host-address {
  font-size: 0.875rem;
  color: #475569;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
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
  transition: background 0.15s, opacity 0.15s;
  text-decoration: none;
}

.btn-edit {
  background: #e2e8f0;
  color: #0f172a;
}

.btn-edit:hover {
  background: #cbd5e1;
}

.btn-delete {
  background: #dc2626;
  color: #fff;
}

.btn-delete:hover {
  background: #b91c1c;
}
</style>
