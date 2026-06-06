import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import zh from './locales/zh.json'

export const locales = [
  { code: 'en', label: 'English' },
  { code: 'zh', label: '简体中文' },
]

function detectLocale() {
  const saved = localStorage.getItem('locale')
  if (saved && locales.some((l) => l.code === saved)) {
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
