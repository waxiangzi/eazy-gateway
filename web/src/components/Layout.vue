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
  background: rgba(var(--page-rgb), 0.85);
  color: var(--text-primary);
  border-bottom: 1px solid rgba(var(--accent-rgb), 0.15);
  box-shadow: 0 0 20px rgba(var(--accent-rgb), 0.08), 0 4px 12px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(12px) saturate(140%);
  -webkit-backdrop-filter: blur(12px) saturate(140%);
}

.nav-brand {
  font-weight: 700;
  font-size: 1.125rem;
  letter-spacing: 0.04em;
  color: var(--accent);
  text-shadow: 0 0 12px rgba(var(--accent-rgb), 0.35);
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.nav-link {
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 0.875rem;
  transition: all 0.2s;
  border: none;
  background: transparent;
  cursor: pointer;
}

.nav-link:hover,
.nav-link.router-link-active {
  background: rgba(var(--accent-rgb), 0.1);
  color: var(--accent);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.1);
}

.nav-link.logout {
  margin-left: 0.5rem;
  color: var(--error);
}

.nav-link.logout:hover {
  background: rgba(var(--error-rgb), 0.1);
  color: var(--error-soft);
  box-shadow: 0 0 8px rgba(var(--error-rgb), 0.1);
}

.locale-select {
  appearance: none;
  background: rgba(var(--page-rgb), 0.8);
  color: var(--text-primary);
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  border-radius: 0.375rem;
  padding: 0.375rem 1.5rem 0.375rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  margin-left: 0.5rem;
  outline: none;
  font-family: inherit;
}

.locale-select:focus {
  border-color: rgba(var(--accent-rgb), 0.5);
  box-shadow: 0 0 8px rgba(var(--accent-rgb), 0.15);
}

.main-content {
  flex: 1;
  padding: 1.5rem;
  background: var(--bg-page);
}
</style>
