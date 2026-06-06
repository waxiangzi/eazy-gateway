<template>
  <div class="app-layout">
    <nav v-if="auth.isLoggedIn" class="top-nav">
      <div class="nav-brand">{{ displayBrand }}</div>
      <div class="nav-links">
        <RouterLink to="/" class="nav-link">{{ t('nav.dashboard') }}</RouterLink>
        <RouterLink to="/hosts" class="nav-link">{{ t('nav.hosts') }}</RouterLink>
        <RouterLink to="/tunnels" class="nav-link">{{ t('nav.tunnels') }}</RouterLink>
        <RouterLink to="/keys" class="nav-link">{{ t('nav.keys') }}</RouterLink>
        <RouterLink to="/settings" class="nav-link">{{ t('nav.settings') }}</RouterLink>
        <button class="nav-link logout" @click="handleLogout">{{ t('nav.logout') }}</button>
        <select
          class="locale-select"
          :value="currentLocale"
          @change="(e) => setLocale(e.target.value)"
        >
          <option v-for="loc in locales" :key="loc.code" :value="loc.code">
            {{ loc.label }}
          </option>
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
import { i18n, setLocale, locales } from '../i18n'

import { useSettingsStore } from '../stores/settings.js'

const { t } = useI18n()
const auth = useAuthStore()
const settings = useSettingsStore()
const router = useRouter()

const currentLocale = computed(() => i18n.global.locale.value)
const displayBrand = computed(() => settings.appName || t('nav.brand'))

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

.locale-select {
  appearance: none;
  background: #1e293b;
  color: #f8fafc;
  border: 1px solid #334155;
  border-radius: 0.375rem;
  padding: 0.375rem 1.5rem 0.375rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  margin-left: 0.5rem;
  outline: none;
}

.locale-select:focus {
  border-color: #64748b;
}

.main-content {
  flex: 1;
  padding: 1.5rem;
  background: #f8fafc;
}
</style>
