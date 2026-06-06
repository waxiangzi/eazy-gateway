import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth.js'
import { useSettingsStore } from './stores/settings.js'
import { i18n } from './i18n'
import './styles/tech-theme.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)

const auth = useAuthStore()
const settings = useSettingsStore()

;(async () => {
  await auth.checkAuth()
  await settings.fetchSettings()
  app.mount('#app')
})()
