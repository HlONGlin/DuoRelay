import { ref } from 'vue'
import { defineStore } from 'pinia'

export type Theme = 'light' | 'dark'
export type Language = 'zh-CN' | 'en'
const storageKey = 'duorelay-theme'
const languageKey = 'duorelay-language'

export function readLanguage(): Language {
  try { return localStorage.getItem(languageKey) === 'en' ? 'en' : 'zh-CN' }
  catch { return 'zh-CN' }
}

export function localize(zh: string, en: string): string {
  return readLanguage() === 'en' ? en : zh
}

export function readTheme(): Theme {
  try { return localStorage.getItem(storageKey) === 'dark' ? 'dark' : 'light' }
  catch { return 'light' }
}

export function initializeTheme() {
  document.documentElement.dataset.theme = readTheme()
  document.documentElement.lang = readLanguage()
}

export const useAppearanceStore = defineStore('appearance', () => {
  const theme = ref<Theme>(readTheme())
  const language = ref<Language>(readLanguage())
  function setLanguage(next: Language) {
    language.value = next
    document.documentElement.lang = next
    try { localStorage.setItem(languageKey, next) } catch { /* Language still works without storage. */ }
  }
  function setTheme(next: Theme) {
    theme.value = next
    document.documentElement.dataset.theme = next
    try { localStorage.setItem(storageKey, next) } catch { /* Theme still works without storage. */ }
  }
  function toggleTheme() { setTheme(theme.value === 'dark' ? 'light' : 'dark') }
  return { theme, setTheme, toggleTheme, language, setLanguage }
})
