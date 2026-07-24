import { defineStore } from 'pinia'
import { ref } from 'vue'
import { settingsApi } from '@/api'

export const useAppStore = defineStore('app', () => {
  const sidebarCollapsed = ref(false)
  const settings = ref<Record<string, string>>({})
  const theme = ref<'light' | 'dark'>('light')
  const language = ref('zh')
  const loading = ref(false)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
    localStorage.setItem('sidebar_collapsed', String(sidebarCollapsed.value))
  }

  async function fetchPublicSettings() {
    try {
      const res = await settingsApi.public()
      settings.value = res.data
    } catch {
      // Ignore.
    }
  }

  function setTheme(t: 'light' | 'dark') {
    theme.value = t
    document.documentElement.setAttribute('data-theme', t)
    document.documentElement.classList.toggle('dark', t === 'dark')
    localStorage.setItem('theme', t)
  }

  function toggleTheme() {
    setTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  // Initialize from localStorage.
  const savedCollapsed = localStorage.getItem('sidebar_collapsed')
  if (savedCollapsed === 'true') {
    sidebarCollapsed.value = true
  }
  const savedTheme = localStorage.getItem('theme') as 'light' | 'dark'
  if (savedTheme) {
    theme.value = savedTheme
    document.documentElement.setAttribute('data-theme', savedTheme)
    if (savedTheme === 'dark') {
      document.documentElement.classList.add('dark')
    }
  }

  return {
    sidebarCollapsed, settings, theme, language, loading,
    toggleSidebar, fetchPublicSettings, setTheme, toggleTheme,
  }
})
