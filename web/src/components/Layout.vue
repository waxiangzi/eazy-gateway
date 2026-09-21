<template>
  <div class="app-layout">
    <nav v-if="auth.isLoggedIn" class="top-nav">
      <RouterLink to="/" class="nav-brand">{{ displayBrand }}</RouterLink>

      <div class="nav-links">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="nav-link"
        >
          {{ item.label }}
        </RouterLink>
      </div>

      <div class="nav-actions">
        <select
          class="locale-select"
          :value="currentLocale"
          @change="(e) => setLocale(e.target.value)"
        >
          <option v-for="loc in locales" :key="loc.code" :value="loc.code">
            {{ loc.label }}
          </option>
        </select>
        <button class="nav-link logout" @click="handleLogout">{{ t('nav.logout') }}</button>
      </div>

      <!-- 窄屏：导航收进抽屉 -->
      <button
        ref="burgerRef"
        class="nav-burger"
        type="button"
        aria-controls="nav-drawer"
        :aria-expanded="drawerOpen ? 'true' : 'false'"
        :aria-label="t('nav.menu')"
        @click="openDrawer"
      >
        <span class="burger-bar" />
        <span class="burger-bar" />
        <span class="burger-bar" />
      </button>
    </nav>

    <main class="main-content">
      <slot />
    </main>

    <template v-if="auth.isLoggedIn">
      <div v-if="drawerOpen" class="nav-scrim" @click="closeDrawer" />
      <aside
        id="nav-drawer"
        ref="drawerRef"
        class="nav-drawer"
        :class="{ open: drawerOpen }"
      >
        <div class="drawer-head">
          <span class="drawer-brand">{{ displayBrand }}</span>
          <button
            ref="closeRef"
            class="drawer-close"
            type="button"
            :aria-label="t('common.close')"
            @click="closeDrawer"
          >
            ✕
          </button>
        </div>

        <nav class="drawer-nav">
          <RouterLink
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="drawer-link"
            @click="closeDrawer"
          >
            {{ item.label }}
          </RouterLink>
        </nav>

        <div class="drawer-foot">
          <select
            class="locale-select block"
            :value="currentLocale"
            @change="(e) => setLocale(e.target.value)"
          >
            <option v-for="loc in locales" :key="loc.code" :value="loc.code">
              {{ loc.label }}
            </option>
          </select>
          <button class="drawer-logout" @click="handleLogout">{{ t('nav.logout') }}</button>
        </div>
      </aside>
    </template>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { i18n, setLocale, locales } from '../i18n'

import { useSettingsStore } from '../stores/settings.js'

const { t } = useI18n()
const auth = useAuthStore()
const settings = useSettingsStore()
const router = useRouter()
const route = useRoute()

const currentLocale = computed(() => i18n.global.locale.value)
const displayBrand = computed(() => settings.appName || t('nav.brand'))
const navItems = computed(() => [
  { to: '/', label: t('nav.dashboard') },
  { to: '/hosts', label: t('nav.hosts') },
  { to: '/tunnels', label: t('nav.tunnels') },
  { to: '/keys', label: t('nav.keys') },
  { to: '/settings', label: t('nav.settings') },
])

const drawerOpen = ref(false)
const drawerRef = ref(null)
const closeRef = ref(null)

function openDrawer() {
  drawerOpen.value = true
  nextTick(() => closeRef.value?.focus())
}

function closeDrawer() {
  drawerOpen.value = false
}

// 抽屉打开时锁定背景滚动
watch(drawerOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})
// 路由变化（含浏览器前进/后退）时收起抽屉
watch(() => route.fullPath, closeDrawer)

function onKeydown(e) {
  if (e.key === 'Escape' && drawerOpen.value) closeDrawer()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  document.body.style.overflow = ''
})

async function handleLogout() {
  closeDrawer()
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
  gap: 1.5rem;
  padding: 0 1.5rem;
  height: 3.5rem;
  background: rgba(var(--card-rgb), 0.85);
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
  backdrop-filter: var(--glass);
  -webkit-backdrop-filter: var(--glass);
  position: sticky;
  top: 0;
  z-index: 30;
}

.nav-brand {
  font-weight: 700;
  font-size: 1.125rem;
  letter-spacing: 0.04em;
  color: var(--accent);
  text-shadow: var(--title-glow);
  text-decoration: none;
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-left: auto;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.nav-link {
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 0.875rem;
  font-family: inherit;
  transition: background 0.2s, color 0.2s, box-shadow 0.2s;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  cursor: pointer;
}

.nav-link:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-link.router-link-active {
  background: var(--bg-active);
  color: var(--accent);
  border-bottom-color: var(--accent);
}

.nav-link.logout {
  margin-left: 0.25rem;
  color: var(--error);
}

.nav-link.logout:hover {
  background: var(--error-bg);
  color: var(--error-soft);
}

.locale-select {
  appearance: none;
  background: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  padding: 0.375rem 1.5rem 0.375rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  outline: none;
  font-family: inherit;
}

.locale-select:focus {
  border-color: var(--border-focus);
  box-shadow: var(--accent-glow);
}

.locale-select.block {
  width: 100%;
}

/* 汉堡按钮：仅窄屏出现 */
.nav-burger {
  display: none;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  margin-left: auto;
  width: 2.5rem;
  height: 2.25rem;
  padding: 0 0.625rem;
  background: transparent;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.nav-burger:hover {
  background: var(--bg-hover);
}

.burger-bar {
  display: block;
  height: 2px;
  width: 100%;
  border-radius: 1px;
  background: var(--text-secondary);
}

.main-content {
  flex: 1;
  padding: 1.5rem;
  background: var(--bg-page);
}

/* 抽屉：桌面端不渲染显示，窄屏才启用 */
.nav-scrim,
.nav-drawer {
  display: none;
}

@media (max-width: 820px) {
  .top-nav {
    gap: 0.75rem;
    padding: 0 1rem;
  }

  .nav-links,
  .nav-actions {
    display: none;
  }

  .nav-burger {
    display: flex;
  }

  .main-content {
    padding: 1rem;
  }

  .nav-scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 40;
    background: var(--overlay);
  }

  .nav-drawer {
    display: flex;
    flex-direction: column;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 50;
    width: min(280px, 82vw);
    background: var(--bg-card-solid);
    border-right: 1px solid var(--border-color);
    transform: translateX(-100%);
    transition: transform 0.22s ease;
    overflow-y: auto;
  }

  .nav-drawer.open {
    transform: translateX(0);
    box-shadow: var(--shadow-md);
  }

  .drawer-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 1rem;
    border-bottom: 1px solid var(--border-color);
  }

  .drawer-brand {
    font-weight: 700;
    color: var(--accent);
  }

  .drawer-close {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 1.125rem;
    line-height: 1;
    cursor: pointer;
    padding: 0.25rem;
  }

  .drawer-nav {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
    padding: 0.5rem;
  }

  .drawer-link {
    padding: 0.75rem 0.875rem;
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    text-decoration: none;
    font-size: 0.9375rem;
  }

  .drawer-link:hover {
    background: var(--bg-hover);
  }

  .drawer-link.router-link-active {
    background: var(--bg-active);
    color: var(--accent);
  }

  .drawer-foot {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 1rem;
    border-top: 1px solid var(--border-color);
  }

  .drawer-logout {
    width: 100%;
    padding: 0.625rem;
    border-radius: var(--radius-md);
    background: var(--error-bg);
    border: 1px solid transparent;
    color: var(--error);
    font-family: inherit;
    font-size: 0.9375rem;
    cursor: pointer;
  }
}
</style>
