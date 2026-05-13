import React from 'react'
import { createRoot } from 'react-dom/client'
import {
  Cpu,
  DownloadCloud,
  Activity,
  Database,
} from 'lucide-react'
import ErrorBoundary from './components/ErrorBoundary'
import { ToastProvider } from './components/ToastProvider'
import Sidebar from './components/Sidebar'
import Topbar from './components/Topbar'
import StatCard from './components/StatCard'
import DashboardCharts from './components/DashboardCharts'
import { api } from './shared/api'
import { useT } from './shared/i18n'
import { applyPanelSettingsToDocument, loadPrivatePanelSettings, normalizePanelSettings } from './shared/panelSettings'
import InboundsModule from './modules/InboundsModule'
import SubscriptionModule from './modules/SubscriptionModule'
import ToolsModule from './modules/ToolsModule'
import XrayConfigModule from './modules/XrayConfigModule'
import LogsModule from './modules/LogsModule'
import SettingsModule from './modules/SettingsModule'
import './styles/dashboard.css'

function App() {
  const savedTheme = localStorage.getItem('zui.theme')
  const savedLang = localStorage.getItem('zui.lang')
  const systemPrefersDark = typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches
  const defaultTheme = savedTheme || (systemPrefersDark ? 'dark' : 'light')

  const [tab, setTab] = React.useState('dashboard')
  const [username, setUsername] = React.useState('admin')
  const [panelSettings, setPanelSettings] = React.useState(normalizePanelSettings())
  const [summary, setSummary] = React.useState(null)
  const [history, setHistory] = React.useState([])
  const [protocolData, setProtocolData] = React.useState([])
  const [theme, setTheme] = React.useState(defaultTheme)
  const [lang, setLang] = React.useState(savedLang || 'zh')
  const [searchValue, setSearchValue] = React.useState('')
  const [searchTarget, setSearchTarget] = React.useState('inbounds')
  const [inboundsQuery, setInboundsQuery] = React.useState('')
  const [logsQuery, setLogsQuery] = React.useState('')
  const [userRole, setUserRole] = React.useState('admin')
  const [currentUser, setCurrentUser] = React.useState(null)
  const t = useT(lang)

  React.useEffect(() => {
    document.body.dataset.theme = theme
    localStorage.setItem('zui.theme', theme)
  }, [theme])

  React.useEffect(() => {
    localStorage.setItem('zui.lang', lang)
  }, [lang])

  React.useEffect(() => {
    applyPanelSettingsToDocument({ ...panelSettings, language: lang }, 'main')
  }, [panelSettings, lang])

  const refreshCurrentUser = React.useCallback(async () => {
    try {
      const me = await api('/auth/me')
      setUsername(me.username || 'admin')
      setUserRole(me.role || 'admin')
      setCurrentUser(me)
    } catch {
      window.location.href = 'login.html'
    }
  }, [])

  React.useEffect(() => {
    let canceled = false
    const load = async () => {
      try {
        const [me, settings] = await Promise.all([api('/auth/me'), loadPrivatePanelSettings()])
        if (canceled) return
        setUsername(me.username || 'admin')
        setUserRole(me.role || 'admin')
        setCurrentUser(me)
        setPanelSettings(settings)
        if (!savedTheme) setTheme(settings.theme)
        if (!savedLang) setLang(settings.language)
      } catch (e) {
        window.location.href = 'login.html'
      }
    }
    load()
    return () => {
      canceled = true
    }
  }, [savedLang, savedTheme])

  React.useEffect(() => {
    let canceled = false
    const loadSummary = async () => {
      try {
        const res = await api('/dashboard/summary')
        if (!canceled) {
          setSummary(res)
          const cpu = Number(res?.system?.cpuPercent || 0)
          const disk = Number(res?.system?.diskPercent || 0)
          const trafficRatio = Number(res?.trafficTotalGB || 0) > 0 ? (Number(res?.trafficUsedGB || 0) / Number(res.trafficTotalGB)) * 100 : 0
          const point = {
            name: new Date().toLocaleTimeString(),
            up: Number.isFinite(cpu) ? Number(cpu.toFixed(1)) : 0,
            down: Number.isFinite(disk) ? Number(disk.toFixed(1)) : Number(trafficRatio.toFixed(1)),
          }
          setHistory((prev) => [...prev.slice(-23), point])
        }
      } catch {}
    }

    const loadProtocols = async () => {
      try {
        const res = await api('/inbounds')
        if (canceled) return
        const map = {}
        ;(res.items || []).forEach((item) => {
          const key = String(item.protocol || 'unknown').toLowerCase()
          map[key] = (map[key] || 0) + 1
        })
        setProtocolData(Object.entries(map).map(([name, value]) => ({ name, value })))
      } catch {}
    }

    loadSummary()
    loadProtocols()
    const timer = window.setInterval(loadSummary, panelSettings.refreshIntervalSec * 1000)
    const protocolTimer = window.setInterval(loadProtocols, 10000)
    return () => {
      canceled = true
      window.clearInterval(timer)
      window.clearInterval(protocolTimer)
    }
  }, [panelSettings.refreshIntervalSec])

  const cardItems = [
    { title: t('cpuStatus'), value: `${Number(summary?.system?.cpuPercent || 0).toFixed(1)} %`, icon: Cpu, colorClass: 'c-indigo' },
    { title: t('inboundTotal'), value: String(summary?.inboundTotal || 0), icon: DownloadCloud, colorClass: 'c-emerald' },
    { title: t('inboundEnabled'), value: String(summary?.inboundEnabled || 0), icon: Database, colorClass: 'c-blue' },
    { title: t('trafficUsage'), value: `${summary?.trafficUsedGB || 0} / ${summary?.trafficTotalGB || 0} GB`, icon: Activity, colorClass: 'c-pink' },
  ]

  function logout() {
    api('/auth/logout', { method: 'POST' })
      .catch(() => {})
      .finally(() => {
        window.location.href = 'login.html'
      })
  }

  function toggleTheme() {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }

  function handleSearchSubmit(event) {
    event.preventDefault()
    if (searchTarget === 'logs') {
      setLogsQuery(searchValue)
      setTab('logs')
      return
    }
    setInboundsQuery(searchValue)
    setTab('inbounds')
  }

  function handleSettingsChange(nextSettings) {
    const normalized = normalizePanelSettings(nextSettings)
    setPanelSettings(normalized)
    setLang(normalized.language)
    setTheme(normalized.theme)
  }

  return (
    <div className="dashboard-shell">
      <Sidebar tab={tab} onTabChange={setTab} onLogout={logout} t={t} brandTitle={panelSettings.title} />

      <main className="main">
        <Topbar
          username={username}
          role={userRole}
          onToggleTheme={toggleTheme}
          theme={theme}
          lang={lang}
          onLangChange={setLang}
          t={t}
          searchValue={searchValue}
          searchTarget={searchTarget}
          onSearchChange={setSearchValue}
          onSearchTargetChange={setSearchTarget}
          onSearchSubmit={handleSearchSubmit}
        />
        <section className="main-inner">
          <div className="title-row">
            <div>
              <h1>{tab === 'dashboard' ? panelSettings.title : t(tab, tab === 'dashboard' ? '控制面板' : tab)}</h1>
              <p>{tab === 'dashboard' ? t('welcome') : t('moduleReady')}</p>
            </div>
          </div>

          {tab === 'dashboard' ? (
            <>
              <div className="stat-grid">
                {cardItems.map((item) => (
                  <StatCard key={item.title} {...item} />
                ))}
              </div>
              <DashboardCharts history={history} protocolData={protocolData} lang={lang} theme={theme} />
            </>
          ) : null}

          {tab === 'inbounds' ? <InboundsModule t={t} lang={lang} externalQuery={inboundsQuery} /> : null}
          {tab === 'subscription' ? <SubscriptionModule t={t} lang={lang} /> : null}
          {tab === 'tools' ? <ToolsModule t={t} lang={lang} /> : null}
          {tab === 'xray-config' ? <XrayConfigModule t={t} lang={lang} /> : null}
          {tab === 'logs' ? <LogsModule t={t} lang={lang} externalQuery={logsQuery} /> : null}
          {tab === 'settings' ? (
            <SettingsModule
              t={t}
              lang={lang}
              onSettingsChange={handleSettingsChange}
              currentUser={currentUser}
              currentUserRole={userRole}
              onUsernameChange={setUsername}
              onProfileRefresh={refreshCurrentUser}
            />
          ) : null}
        </section>
      </main>
    </div>
  )
}

createRoot(document.getElementById('root')).render(
  <ErrorBoundary>
    <ToastProvider>
      <App />
    </ToastProvider>
  </ErrorBoundary>
)
