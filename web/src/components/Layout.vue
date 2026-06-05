<template>
  <div class="app-layout">
    <nav v-if="auth.isLoggedIn" class="top-nav">
      <div class="nav-brand">{{ t('nav.brand') }}</div>
      <div class="nav-links">
        <RouterLink to="/" class="nav-link">{{ t('nav.dashboard') }}</RouterLink>
        <RouterLink to="/hosts" class="nav-link">{{ t('nav.hosts') }}</RouterLink>
        <RouterLink to="/tunnels" class="nav-link">{{ t('nav.tunnels') }}</RouterLink>
        <RouterLink to="/keys" class="nav-link">{{ t('nav.keys') }}</RouterLink>
        <RouterLink to="/settings" class="nav-link">{{ t('nav.settings') }}</RouterLink>
        <button class="nav-link logout" @click="handleLogout">{{ t('nav.logout') }}</button>
        <select class="lang-select" :value="currentLocale" @change="onLangChange">
          <option value="en">English</option>
          <option value="zh">中文</option>
        </select>
      </div>
    </nav>
    <main class="main-content">
      <slot />
    </main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth.js'
import { useRouter } from 'vue-router'
import { i18n, setLocale } from '../i18n'

const { t } = useI18n()
const auth = useAuthStore()
const router = useRouter()

const currentLocale = computed(() => i18n.global.locale.value)

function onLangChange(e) {
  setLocale(e.target.value)
}

async function handleLogout() {
  await auth.logout()
  router.push({ name: 'Login' })
}
</script>

<style scoped>
.app-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.top-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 1.5rem;
  height: 3.5rem;
  background: #0f172a;
  color: #e2e8f0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.nav-brand {
  font-weight: 700;
  font-size: 1.125rem;
  letter-spacing: 0.02em;
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.nav-link {
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  color: #cbd5e1;
  text-decoration: none;
  font-size: 0.875rem;
  transition: background 0.15s, color 0.15s;
  border: none;
  background: transparent;
  cursor: pointer;
}

.nav-link:hover,
.nav-link.router-link-active {
  background: #1e293b;
  color: #f8fafc;
}

.nav-link.logout {
  margin-left: 0.5rem;
  color: #f87171;
}

.nav-link.logout:hover {
  background: #450a0a;
  color: #fca5a5;
}

.lang-select {
  margin-left: 0.5rem;
  padding: 0.35rem 0.5rem;
  border-radius: 0.375rem;
  border: 1px solid #334155;
  background: #1e293b;
  color: #e2e8f0;
  font-size: 0.875rem;
  cursor: pointer;
  outline: none;
}

.lang-select:focus {
  border-color: #475569;
}

.main-content {
  flex: 1;
  padding: 1.5rem;
  background: #f8fafc;
}
</style>
