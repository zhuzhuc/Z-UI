import React from 'react'
import { createRoot } from 'react-dom/client'
import {
  Cpu,
  DownloadCloud,
  Activity,
  Database,
} from 'lucide-react'
import Sidebar from './components/Sidebar'
import Topbar from './components/Topbar'
import StatCard from './components/StatCard'
import DashboardCharts from './components/DashboardCharts'
import { api } from './shared/api'
import { useT } from './shared/i18n'
import InboundsModule from './modules/InboundsModule'
import SubscriptionModule from './modules/SubscriptionModule'
import ToolsModule from './modules/ToolsModule'
import LogsModule from './modules/LogsModule'
import SettingsModule from './modules/SettingsModule'
import './styles/dashboard.css'

function App() {
  const [tab, setTab] = React.useState('dashboard')
  const [username, setUsername] = React.useState('admin')
  const [summary, setSummary] = React.useState(null)
  const [history, setHistory] = React.useState([])
  const [protocolData, setProtocolData] = React.useState([])
  const [theme, setTheme] = React.useState(localStorage.getItem('zui.theme') || 'dark')
  const [lang, setLang] = React.useState(localStorage.getItem('zui.lang') || 'zh')
  const t = useT(lang)

  React.useEffect(() => {
    document.body.dataset.theme = theme
    localStorage.setItem('zui.theme', theme)
  }, [theme])

  React.useEffect(() => {
    localStorage.setItem('zui.lang', lang)
  }, [lang])

  React.useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      window.location.href = 'login.html'
      return
    }
    api('/auth/me').then((me) => setUsername(me.username || 'admin')).catch(() => {
      localStorage.removeItem('token')
      window.location.href = 'login.html'
    })

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
    const timer = window.setInterval(loadSummary, 1000)
    const protocolTimer = window.setInterval(loadProtocols, 10000)
    return () => {
      canceled = true
      window.clearInterval(timer)
      window.clearInterval(protocolTimer)
    }
  }, [])

  const cardItems = [
    { title: t('cpuStatus', 'CPU 状态'), value: `${Number(summary?.system?.cpuPercent || 0).toFixed(1)} %`, icon: Cpu, colorClass: 'c-indigo' },
    { title: t('inboundTotal', '入站总数'), value: String(summary?.inboundTotal || 0), icon: DownloadCloud, colorClass: 'c-emerald' },
    { title: t('inboundEnabled', '启用入站'), value: String(summary?.inboundEnabled || 0), icon: Database, colorClass: 'c-blue' },
    { title: t('trafficUsage', '流量使用'), value: `${summary?.trafficUsedGB || 0} / ${summary?.trafficTotalGB || 0} GB`, icon: Activity, colorClass: 'c-pink' },
  ]

  function logout() {
    localStorage.removeItem('token')
    window.location.href = 'login.html'
  }

  function toggleTheme() {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }

  return (
    <div className="dashboard-shell">
      <Sidebar tab={tab} onTabChange={setTab} onLogout={logout} t={t} />

      <main className="main">
        <Topbar username={username} onToggleTheme={toggleTheme} theme={theme} lang={lang} onLangChange={setLang} t={t} />
        <section className="main-inner">
          <div className="title-row">
            <div>
              <h1>{t(tab, tab === 'dashboard' ? '控制面板' : tab)}</h1>
              <p>{tab === 'dashboard' ? t('welcome', '欢迎回来，系统运行状态良好。') : t('moduleReady', '模块功能已接入，可直接操作。')}</p>
            </div>
          </div>

          {tab === 'dashboard' ? (
            <>
              <div className="stat-grid">
                {cardItems.map((item) => (
                  <StatCard key={item.title} {...item} />
                ))}
              </div>
              <DashboardCharts history={history} protocolData={protocolData} lang={lang} />
            </>
          ) : null}

          {tab === 'inbounds' ? <InboundsModule t={t} lang={lang} /> : null}
          {tab === 'subscription' ? <SubscriptionModule /> : null}
          {tab === 'tools' ? <ToolsModule /> : null}
          {tab === 'logs' ? <LogsModule /> : null}
          {tab === 'settings' ? <SettingsModule /> : null}
        </section>
      </main>
    </div>
  )
}

createRoot(document.getElementById('root')).render(<App />)
