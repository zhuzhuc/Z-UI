import React from 'react'
import {
  LayoutDashboard,
  Server,
  Link2,
  PenTool,
  FileText,
  Settings,
  LogOut,
  Code2,
} from 'lucide-react'

const items = [
  { key: 'dashboard', label: 'dashboard', icon: LayoutDashboard },
  { key: 'inbounds', label: 'inbounds', icon: Server },
  { key: 'subscription', label: 'subscription', icon: Link2 },
  { key: 'tools', label: 'tools', icon: PenTool },
  { key: 'xray-config', label: 'xrayConfig', icon: Code2 },
  { key: 'logs', label: 'logs', icon: FileText },
  { key: 'settings', label: 'settings', icon: Settings },
]

export default function Sidebar({ tab, onTabChange, onLogout, t, brandTitle = 'Z-UI' }) {
  return (
    <aside className="side">
      <div className="brand-head">
        <div>
          <h2>{brandTitle}</h2>
          <p>V2Ray/Xray Panel</p>
        </div>
      </div>

      <div className="menu-section" aria-hidden="true">{t('manage')}</div>
      <nav className="side-nav" aria-label="Main navigation">
        {items.map((item) => {
          const Icon = item.icon
          const isActive = tab === item.key
          return (
            <button
              key={item.key}
              type="button"
              className={`side-item ${isActive ? 'active' : ''}`}
              onClick={() => onTabChange(item.key)}
              aria-current={isActive ? 'page' : undefined}
              aria-label={t(item.label)}
            >
              <Icon size={18} />
              <span>{t(item.label)}</span>
            </button>
          )
        })}
      </nav>

      <button className="side-item logout" type="button" onClick={onLogout} aria-label={t('logout')}>
        <LogOut size={18} />
        <span>{t('logout')}</span>
      </button>
    </aside>
  )
}
