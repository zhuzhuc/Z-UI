import { API_BASE, api } from './api'

export const defaultPanelSettings = {
  title: 'Z-UI',
  language: 'zh',
  theme: 'dark',
  refreshIntervalSec: 5,
  requireLogin: true,
  allowRegister: false,
  publicBaseUrl: '',
}

export function normalizePanelSettings(raw = {}) {
  const normalizedLanguage = String(raw.language || '').toLowerCase().startsWith('en') ? 'en' : 'zh'
  const rawTheme = String(raw.theme || '').toLowerCase()
  const normalizedTheme = rawTheme === 'light' || rawTheme === 'dark' ? rawTheme : defaultPanelSettings.theme
  const refreshIntervalSec = Math.max(Number(raw.refreshIntervalSec || defaultPanelSettings.refreshIntervalSec) || defaultPanelSettings.refreshIntervalSec, 1)

  return {
    ...defaultPanelSettings,
    ...raw,
    title: String(raw.title || defaultPanelSettings.title).trim() || defaultPanelSettings.title,
    language: normalizedLanguage,
    theme: normalizedTheme,
    refreshIntervalSec,
    requireLogin: raw.requireLogin !== false,
    allowRegister: !!raw.allowRegister,
    publicBaseUrl: String(raw.publicBaseUrl || '').trim(),
  }
}

export function applyPanelSettingsToDocument(settings, page = 'main') {
  const next = normalizePanelSettings(settings)
  document.documentElement.lang = next.language === 'en' ? 'en' : 'zh-CN'
  document.title = page === 'login' ? `${next.title} 登录` : `${next.title} 管理面板`
  return next
}

export async function loadPublicPanelSettings() {
  const response = await fetch(`${API_BASE}/public/settings`)
  if (!response.ok) {
    throw new Error('load public settings failed')
  }
  const data = await response.json()
  return normalizePanelSettings(data)
}

export async function loadPrivatePanelSettings() {
  const data = await api('/panel/settings')
  return normalizePanelSettings(data)
}
