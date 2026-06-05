import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import zh from './locales/zh.json'

function detectLocale() {
  const saved = localStorage.getItem('locale')
  if (saved && ['en', 'zh'].includes(saved)) {
    return saved
  }
  const browser = navigator.language || navigator.userLanguage
  if (browser && browser.toLowerCase().startsWith('zh')) {
    return 'zh'
  }
  return 'en'
}

export const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: 'en',
  legacy: false,
  messages: { en, zh },
})

export function setLocale(locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('locale', locale)
}
