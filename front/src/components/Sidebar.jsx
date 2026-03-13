import React from 'react'
import {
  LayoutDashboard,
  Server,
  Link2,
  PenTool,
  FileText,
  Settings,
  LogOut,
} from 'lucide-react'

const items = [
  { key: 'dashboard', label: 'dashboard', icon: LayoutDashboard },
  { key: 'inbounds', label: 'inbounds', icon: Server },
  { key: 'subscription', label: 'subscription', icon: Link2 },
  { key: 'tools', label: 'tools', icon: PenTool },
  { key: 'logs', label: 'logs', icon: FileText },
  { key: 'settings', label: 'settings', icon: Settings },
]

export default function Sidebar({ tab, onTabChange, onLogout, t }) {
  return (
    <aside className="side">
      <div className="brand-head">
        <div>
          <h2>Z-UI</h2>
          <p>V2Ray/Xray Panel</p>
        </div>
      </div>

      <div className="menu-section">{t('manage', 'MANAGE')}</div>
      <nav className="side-nav">
        {items.map((item) => {
          const Icon = item.icon
          return (
            <button
              key={item.key}
              type="button"
              className={`side-item ${tab === item.key ? 'active' : ''}`}
              onClick={() => onTabChange(item.key)}
            >
              <Icon size={18} />
              <span>{t(item.label, item.label)}</span>
            </button>
          )
        })}
      </nav>

      <button className="side-item logout" type="button" onClick={onLogout}>
        <LogOut size={18} />
        <span>{t('logout', '退出登录')}</span>
      </button>
    </aside>
  )
}
